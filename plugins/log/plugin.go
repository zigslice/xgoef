// Package log registers log:write. The page sends a level and a message.
// It cannot choose the directory, the file name, or a crash record.
package log

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"os"
	"sync"

	"github.com/zigslice/xgoef/ipc"
	applog "github.com/zigslice/xgoef/log"
	"github.com/zigslice/xgoef/plugin"
)

// Plugin implements plugin.Plugin.
type Plugin struct {
	id     string
	min    applog.Level
	dir    string
	hasDir bool
	stderr io.Writer
	hasErr bool
	mu     sync.Mutex
	out    *applog.Writer
}

// New returns the log builtin.
// identifier becomes the file name. level is the minimum page level;
// empty means info.
func New(identifier, level string) (*Plugin, error) {
	if _, err := applog.FileName(identifier); err != nil {
		return nil, err
	}
	min, err := applog.ParseLevel(level)
	if err != nil {
		return nil, err
	}
	return &Plugin{id: identifier, min: min}, nil
}

// SetDir overrides the log directory.
// Tests use it before the first write. Production leaves it unset and uses
// the current user's state directory.
func (p *Plugin) SetDir(dir string) {
	if p == nil {
		return
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.out != nil {
		return
	}
	p.dir = dir
	p.hasDir = true
}

// SetStderr overrides stderr. Nil disables it.
// Tests use it before the first write. Production leaves it unset and uses os.Stderr.
func (p *Plugin) SetStderr(w io.Writer) {
	if p == nil {
		return
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.out != nil {
		return
	}
	p.stderr = w
	p.hasErr = true
}

// Dir is the absolute directory passed to the browser as --xgoef-crash-dumps,
// or empty when no file target is available.
func (p *Plugin) Dir() string {
	if p == nil {
		return ""
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.hasDir {
		return p.dir
	}
	dir, err := applog.UserDir()
	if err != nil {
		return ""
	}
	return dir
}

// Name is the builtin plugin id.
func (p *Plugin) Name() string { return "log" }

// Permissions are the capability ids an app must grant.
func (p *Plugin) Permissions() []string {
	return []string{"log:write"}
}

// Setup registers log:write.
func (p *Plugin) Setup(app plugin.Host) error {
	app.Register("log:write", p.write)
	return nil
}

// Teardown is a no-op.
func (p *Plugin) Teardown(plugin.Host) error { return nil }

// Crash records a host panic. The page cannot call this.
func (p *Plugin) Crash(summary string) error {
	w, err := p.writer()
	if err != nil {
		return err
	}
	return w.Crash(summary)
}

func (p *Plugin) write(_ ipc.Context, argsJSON []byte) (any, error) {
	level, message, err := parseWrite(argsJSON)
	if err != nil {
		return nil, err
	}
	parsed, err := applog.ParseLevel(level)
	if err != nil {
		return nil, mapErr(err)
	}
	w, err := p.writer()
	if err != nil {
		return nil, mapErr(err)
	}
	if err := w.Write(parsed, message); err != nil {
		return nil, mapErr(err)
	}
	return nil, nil
}

func (p *Plugin) writer() (*applog.Writer, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.out != nil {
		return p.out, nil
	}
	dir := ""
	if p.hasDir {
		dir = p.dir
	} else if found, err := applog.UserDir(); err == nil {
		dir = found
	}
	stderr := p.stderr
	if !p.hasErr {
		stderr = os.Stderr
	}
	w, err := applog.Open(dir, p.id, p.min, stderr)
	if err != nil {
		return nil, err
	}
	p.out = w
	return w, nil
}

func parseWrite(argsJSON []byte) (string, string, error) {
	trimmed := bytes.TrimSpace(argsJSON)
	if len(trimmed) == 0 || string(trimmed) == "null" {
		return "", "", ipc.Fail("log.invalid", "log: message is empty")
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(trimmed, &raw); err != nil || raw == nil {
		return "", "", ipc.Fail("log.invalid", "log: arguments are invalid")
	}
	for key := range raw {
		if key != "level" && key != "message" {
			return "", "", ipc.Fail("log.denied", "log: unexpected field")
		}
	}
	msgRaw, ok := raw["message"]
	if !ok {
		return "", "", ipc.Fail("log.invalid", "log: message is empty")
	}
	var message string
	if err := json.Unmarshal(msgRaw, &message); err != nil {
		return "", "", ipc.Fail("log.invalid", "log: message is invalid")
	}
	level := ""
	if lvl, ok := raw["level"]; ok {
		if err := json.Unmarshal(lvl, &level); err != nil {
			return "", "", ipc.Fail("log.invalid", "log: level is invalid")
		}
	}
	return level, message, nil
}

func mapErr(err error) error {
	var le *applog.Error
	if errors.As(err, &le) {
		return ipc.Fail("log."+le.Code(), "%s", le.Error())
	}
	return ipc.Fail("log.write", "log: %s", err.Error())
}
