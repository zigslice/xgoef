// Package autostart registers the commands that turn a user autostart entry
// on or off. The page cannot choose the executable or the arguments.
package autostart

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"runtime"

	entry "github.com/zigslice/xgoef/autostart"
	"github.com/zigslice/xgoef/ipc"
	"github.com/zigslice/xgoef/plugin"
)

// Plugin implements plugin.Plugin.
type Plugin struct {
	id     string
	name   string
	exe    string
	args   []string
	dir    string
	hasDir bool
	system string
}

// New returns the autostart builtin.
// identifier becomes the desktop file name. name is the desktop Name.
// exe is the absolute host path; empty is resolved when a command runs.
// args come from config. The page cannot replace them.
func New(identifier, name, exe string, args []string) (*Plugin, error) {
	if _, err := entry.FileName(identifier); err != nil {
		return nil, err
	}
	if err := entry.ValidateName(name); err != nil {
		return nil, err
	}
	if err := entry.ValidateArgs(args); err != nil {
		return nil, err
	}
	return &Plugin{
		id:   identifier,
		name: name,
		exe:  exe,
		args: append([]string(nil), args...),
	}, nil
}

// SetDir overrides the autostart directory.
// Tests use it. Production leaves it unset and uses the current user's
// XDG autostart directory.
func (p *Plugin) SetDir(dir string) {
	if p != nil {
		p.dir = dir
		p.hasDir = true
	}
}

// Name is the builtin plugin id.
func (p *Plugin) Name() string { return "autostart" }

// Permissions are the capability ids an app must grant.
func (p *Plugin) Permissions() []string {
	return []string{"autostart:enable", "autostart:disable", "autostart:is_enabled"}
}

// Setup registers autostart:enable, autostart:disable, and autostart:is_enabled.
func (p *Plugin) Setup(app plugin.Host) error {
	app.Register("autostart:enable", p.enable)
	app.Register("autostart:disable", p.disable)
	app.Register("autostart:is_enabled", p.isEnabled)
	return nil
}

// Teardown is a no-op.
func (p *Plugin) Teardown(plugin.Host) error { return nil }

func (p *Plugin) enable(_ ipc.Context, argsJSON []byte) (any, error) {
	if err := noArgs(argsJSON); err != nil {
		return nil, err
	}
	if err := p.supported(); err != nil {
		return nil, err
	}
	dir, err := p.directory()
	if err != nil {
		return nil, mapErr(err)
	}
	exe, err := p.executable()
	if err != nil {
		return nil, mapErr(err)
	}
	if _, err := entry.Enable(dir, p.id, p.name, exe, p.args); err != nil {
		return nil, mapErr(err)
	}
	return nil, nil
}

func (p *Plugin) disable(_ ipc.Context, argsJSON []byte) (any, error) {
	if err := noArgs(argsJSON); err != nil {
		return nil, err
	}
	if err := p.supported(); err != nil {
		return nil, err
	}
	dir, err := p.directory()
	if err != nil {
		return nil, mapErr(err)
	}
	if err := entry.Disable(dir, p.id); err != nil {
		return nil, mapErr(err)
	}
	return nil, nil
}

func (p *Plugin) isEnabled(_ ipc.Context, argsJSON []byte) (any, error) {
	if err := noArgs(argsJSON); err != nil {
		return nil, err
	}
	if err := p.supported(); err != nil {
		return nil, err
	}
	dir, err := p.directory()
	if err != nil {
		return nil, mapErr(err)
	}
	on, err := entry.Enabled(dir, p.id)
	if err != nil {
		return nil, mapErr(err)
	}
	return on, nil
}

func (p *Plugin) supported() error {
	system := p.system
	if system == "" {
		system = runtime.GOOS
	}
	if system != "linux" {
		return ipc.Fail("autostart.unavailable", "autostart: unavailable")
	}
	return nil
}

func (p *Plugin) directory() (string, error) {
	if p.hasDir {
		return p.dir, nil
	}
	return entry.UserDir()
}

func (p *Plugin) executable() (string, error) {
	if p.exe != "" {
		return p.exe, nil
	}
	found, err := os.Executable()
	if err != nil {
		return "", err
	}
	return found, nil
}

func noArgs(argsJSON []byte) error {
	trimmed := bytes.TrimSpace(argsJSON)
	if len(trimmed) == 0 || string(trimmed) == "null" {
		return nil
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(trimmed, &raw); err != nil || raw == nil || len(raw) != 0 {
		return ipc.Fail("autostart.invalid", "autostart: takes no arguments")
	}
	return nil
}

func mapErr(err error) error {
	var ae *entry.Error
	if errors.As(err, &ae) {
		return ipc.Fail("autostart."+ae.Code(), "%s", ae.Error())
	}
	return ipc.Fail("autostart.write", "autostart: %s", err.Error())
}
