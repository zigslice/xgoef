// Package eval runs named scripts that the config registered.
// The page sends a name. It cannot send source text, and it cannot name
// another window. The script runs in the calling window only when that
// window is still on its pinned origin.
package eval

import (
	"encoding/json"
	"errors"

	gate "github.com/zigslice/xgoef/eval"
	"github.com/zigslice/xgoef/ipc"
	"github.com/zigslice/xgoef/plugin"
)

// Runner executes a trusted source in one window. The source is not taken
// from the page.
type Runner func(window, source string) error

// Plugin implements plugin.Plugin.
type Plugin struct {
	scripts map[string]string
	run     Runner
}

// New returns the eval builtin. scripts is copied. A nil runner fails
// eval:run with eval.unavailable.
func New(scripts map[string]string, run Runner) *Plugin {
	cp := make(map[string]string, len(scripts))
	for name, source := range scripts {
		cp[name] = source
	}
	return &Plugin{scripts: cp, run: run}
}

// Name is the builtin plugin id.
func (p *Plugin) Name() string { return "eval" }

// Permissions are the capability ids an app must grant.
func (p *Plugin) Permissions() []string {
	return []string{"eval:run"}
}

// Setup registers eval:run.
func (p *Plugin) Setup(app plugin.Host) error {
	app.Register("eval:run", p.doRun)
	return nil
}

// Teardown is a no-op.
func (p *Plugin) Teardown(plugin.Host) error { return nil }

func (p *Plugin) doRun(ctx ipc.Context, argsJSON []byte) (any, error) {
	name, err := parseName(argsJSON)
	if err != nil {
		return nil, err
	}
	source, ok := p.scripts[name]
	if !ok {
		return nil, ipc.Fail("eval.denied", "eval: script is not registered")
	}
	if p.run == nil {
		return nil, ipc.Fail("eval.unavailable", "eval: unavailable")
	}
	window := ctx.Window
	if window == "" {
		window = "main"
	}
	if err := p.run(window, source); err != nil {
		return nil, mapRun(err)
	}
	return nil, nil
}

func parseName(argsJSON []byte) (string, error) {
	if len(argsJSON) == 0 {
		argsJSON = []byte("{}")
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(argsJSON, &raw); err != nil || raw == nil {
		return "", ipc.Fail("eval.invalid", "eval: name is required")
	}
	body, ok := raw["name"]
	if !ok {
		if len(raw) == 0 {
			return "", ipc.Fail("eval.invalid", "eval: name is required")
		}
		return "", ipc.Fail("eval.denied", "eval: only a script name is accepted")
	}
	if len(raw) != 1 {
		return "", ipc.Fail("eval.denied", "eval: only a script name is accepted")
	}
	var name string
	if err := json.Unmarshal(body, &name); err != nil || gate.ValidateName(name) != nil {
		return "", ipc.Fail("eval.invalid", "eval: name is invalid")
	}
	return name, nil
}

func mapRun(err error) error {
	switch {
	case errors.Is(err, gate.ErrOrigin), errors.Is(err, gate.ErrUnpinned):
		return ipc.Fail("eval.denied", "eval: origin denied")
	case errors.Is(err, gate.ErrRejected):
		return ipc.Fail("eval.invalid", "eval: script rejected")
	default:
		return ipc.Fail("eval.unavailable", "eval: unavailable")
	}
}
