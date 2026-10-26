// Package autostart writes a current-user XDG autostart desktop entry.
// The page cannot choose the executable or the arguments.
package autostart

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

const (
	// MaxArgs is the most launch arguments a config may store.
	MaxArgs = 8
	// MaxArgLen is the longest single argument, in bytes.
	MaxArgLen = 128
	// maxEntryBytes is the largest desktop file read back to decide state.
	maxEntryBytes = 64 * 1024
)

// Error is a failure with a stable suffix. The plugin prefixes it as
// autostart.<code>.
type Error struct {
	code string
	msg  string
}

func (e *Error) Error() string {
	if e == nil || e.msg == "" {
		return "autostart: failed"
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

// ValidateArgs checks launch arguments from config.
// Empty and whitespace-only values are rejected. Each remaining value is
// 1 to 128 bytes and has no rune below U+0020.
func ValidateArgs(args []string) error {
	if len(args) > MaxArgs {
		return fail("invalid", "autostart: too many arguments")
	}
	for _, arg := range args {
		if strings.TrimSpace(arg) == "" {
			return fail("invalid", "autostart: argument is empty")
		}
		if len(arg) > MaxArgLen {
			return fail("invalid", "autostart: argument is too long")
		}
		for _, r := range arg {
			if r < 0x20 {
				return fail("invalid", "autostart: argument is invalid")
			}
		}
	}
	return nil
}

// ValidateName checks the application name written into the desktop entry.
func ValidateName(name string) error {
	if strings.TrimSpace(name) == "" {
		return fail("invalid", "autostart: application name is empty")
	}
	for _, r := range name {
		if r < 0x20 {
			return fail("invalid", "autostart: application name is invalid")
		}
	}
	return nil
}

// FileName is the desktop entry file for an application identifier.
func FileName(identifier string) (string, error) {
	if strings.TrimSpace(identifier) == "" {
		return "", fail("invalid", "autostart: identifier is required")
	}
	name := sanitize(identifier)
	if name == "" {
		return "", fail("invalid", "autostart: identifier is empty")
	}
	return "xgoef-" + name + ".desktop", nil
}

// Entry is the desktop file body. exec must be absolute. Every token in
// Exec is quoted.
func Entry(name, exec string, args []string) (string, error) {
	if err := ValidateName(name); err != nil {
		return "", err
	}
	if err := validExec(exec); err != nil {
		return "", err
	}
	if err := ValidateArgs(args); err != nil {
		return "", err
	}
	var b strings.Builder
	b.WriteString("[Desktop Entry]\nVersion=1.0\nType=Application\nName=")
	b.WriteString(name)
	b.WriteString("\nExec=")
	b.WriteString(quote(exec))
	for _, arg := range args {
		b.WriteByte(' ')
		b.WriteString(quote(arg))
	}
	b.WriteString("\nX-GNOME-Autostart-enabled=true\n")
	return b.String(), nil
}

// Enable writes the desktop entry for identifier inside dir.
// A symlink at the destination is refused. The write is a temp file in dir
// renamed into place, so a rewrite replaces the entry.
func Enable(dir, identifier, name, exec string, args []string) (string, error) {
	path, err := slot(dir, identifier)
	if err != nil {
		return "", err
	}
	body, err := Entry(name, exec, args)
	if err != nil {
		return "", err
	}
	clean := filepath.Dir(path)
	if err := os.MkdirAll(clean, 0o755); err != nil {
		return "", failf("write", "autostart: %s", err.Error())
	}
	if err := refuseSpecial(path); err != nil {
		return "", err
	}
	tmp, err := os.CreateTemp(clean, ".xgoef-autostart-*")
	if err != nil {
		return "", failf("write", "autostart: %s", err.Error())
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
		return "", failf("write", "autostart: %s", err.Error())
	}
	if err := tmp.Chmod(0o644); err != nil {
		_ = tmp.Close()
		return "", failf("write", "autostart: %s", err.Error())
	}
	if err := tmp.Close(); err != nil {
		return "", failf("write", "autostart: %s", err.Error())
	}
	if err := os.Rename(tmpName, path); err != nil {
		return "", failf("write", "autostart: %s", err.Error())
	}
	written = true
	return path, nil
}

// Disable removes the desktop entry for identifier.
// A missing file is success. A symlink is removed without deleting its target.
// A directory at that name is an error.
func Disable(dir, identifier string) error {
	path, err := slot(dir, identifier)
	if err != nil {
		return err
	}
	info, err := os.Lstat(path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return failf("write", "autostart: %s", err.Error())
	}
	if info.IsDir() {
		return fail("denied", "autostart: entry is a directory")
	}
	if err := os.Remove(path); err != nil {
		return failf("write", "autostart: %s", err.Error())
	}
	return nil
}

// Enabled reports whether the desktop entry is present and not turned off.
// A missing file is false. Hidden=true or X-GNOME-Autostart-enabled=false
// inside the first Desktop Entry group is false. Other existing files are true.
func Enabled(dir, identifier string) (bool, error) {
	path, err := slot(dir, identifier)
	if err != nil {
		return false, err
	}
	info, err := os.Lstat(path)
	if os.IsNotExist(err) {
		return false, nil
	}
	if err != nil {
		return false, failf("write", "autostart: %s", err.Error())
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return false, fail("denied", "autostart: entry is a symlink")
	}
	if info.IsDir() {
		return false, fail("denied", "autostart: entry is a directory")
	}
	if !info.Mode().IsRegular() {
		return false, fail("denied", "autostart: entry is not a file")
	}
	if info.Size() > maxEntryBytes {
		return false, fail("invalid", "autostart: entry is too large")
	}
	body, err := os.ReadFile(path)
	if err != nil {
		return false, failf("write", "autostart: %s", err.Error())
	}
	if len(body) > maxEntryBytes {
		return false, fail("invalid", "autostart: entry is too large")
	}
	return entryEnabled(string(body)), nil
}

// UserDir is the current user's autostart directory.
// On Linux that is $XDG_CONFIG_HOME/autostart, or ~/.config/autostart.
// A relative XDG_CONFIG_HOME is unavailable. Other operating systems are unavailable.
func UserDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		home = ""
	}
	return userDir(runtime.GOOS, os.Getenv("XDG_CONFIG_HOME"), home)
}

func userDir(goos, xdg, home string) (string, error) {
	if goos != "linux" {
		return "", fail("unavailable", "autostart: unavailable")
	}
	if xdg != "" {
		if !filepath.IsAbs(xdg) {
			return "", fail("unavailable", "autostart: unavailable")
		}
		return filepath.Join(xdg, "autostart"), nil
	}
	if !filepath.IsAbs(home) {
		return "", fail("unavailable", "autostart: unavailable")
	}
	return filepath.Join(home, ".config", "autostart"), nil
}

func slot(dir, identifier string) (string, error) {
	if !filepath.IsAbs(dir) {
		return "", fail("unavailable", "autostart: unavailable")
	}
	file, err := FileName(identifier)
	if err != nil {
		return "", err
	}
	clean := filepath.Clean(dir)
	path := filepath.Join(clean, file)
	if filepath.Dir(path) != clean {
		return "", fail("denied", "autostart: path left the autostart directory")
	}
	return path, nil
}

func refuseSpecial(path string) error {
	info, err := os.Lstat(path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return failf("write", "autostart: %s", err.Error())
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return fail("denied", "autostart: entry is a symlink")
	}
	if info.IsDir() {
		return fail("denied", "autostart: entry is a directory")
	}
	if !info.Mode().IsRegular() {
		return fail("denied", "autostart: entry is not a file")
	}
	return nil
}

func entryEnabled(body string) bool {
	body = strings.ReplaceAll(body, "\r\n", "\n")
	body = strings.ReplaceAll(body, "\r", "\n")
	in := false
	for _, line := range strings.Split(body, "\n") {
		trim := strings.TrimSpace(line)
		if trim == "" || strings.HasPrefix(trim, "#") {
			continue
		}
		if strings.HasPrefix(trim, "[") && strings.HasSuffix(trim, "]") {
			if in {
				break
			}
			in = trim == "[Desktop Entry]"
			continue
		}
		if !in {
			continue
		}
		key, val, ok := strings.Cut(trim, "=")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		val = strings.TrimSpace(val)
		if key == "Hidden" && val == "true" {
			return false
		}
		if key == "X-GNOME-Autostart-enabled" && val == "false" {
			return false
		}
	}
	return true
}

func validExec(exec string) error {
	if !filepath.IsAbs(exec) || strings.IndexByte(exec, 0) >= 0 {
		return fail("invalid", "autostart: executable must be absolute")
	}
	for _, r := range exec {
		if r < 0x20 {
			return fail("invalid", "autostart: executable must be absolute")
		}
	}
	return nil
}

func sanitize(identifier string) string {
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
		return ""
	}
	return name
}

func quote(token string) string {
	var b strings.Builder
	b.WriteByte('"')
	for _, r := range token {
		if r == '\\' || r == '"' || r == '$' || r == '`' {
			b.WriteByte('\\')
		}
		b.WriteRune(r)
	}
	b.WriteByte('"')
	return b.String()
}
