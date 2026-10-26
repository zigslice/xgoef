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

// Limit changes one resize limit. field is a cef.WindowLimit* code.
type Limit func(window string, field, a, b int) error

// LimitQuery reads one resize limit. For a flag, a is 0 or 1.
// For a size, a and b are width and height, and 0,0 means unset.
type LimitQuery func(window string, field int) (a, b int, err error)

// Plugin implements plugin.Plugin.
type Plugin struct {
	op         Op
	query      Query
	limit      Limit
	limitQuery LimitQuery
}

// New returns the window builtin. A nil func makes that call fail
// with window.unavailable.
func New(op Op, query Query, limit Limit, limitQuery LimitQuery) *Plugin {
	return &Plugin{op: op, query: query, limit: limit, limitQuery: limitQuery}
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
		"window:set_resizable",
		"window:is_resizable",
		"window:set_minimizable",
		"window:is_minimizable",
		"window:set_maximizable",
		"window:is_maximizable",
		"window:set_closable",
		"window:is_closable",
		"window:set_min_size",
		"window:set_max_size",
		"window:min_size",
		"window:max_size",
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
	app.Register("window:set_resizable", p.setBool(cef.WindowLimitResizable, "resizable"))
	app.Register("window:is_resizable", p.limitFlag(cef.WindowLimitResizable))
	app.Register("window:set_minimizable", p.setBool(cef.WindowLimitMinimizable, "minimizable"))
	app.Register("window:is_minimizable", p.limitFlag(cef.WindowLimitMinimizable))
	app.Register("window:set_maximizable", p.setBool(cef.WindowLimitMaximizable, "maximizable"))
	app.Register("window:is_maximizable", p.limitFlag(cef.WindowLimitMaximizable))
	app.Register("window:set_closable", p.setBool(cef.WindowLimitClosable, "closable"))
	app.Register("window:is_closable", p.limitFlag(cef.WindowLimitClosable))
	app.Register("window:set_min_size", p.setBound(cef.WindowLimitMinSize, cef.WindowLimitMaxSize))
	app.Register("window:set_max_size", p.setBound(cef.WindowLimitMaxSize, cef.WindowLimitMinSize))
	app.Register("window:min_size", p.sizeQuery(cef.WindowLimitMinSize))
	app.Register("window:max_size", p.sizeQuery(cef.WindowLimitMaxSize))
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

func (p *Plugin) setBool(field int, key string) ipc.Handler {
	return func(ctx ipc.Context, argsJSON []byte) (any, error) {
		on, err := parseBoolKey(argsJSON, key)
		if err != nil {
			return nil, err
		}
		a := 0
		if on {
			a = 1
		}
		return nil, p.change(ctx.Window, field, a, 0)
	}
}

func (p *Plugin) limitFlag(field int) ipc.Handler {
	return func(ctx ipc.Context, argsJSON []byte) (any, error) {
		if err := decode(argsJSON, &struct{}{}); err != nil {
			return nil, err
		}
		a, _, err := p.readLimit(ctx.Window, field)
		if err != nil {
			return nil, err
		}
		return a != 0, nil
	}
}

func (p *Plugin) setBound(field, other int) ipc.Handler {
	return func(ctx ipc.Context, argsJSON []byte) (any, error) {
		width, height, err := parseLimitSize(argsJSON)
		if err != nil {
			return nil, err
		}
		if width != 0 || height != 0 {
			ow, oh, err := p.readLimit(ctx.Window, other)
			if err != nil {
				return nil, err
			}
			if field == cef.WindowLimitMinSize {
				if (ow > 0 && width > ow) || (oh > 0 && height > oh) {
					return nil, ipc.Fail("window.invalid", "window: size out of range")
				}
			} else if (ow > 0 && width < ow) || (oh > 0 && height < oh) {
				return nil, ipc.Fail("window.invalid", "window: size out of range")
			}
		}
		return nil, p.change(ctx.Window, field, width, height)
	}
}

func (p *Plugin) sizeQuery(field int) ipc.Handler {
	return func(ctx ipc.Context, argsJSON []byte) (any, error) {
		if err := decode(argsJSON, &struct{}{}); err != nil {
			return nil, err
		}
		width, height, err := p.readLimit(ctx.Window, field)
		if err != nil {
			return nil, err
		}
		if width == 0 && height == 0 {
			return nil, nil
		}
		return struct {
			Width  int `json:"width"`
			Height int `json:"height"`
		}{width, height}, nil
	}
}

func (p *Plugin) change(window string, field, a, b int) error {
	if p.limit == nil {
		return ipc.Fail("window.unavailable", "window: cannot apply")
	}
	if window == "" {
		window = "main"
	}
	err := p.limit(window, field, a, b)
	if err == nil {
		return nil
	}
	if errors.Is(err, cef.ErrWindowRejected) {
		return ipc.Fail("window.invalid", "window: rejected")
	}
	return ipc.Fail("window.unavailable", "%s", err.Error())
}

func (p *Plugin) readLimit(window string, field int) (int, int, error) {
	if p.limitQuery == nil {
		return 0, 0, ipc.Fail("window.unavailable", "window: cannot query")
	}
	if window == "" {
		window = "main"
	}
	a, b, err := p.limitQuery(window, field)
	if err != nil {
		if errors.Is(err, cef.ErrWindowRejected) {
			return 0, 0, ipc.Fail("window.invalid", "window: rejected")
		}
		return 0, 0, ipc.Fail("window.unavailable", "%s", err.Error())
	}
	return a, b, nil
}

func parseBoolKey(argsJSON []byte, key string) (bool, error) {
	trimmed := bytes.TrimSpace(argsJSON)
	if len(trimmed) == 0 || string(trimmed) == "null" {
		return false, ipc.Fail("window.invalid", "window: arguments are invalid")
	}
	var raw map[string]json.RawMessage
	dec := json.NewDecoder(bytes.NewReader(trimmed))
	dec.UseNumber()
	if err := dec.Decode(&raw); err != nil || dec.More() || raw == nil {
		return false, ipc.Fail("window.invalid", "window: arguments are invalid")
	}
	if len(raw) != 1 {
		return false, ipc.Fail("window.invalid", "window: arguments are invalid")
	}
	body, ok := raw[key]
	if !ok {
		return false, ipc.Fail("window.invalid", "window: arguments are invalid")
	}
	var on bool
	if err := json.Unmarshal(body, &on); err != nil {
		return false, ipc.Fail("window.invalid", "window: arguments are invalid")
	}
	return on, nil
}

func parseLimitSize(argsJSON []byte) (int, int, error) {
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
	width, err := limitSide(*raw.Width)
	if err != nil {
		return 0, 0, err
	}
	height, err := limitSide(*raw.Height)
	if err != nil {
		return 0, 0, err
	}
	if (width == 0) != (height == 0) {
		return 0, 0, ipc.Fail("window.invalid", "window: size out of range")
	}
	return width, height, nil
}

func limitSide(v float64) (int, error) {
	if math.IsNaN(v) || math.IsInf(v, 0) || v != math.Trunc(v) {
		return 0, ipc.Fail("window.invalid", "window: size must be an integer")
	}
	if v == 0 {
		return 0, nil
	}
	if v < float64(cef.WindowSizeMin) || v > float64(cef.WindowSizeMax) {
		return 0, ipc.Fail("window.invalid", "window: size out of range")
	}
	return int(v), nil
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
