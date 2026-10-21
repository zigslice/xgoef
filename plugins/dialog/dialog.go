// Package dialog asks the user for a file path and then checks it against a
// directory scope. The default picker is zenity. Tests pass their own Picker.
package dialog

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"

	"github.com/zigslice/xgoef/ipc"
	"github.com/zigslice/xgoef/plugin"
	"github.com/zigslice/xgoef/plugins/fs"
)

// Request is what the picker shows.
type Request struct {
	Title       string
	DefaultPath string
	Directory   bool
	Save        bool
}

// Result is the user's choice. Cancelled is not an error.
type Result struct {
	Path      string
	Cancelled bool
}

// Picker runs one dialog.
type Picker func(Request) (Result, error)

// Plugin implements plugin.Plugin.
type Plugin struct {
	base   string
	scope  *fs.Scope
	picker Picker
}

// New builds the dialog plugin. An empty allow list rejects every chosen path.
// picker nil uses zenity when the command is invoked.
func New(base, resource string, allow, deny []string, picker Picker) (*Plugin, error) {
	if base == "" {
		base = "."
	}
	scope, err := fs.NewScope(base, resource, allow, deny)
	if err != nil {
		return nil, err
	}
	if picker == nil {
		picker = zenity
	}
	return &Plugin{base: base, scope: scope, picker: picker}, nil
}

// Name is the builtin plugin id.
func (p *Plugin) Name() string { return "dialog" }

// Permissions are the capability ids an app must grant.
func (p *Plugin) Permissions() []string {
	return []string{"dialog:open", "dialog:save"}
}

// Setup registers dialog:open and dialog:save.
func (p *Plugin) Setup(app plugin.Host) error {
	app.Register("dialog:open", p.open)
	app.Register("dialog:save", p.save)
	return nil
}

// Teardown is a no-op.
func (p *Plugin) Teardown(plugin.Host) error { return nil }

type args struct {
	Title       string `json:"title"`
	DefaultPath string `json:"defaultPath"`
	Directory   bool   `json:"directory"`
}

func (p *Plugin) open(_ ipc.Context, raw []byte) (any, error) {
	var a args
	if len(raw) > 0 && string(raw) != "null" {
		if err := json.Unmarshal(raw, &a); err != nil {
			return nil, fmt.Errorf("dialog: %w", err)
		}
	}
	return p.pick(Request{Title: a.Title, DefaultPath: a.DefaultPath, Directory: a.Directory})
}

func (p *Plugin) save(_ ipc.Context, raw []byte) (any, error) {
	var a args
	if len(raw) > 0 && string(raw) != "null" {
		if err := json.Unmarshal(raw, &a); err != nil {
			return nil, fmt.Errorf("dialog: %w", err)
		}
	}
	return p.pick(Request{Title: a.Title, DefaultPath: a.DefaultPath, Save: true})
}

func (p *Plugin) pick(req Request) (any, error) {
	res, err := p.picker(req)
	if err != nil {
		return nil, err
	}
	if res.Cancelled || strings.TrimSpace(res.Path) == "" {
		return nil, nil
	}
	abs, err := p.scope.Resolve(p.base, res.Path)
	if err != nil {
		return nil, ipc.Fail("dialog.denied", "dialog: path %q is outside the configured scope", res.Path)
	}
	return abs, nil
}

func zenity(req Request) (Result, error) {
	if _, err := exec.LookPath("zenity"); err != nil {
		return Result{}, fmt.Errorf("dialog: zenity not found")
	}
	argv := []string{"--file-selection"}
	if req.Title != "" {
		argv = append(argv, "--title", req.Title)
	}
	if req.Save {
		argv = append(argv, "--save")
	}
	if req.Directory {
		argv = append(argv, "--directory")
	}
	if req.DefaultPath != "" {
		argv = append(argv, "--filename", req.DefaultPath)
	}
	cmd := exec.Command("zenity", argv...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	if err != nil {
		if exit, ok := err.(*exec.ExitError); ok && exit.ExitCode() == 1 {
			return Result{Cancelled: true}, nil
		}
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = err.Error()
		}
		return Result{}, fmt.Errorf("dialog: %s", msg)
	}
	return Result{Path: strings.TrimSpace(stdout.String())}, nil
}
