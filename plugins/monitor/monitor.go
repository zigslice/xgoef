// Package monitor lists displays and moves a Views window.
// Coordinates are density-independent pixels, matching window-state.
// Physical pixels and the device scale come from CEF.
package monitor

import (
	"encoding/json"
	"math"

	"github.com/zigslice/xgoef/cef"
	"github.com/zigslice/xgoef/ipc"
	"github.com/zigslice/xgoef/plugin"
)

const (
	minPos = -100000
	maxPos = 100000
)

// Snapshot reads the display table for a window label.
type Snapshot func(window string) (string, error)

// Mover moves a Views window to a DIP origin.
type Mover func(window string, x, y int) error

// Plugin implements plugin.Plugin.
type Plugin struct {
	snapshot Snapshot
	move     Mover
}

// New returns the monitor builtin. A nil snapshot makes every command fail
// with monitor.unavailable.
func New(snapshot Snapshot, move Mover) *Plugin {
	return &Plugin{snapshot: snapshot, move: move}
}

// Name is the builtin plugin id.
func (p *Plugin) Name() string { return "monitor" }

// Permissions are the capability ids an app must grant.
func (p *Plugin) Permissions() []string {
	return []string{"monitor:list", "monitor:primary", "monitor:current", "monitor:scale", "monitor:move"}
}

// Setup registers the monitor commands.
func (p *Plugin) Setup(app plugin.Host) error {
	app.Register("monitor:list", p.list)
	app.Register("monitor:primary", p.primary)
	app.Register("monitor:current", p.current)
	app.Register("monitor:scale", p.scale)
	app.Register("monitor:move", p.moveTo)
	return nil
}

// Teardown is a no-op.
func (p *Plugin) Teardown(plugin.Host) error { return nil }

func (p *Plugin) list(ctx ipc.Context, _ []byte) (any, error) {
	table, err := p.read(ctx.Window)
	if err != nil {
		return nil, err
	}
	if table.Displays == nil {
		return []cef.Display{}, nil
	}
	return table.Displays, nil
}

func (p *Plugin) primary(ctx ipc.Context, _ []byte) (any, error) {
	table, err := p.read(ctx.Window)
	if err != nil {
		return nil, err
	}
	for _, d := range table.Displays {
		if d.Primary {
			return d, nil
		}
	}
	return nil, nil
}

func (p *Plugin) current(ctx ipc.Context, _ []byte) (any, error) {
	table, err := p.read(ctx.Window)
	if err != nil {
		return nil, err
	}
	if !table.HasWindow {
		return nil, nil
	}
	for _, d := range table.Displays {
		if d.ID == table.WindowID {
			return d, nil
		}
	}
	return nil, nil
}

func (p *Plugin) scale(ctx ipc.Context, _ []byte) (any, error) {
	table, err := p.read(ctx.Window)
	if err != nil {
		return nil, err
	}
	if table.HasWindow && table.WindowScale > 0 {
		return table.WindowScale, nil
	}
	for _, d := range table.Displays {
		if d.Primary {
			return d.ScaleFactor, nil
		}
	}
	return nil, ipc.Fail("monitor.unavailable", "monitor: no display")
}

func (p *Plugin) moveTo(ctx ipc.Context, argsJSON []byte) (any, error) {
	x, y, err := parseMove(argsJSON)
	if err != nil {
		return nil, err
	}
	if p.move == nil {
		return nil, ipc.Fail("monitor.unavailable", "monitor: cannot move")
	}
	window := ctx.Window
	if window == "" {
		window = "main"
	}
	if err := p.move(window, x, y); err != nil {
		return nil, ipc.Fail("monitor.unavailable", "%s", err.Error())
	}
	return nil, nil
}

func (p *Plugin) read(window string) (cef.DisplayTable, error) {
	if p.snapshot == nil {
		return cef.DisplayTable{}, ipc.Fail("monitor.unavailable", "monitor: no display")
	}
	text, err := p.snapshot(window)
	if err != nil {
		return cef.DisplayTable{}, ipc.Fail("monitor.unavailable", "%s", err.Error())
	}
	table, err := cef.ParseDisplayTable(text)
	if err != nil {
		return cef.DisplayTable{}, ipc.Fail("monitor.unavailable", "%s", err.Error())
	}
	return table, nil
}

func parseMove(argsJSON []byte) (int, int, error) {
	if len(argsJSON) == 0 {
		argsJSON = []byte("{}")
	}
	var raw struct {
		X *float64 `json:"x"`
		Y *float64 `json:"y"`
	}
	if err := json.Unmarshal(argsJSON, &raw); err != nil || raw.X == nil || raw.Y == nil {
		return 0, 0, ipc.Fail("monitor.invalid", "monitor: x and y are required")
	}
	x, err := whole(*raw.X)
	if err != nil {
		return 0, 0, err
	}
	y, err := whole(*raw.Y)
	if err != nil {
		return 0, 0, err
	}
	return x, y, nil
}

func whole(v float64) (int, error) {
	if math.IsNaN(v) || math.IsInf(v, 0) || v != math.Trunc(v) {
		return 0, ipc.Fail("monitor.invalid", "monitor: position must be an integer")
	}
	if v < minPos || v > maxPos {
		return 0, ipc.Fail("monitor.invalid", "monitor: position out of range")
	}
	return int(v), nil
}
