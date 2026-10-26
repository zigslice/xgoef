// Package taskbar shows or hides a window's taskbar button and sets its progress.
// The page acts on the calling window. It cannot name another label.
package taskbar

import (
	"bytes"
	"encoding/json"
	"math"

	"github.com/zigslice/xgoef/ipc"
	"github.com/zigslice/xgoef/plugin"
)

// Skipper shows or hides the taskbar button for one window label.
type Skipper func(window string, skip bool) error

// Progress sets or clears the taskbar progress for one window label.
// progress -1 clears it. 0..100 sets it.
type Progress func(window string, progress int) error

// Plugin implements plugin.Plugin.
type Plugin struct {
	skip     Skipper
	progress Progress
}

// New returns the taskbar builtin. A nil func makes that command fail
// with taskbar.unavailable.
func New(skip Skipper, progress Progress) *Plugin {
	return &Plugin{skip: skip, progress: progress}
}

// Name is the builtin plugin id.
func (p *Plugin) Name() string { return "taskbar" }

// Permissions are the capability ids an app must grant.
func (p *Plugin) Permissions() []string {
	return []string{"taskbar:skip", "taskbar:progress"}
}

// Setup registers the taskbar commands.
func (p *Plugin) Setup(app plugin.Host) error {
	app.Register("taskbar:skip", p.doSkip)
	app.Register("taskbar:progress", p.doProgress)
	return nil
}

// Teardown is a no-op.
func (p *Plugin) Teardown(plugin.Host) error { return nil }

func (p *Plugin) doSkip(ctx ipc.Context, argsJSON []byte) (any, error) {
	skip, err := parseSkip(argsJSON)
	if err != nil {
		return nil, err
	}
	if p.skip == nil {
		return nil, ipc.Fail("taskbar.unavailable", "taskbar: cannot skip")
	}
	window := ctx.Window
	if window == "" {
		window = "main"
	}
	if err := p.skip(window, skip); err != nil {
		return nil, ipc.Fail("taskbar.unavailable", "%s", err.Error())
	}
	return nil, nil
}

func (p *Plugin) doProgress(ctx ipc.Context, argsJSON []byte) (any, error) {
	progress, err := parseProgress(argsJSON)
	if err != nil {
		return nil, err
	}
	if p.progress == nil {
		return nil, ipc.Fail("taskbar.unavailable", "taskbar: cannot set progress")
	}
	window := ctx.Window
	if window == "" {
		window = "main"
	}
	if err := p.progress(window, progress); err != nil {
		return nil, ipc.Fail("taskbar.unavailable", "%s", err.Error())
	}
	return nil, nil
}

func parseSkip(argsJSON []byte) (bool, error) {
	if len(argsJSON) == 0 {
		argsJSON = []byte("{}")
	}
	var raw struct {
		Skip *bool `json:"skip"`
	}
	dec := json.NewDecoder(bytes.NewReader(argsJSON))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&raw); err != nil || raw.Skip == nil || dec.More() {
		return false, ipc.Fail("taskbar.invalid", "taskbar: skip must be a boolean")
	}
	return *raw.Skip, nil
}

func parseProgress(argsJSON []byte) (int, error) {
	if len(argsJSON) == 0 {
		argsJSON = []byte("{}")
	}
	var raw struct {
		Progress json.RawMessage `json:"progress"`
	}
	dec := json.NewDecoder(bytes.NewReader(argsJSON))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&raw); err != nil || len(raw.Progress) == 0 || dec.More() {
		return 0, ipc.Fail("taskbar.invalid", "taskbar: progress is required")
	}
	if string(raw.Progress) == "null" {
		return -1, nil
	}
	var num float64
	if err := json.Unmarshal(raw.Progress, &num); err != nil {
		return 0, ipc.Fail("taskbar.invalid", "taskbar: progress must be an integer or null")
	}
	if math.IsNaN(num) || math.IsInf(num, 0) || num != math.Trunc(num) {
		return 0, ipc.Fail("taskbar.invalid", "taskbar: progress must be an integer or null")
	}
	if num < 0 || num > 100 {
		return 0, ipc.Fail("taskbar.invalid", "taskbar: progress out of range")
	}
	return int(num), nil
}
