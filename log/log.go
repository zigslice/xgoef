// Package log appends application lines and the latest host crash.
// The page chooses a level and a message. It does not choose the directory,
// the file name, or the crash text.
package log

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"
)

const (
	// MaxMessage is the longest page message, in bytes.
	MaxMessage = 4096
	// MaxFileBytes is the size at which the active log rotates to a single backup.
	MaxFileBytes = 1 << 20
)

// Level is a page log level. Crash is host-only and is not a Level.
type Level int

const (
	// LevelTrace is the lowest page level.
	LevelTrace Level = iota
	// LevelDebug is below info.
	LevelDebug
	// LevelInfo is the default minimum.
	LevelInfo
	// LevelWarn is above info.
	LevelWarn
	// LevelError is the highest page level.
	LevelError
)

// Error is a failure with a stable suffix. The plugin prefixes it as log.<code>.
type Error struct {
	code string
	msg  string
}

func (e *Error) Error() string {
	if e == nil || e.msg == "" {
		return "log: failed"
	}
	return e.msg
}

// Code is the suffix the plugin turns into an invoke code.
func (e *Error) Code() string {
	if e == nil {
		return ""
	}
	return e.code
}

func fail(code, msg string) error {
	return &Error{code: code, msg: msg}
}

func failf(code, format string, args ...any) error {
	return &Error{code: code, msg: fmt.Sprintf(format, args...)}
}

// ParseLevel reads a page or config level.
// An empty string is info. Unknown text is invalid.
func ParseLevel(s string) (Level, error) {
	switch s {
	case "":
		return LevelInfo, nil
	case "trace":
		return LevelTrace, nil
	case "debug":
		return LevelDebug, nil
	case "info":
		return LevelInfo, nil
	case "warn":
		return LevelWarn, nil
	case "error":
		return LevelError, nil
	default:
		return 0, fail("invalid", "log: level is invalid")
	}
}

// ValidateLevel checks a config level. An empty string is info.
func ValidateLevel(s string) error {
	_, err := ParseLevel(s)
	return err
}

// FileName is the log file for an application identifier.
func FileName(identifier string) (string, error) {
	name, err := sanitize(identifier)
	if err != nil {
		return "", err
	}
	return "xgoef-" + name + ".log", nil
}

// CrashFileName is the latest host-crash sidecar for an application identifier.
func CrashFileName(identifier string) (string, error) {
	name, err := sanitize(identifier)
	if err != nil {
		return "", err
	}
	return "xgoef-" + name + ".crash", nil
}

// UserDir is the current user's log directory.
// On Linux that is $XDG_STATE_HOME/xgoef, or ~/.local/state/xgoef.
// A relative XDG_STATE_HOME is unavailable. Other operating systems are unavailable.
func UserDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		home = ""
	}
	return userDir(runtime.GOOS, os.Getenv("XDG_STATE_HOME"), home)
}

func userDir(goos, xdg, home string) (string, error) {
	if goos != "linux" {
		return "", fail("unavailable", "log: unavailable")
	}
	if xdg != "" {
		if !filepath.IsAbs(xdg) {
			return "", fail("unavailable", "log: unavailable")
		}
		return filepath.Join(xdg, "xgoef"), nil
	}
	if !filepath.IsAbs(home) {
		return "", fail("unavailable", "log: unavailable")
	}
	return filepath.Join(home, ".local", "state", "xgoef"), nil
}

// Writer appends lines to a log file and, when set, to stderr.
type Writer struct {
	dir    string
	log    string
	crash  string
	min    Level
	stderr io.Writer
	now    func() time.Time
	max    int
	mu     sync.Mutex
}

// Open prepares a writer.
// dir is the absolute directory for the log and crash files. Empty means no file.
// A relative dir is unavailable. stderr may be nil. If both targets are absent,
// Open returns unavailable.
func Open(dir, identifier string, min Level, stderr io.Writer) (*Writer, error) {
	logName, err := FileName(identifier)
	if err != nil {
		return nil, err
	}
	crashName, err := CrashFileName(identifier)
	if err != nil {
		return nil, err
	}
	if dir != "" && !filepath.IsAbs(dir) {
		return nil, fail("unavailable", "log: unavailable")
	}
	if dir == "" && stderr == nil {
		return nil, fail("unavailable", "log: unavailable")
	}
	if min < LevelTrace || min > LevelError {
		min = LevelInfo
	}
	return &Writer{
		dir:    dir,
		log:    logName,
		crash:  crashName,
		min:    min,
		stderr: stderr,
		now:    time.Now,
		max:    MaxFileBytes,
	}, nil
}

// SetNow replaces the clock used for line timestamps.
func (w *Writer) SetNow(now func() time.Time) {
	if w == nil {
		return
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	if now == nil {
		w.now = time.Now
		return
	}
	w.now = now
}

// SetMaxBytes sets the size that triggers rotation.
// Non-positive values are ignored.
func (w *Writer) SetMaxBytes(n int) {
	if w == nil || n < 1 {
		return
	}
	w.mu.Lock()
	w.max = n
	w.mu.Unlock()
}

// Write appends one page line when level is at least the configured minimum.
// A lower level is a success and writes nothing. The message must be one
// non-empty line of at most 4096 bytes.
func (w *Writer) Write(level Level, message string) error {
	if w == nil {
		return fail("unavailable", "log: unavailable")
	}
	if err := validMessage(message); err != nil {
		return err
	}
	if level < w.min {
		return nil
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.writeUnlocked(w.stamp(level.name(), message), false)
}

// Crash records a host summary. It is not filtered by level.
// The sidecar keeps only the latest crash. The same line is appended to the log.
func (w *Writer) Crash(summary string) error {
	if w == nil {
		return fail("unavailable", "log: unavailable")
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.writeUnlocked(w.stamp("CRASH", flatten(summary)), true)
}

func (w *Writer) writeUnlocked(line string, crash bool) error {
	wrote := false
	if w.dir != "" {
		logPath, crashPath, err := w.paths()
		if err != nil {
			return err
		}
		if err := os.MkdirAll(w.dir, 0o755); err != nil {
			return failf("write", "log: %s", err.Error())
		}
		if crash {
			if err := writeAtomic(filepath.Clean(w.dir), crashPath, line); err != nil {
				return err
			}
		}
		if err := w.appendLine(logPath, line); err != nil {
			return err
		}
		wrote = true
	}
	if w.stderr != nil {
		if _, err := io.WriteString(w.stderr, line); err != nil {
			return failf("write", "log: %s", err.Error())
		}
		wrote = true
	}
	if !wrote {
		return fail("unavailable", "log: unavailable")
	}
	return nil
}

func (w *Writer) stamp(kind, message string) string {
	now := time.Now
	if w.now != nil {
		now = w.now
	}
	ts := now().UTC().Format("2006-01-02T15:04:05.000Z")
	return ts + " " + kind + " " + message + "\n"
}

func (w *Writer) paths() (string, string, error) {
	clean := filepath.Clean(w.dir)
	logPath := filepath.Join(clean, w.log)
	if filepath.Dir(logPath) != clean {
		return "", "", fail("denied", "log: path left the log directory")
	}
	crashPath := filepath.Join(clean, w.crash)
	if filepath.Dir(crashPath) != clean {
		return "", "", fail("denied", "log: path left the log directory")
	}
	return logPath, crashPath, nil
}

func (w *Writer) appendLine(path, line string) error {
	info, err := os.Lstat(path)
	if err != nil && !os.IsNotExist(err) {
		return failf("write", "log: %s", err.Error())
	}
	if err == nil {
		if err := regularFile(info); err != nil {
			return err
		}
		if info.Size() > 0 && info.Size()+int64(len(line)) > int64(w.max) {
			if err := rotate(path); err != nil {
				return err
			}
		}
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY|noFollowFlag(), 0o644)
	if err != nil {
		if info, lerr := os.Lstat(path); lerr == nil && info.Mode()&os.ModeSymlink != 0 {
			return fail("denied", "log: file is a symlink")
		}
		return failf("write", "log: %s", err.Error())
	}
	if err := f.Chmod(0o644); err != nil {
		_ = f.Close()
		return failf("write", "log: %s", err.Error())
	}
	if _, err := io.WriteString(f, line); err != nil {
		_ = f.Close()
		return failf("write", "log: %s", err.Error())
	}
	if err := f.Close(); err != nil {
		return failf("write", "log: %s", err.Error())
	}
	return nil
}

func rotate(path string) error {
	dest := path + ".1"
	info, err := os.Lstat(dest)
	if err != nil && !os.IsNotExist(err) {
		return failf("write", "log: %s", err.Error())
	}
	if err == nil {
		if info.Mode()&os.ModeSymlink != 0 {
			return fail("denied", "log: file is a symlink")
		}
		if info.IsDir() {
			return fail("denied", "log: file is a directory")
		}
		if !info.Mode().IsRegular() {
			return fail("denied", "log: file is not a file")
		}
	}
	if err := os.Rename(path, dest); err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return failf("write", "log: %s", err.Error())
	}
	return nil
}

func writeAtomic(dir, path, body string) error {
	if err := refuseSpecial(path); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, ".xgoef-crash-*")
	if err != nil {
		return failf("write", "log: %s", err.Error())
	}
	tmpName := tmp.Name()
	written := false
	defer func() {
		if !written {
			_ = os.Remove(tmpName)
		}
	}()
	if _, err := io.WriteString(tmp, body); err != nil {
		_ = tmp.Close()
		return failf("write", "log: %s", err.Error())
	}
	if err := tmp.Chmod(0o644); err != nil {
		_ = tmp.Close()
		return failf("write", "log: %s", err.Error())
	}
	if err := tmp.Close(); err != nil {
		return failf("write", "log: %s", err.Error())
	}
	if err := os.Rename(tmpName, path); err != nil {
		return failf("write", "log: %s", err.Error())
	}
	written = true
	return nil
}

func refuseSpecial(path string) error {
	info, err := os.Lstat(path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return failf("write", "log: %s", err.Error())
	}
	return regularFile(info)
}

func regularFile(info os.FileInfo) error {
	if info.Mode()&os.ModeSymlink != 0 {
		return fail("denied", "log: file is a symlink")
	}
	if info.IsDir() {
		return fail("denied", "log: file is a directory")
	}
	if !info.Mode().IsRegular() {
		return fail("denied", "log: file is not a file")
	}
	return nil
}

func validMessage(message string) error {
	if strings.TrimSpace(message) == "" {
		return fail("invalid", "log: message is empty")
	}
	if len(message) > MaxMessage {
		return fail("invalid", "log: message is too long")
	}
	for _, r := range message {
		if r < 0x20 {
			return fail("invalid", "log: message is invalid")
		}
	}
	return nil
}

func flatten(summary string) string {
	var b strings.Builder
	for _, r := range summary {
		if r < 0x20 {
			b.WriteByte(' ')
			continue
		}
		b.WriteRune(r)
	}
	s := strings.TrimSpace(b.String())
	if s == "" {
		s = "panic"
	}
	if len(s) <= MaxMessage {
		return s
	}
	s = s[:MaxMessage]
	for len(s) > 0 && s[len(s)-1]&0xC0 == 0x80 {
		s = s[:len(s)-1]
	}
	if s == "" {
		return "panic"
	}
	return s
}

func (l Level) name() string {
	switch l {
	case LevelTrace:
		return "TRACE"
	case LevelDebug:
		return "DEBUG"
	case LevelInfo:
		return "INFO"
	case LevelWarn:
		return "WARN"
	case LevelError:
		return "ERROR"
	default:
		return "INFO"
	}
}

func sanitize(identifier string) (string, error) {
	if strings.TrimSpace(identifier) == "" {
		return "", fail("invalid", "log: identifier is required")
	}
	var b strings.Builder
	for _, r := range strings.TrimSpace(identifier) {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '.', r == '_', r == '-':
			b.WriteRune(r)
		default:
			b.WriteByte('_')
		}
	}
	name := b.String()
	if name == "" || name == "." || name == ".." {
		return "", fail("invalid", "log: identifier is empty")
	}
	return name, nil
}
