// Package window applies verbs to the calling Views window.
// The page cannot name another window. An empty label is main.
// Moving the origin stays on monitor:move.
package window

import (
	"bytes"
	"encoding/json"
	"errors"
	"math"

	"github.com/zigslice/xgoef/cef"
	"github.com/zigslice/xgoef/ipc"
	"github.com/zigslice/xgoef/plugin"
)

// Op applies one verb. op, a, and b are the codes in cef.WindowOp*.
type Op func(window string, op, a, b int, text string) error

// Query reads one flag. query is a cef.WindowQuery* code.
type Query func(window string, query int) (bool, error)

// Plugin implements plugin.Plugin.
type Plugin struct {
	op    Op
	query Query
}

// New returns the window builtin. A nil op or query makes that call fail
// with window.unavailable.
func New(op Op, query Query) *Plugin {
	return &Plugin{op: op, query: query}
}

// Name is the builtin plugin id.
func (p *Plugin) Name() string { return "window" }

// Permissions are the capability ids an app must grant.
func (p *Plugin) Permissions() []string {
	return []string{
		"window:minimize",
		"window:maximize",
		"window:unminimize",
		"window:unmaximize",
		"window:toggle_maximize",
		"window:show",
		"window:hide",
		"window:center",
		"window:set_title",
		"window:set_always_on_top",
		"window:set_fullscreen",
		"window:set_size",
		"window:is_minimized",
		"window:is_maximized",
		"window:is_fullscreen",
		"window:is_always_on_top",
		"window:is_visible",
	}
}

// Setup registers the window commands.
func (p *Plugin) Setup(app plugin.Host) error {
	app.Register("window:minimize", p.verb(cef.WindowOpMinimize))
	app.Register("window:maximize", p.verb(cef.WindowOpMaximize))
	app.Register("window:unminimize", p.verb(cef.WindowOpUnminimize))
	app.Register("window:unmaximize", p.verb(cef.WindowOpUnmaximize))
	app.Register("window:toggle_maximize", p.verb(cef.WindowOpToggleMaximize))
	app.Register("window:show", p.verb(cef.WindowOpShow))
	app.Register("window:hide", p.verb(cef.WindowOpHide))
	app.Register("window:center", p.verb(cef.WindowOpCenter))
	app.Register("window:set_title", p.setTitle)
	app.Register("window:set_always_on_top", p.setAlwaysOnTop)
	app.Register("window:set_fullscreen", p.setFullscreen)
	app.Register("window:set_size", p.setSize)
	app.Register("window:is_minimized", p.flag(cef.WindowQueryMinimized))
	app.Register("window:is_maximized", p.flag(cef.WindowQueryMaximized))
	app.Register("window:is_fullscreen", p.flag(cef.WindowQueryFullscreen))
	app.Register("window:is_always_on_top", p.flag(cef.WindowQueryAlwaysOnTop))
	app.Register("window:is_visible", p.flag(cef.WindowQueryVisible))
	return nil
}

// Teardown is a no-op.
func (p *Plugin) Teardown(plugin.Host) error { return nil }

func (p *Plugin) verb(op int) ipc.Handler {
	return func(ctx ipc.Context, argsJSON []byte) (any, error) {
		if err := decode(argsJSON, &struct{}{}); err != nil {
			return nil, err
		}
		return nil, p.apply(ctx.Window, op, 0, 0, "")
	}
}

func (p *Plugin) setTitle(ctx ipc.Context, argsJSON []byte) (any, error) {
	var raw struct {
		Title *string `json:"title"`
	}
	if err := decode(argsJSON, &raw); err != nil {
		return nil, err
	}
	if raw.Title == nil || !cef.WindowTitleOK(*raw.Title) {
		return nil, ipc.Fail("window.invalid", "window: title is invalid")
	}
	return nil, p.apply(ctx.Window, cef.WindowOpSetTitle, 0, 0, *raw.Title)
}

func (p *Plugin) setAlwaysOnTop(ctx ipc.Context, argsJSON []byte) (any, error) {
	var raw struct {
		AlwaysOnTop *bool `json:"alwaysOnTop"`
	}
	if err := decode(argsJSON, &raw); err != nil {
		return nil, err
	}
	if raw.AlwaysOnTop == nil {
		return nil, ipc.Fail("window.invalid", "window: alwaysOnTop is required")
	}
	a := 0
	if *raw.AlwaysOnTop {
		a = 1
	}
	return nil, p.apply(ctx.Window, cef.WindowOpSetAlwaysOnTop, a, 0, "")
}

func (p *Plugin) setFullscreen(ctx ipc.Context, argsJSON []byte) (any, error) {
	var raw struct {
		Fullscreen *bool `json:"fullscreen"`
	}
	if err := decode(argsJSON, &raw); err != nil {
		return nil, err
	}
	if raw.Fullscreen == nil {
		return nil, ipc.Fail("window.invalid", "window: fullscreen is required")
	}
	a := 0
	if *raw.Fullscreen {
		a = 1
	}
	return nil, p.apply(ctx.Window, cef.WindowOpSetFullscreen, a, 0, "")
}

func (p *Plugin) setSize(ctx ipc.Context, argsJSON []byte) (any, error) {
	width, height, err := parseSize(argsJSON)
	if err != nil {
		return nil, err
	}
	return nil, p.apply(ctx.Window, cef.WindowOpSetSize, width, height, "")
}

func (p *Plugin) flag(query int) ipc.Handler {
	return func(ctx ipc.Context, argsJSON []byte) (any, error) {
		if err := decode(argsJSON, &struct{}{}); err != nil {
			return nil, err
		}
		return p.ask(ctx.Window, query)
	}
}

func (p *Plugin) apply(window string, op, a, b int, text string) error {
	if p.op == nil {
		return ipc.Fail("window.unavailable", "window: cannot apply")
	}
	if window == "" {
		window = "main"
	}
	err := p.op(window, op, a, b, text)
	if err == nil {
		return nil
	}
	if errors.Is(err, cef.ErrWindowRejected) {
		return ipc.Fail("window.invalid", "window: rejected")
	}
	return ipc.Fail("window.unavailable", "%s", err.Error())
}

func (p *Plugin) ask(window string, query int) (bool, error) {
	if p.query == nil {
		return false, ipc.Fail("window.unavailable", "window: cannot query")
	}
	if window == "" {
		window = "main"
	}
	value, err := p.query(window, query)
	if err != nil {
		if errors.Is(err, cef.ErrWindowRejected) {
			return false, ipc.Fail("window.invalid", "window: rejected")
		}
		return false, ipc.Fail("window.unavailable", "%s", err.Error())
	}
	return value, nil
}

func decode(argsJSON []byte, dest any) error {
	trimmed := bytes.TrimSpace(argsJSON)
	if len(trimmed) == 0 {
		trimmed = []byte("{}")
	}
	if string(trimmed) == "null" {
		return ipc.Fail("window.invalid", "window: arguments are invalid")
	}
	dec := json.NewDecoder(bytes.NewReader(trimmed))
	dec.DisallowUnknownFields()
	if err := dec.Decode(dest); err != nil {
		return ipc.Fail("window.invalid", "window: arguments are invalid")
	}
	if dec.More() {
		return ipc.Fail("window.invalid", "window: arguments are invalid")
	}
	return nil
}

func parseSize(argsJSON []byte) (int, int, error) {
	var raw struct {
		Width  *float64 `json:"width"`
		Height *float64 `json:"height"`
	}
	if err := decode(argsJSON, &raw); err != nil {
		return 0, 0, err
	}
	if raw.Width == nil || raw.Height == nil {
		return 0, 0, ipc.Fail("window.invalid", "window: width and height are required")
	}
	width, err := whole(*raw.Width)
	if err != nil {
		return 0, 0, err
	}
	height, err := whole(*raw.Height)
	if err != nil {
		return 0, 0, err
	}
	return width, height, nil
}

func whole(v float64) (int, error) {
	if math.IsNaN(v) || math.IsInf(v, 0) || v != math.Trunc(v) {
		return 0, ipc.Fail("window.invalid", "window: size must be an integer")
	}
	if v < float64(cef.WindowSizeMin) || v > float64(cef.WindowSizeMax) {
		return 0, ipc.Fail("window.invalid", "window: size out of range")
	}
	return int(v), nil
}
