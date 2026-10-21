// Package process exits or relaunches the current host.
// The page chooses neither the executable nor the arguments.
package process

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/zigslice/xgoef/ipc"
	"github.com/zigslice/xgoef/plugin"
)

// Starter launches the host again. argv[0] is the executable.
type Starter func(exe string, argv []string) error

// Spec configures the plugin. Empty Executable and Args are read from the
// current process at relaunch time. A nil Release skips the single-instance
// handoff. Nil Exit and Start use os.Exit and a direct process start.
type Spec struct {
	Executable string
	Args       []string
	Release    func()
	Exit       func(code int)
	Start      Starter
}

// Plugin implements plugin.Plugin.
type Plugin struct {
	exe     string
	args    []string
	hasArgs bool
	release func()
	exit    func(int)
	start   Starter
}

// New builds the process plugin.
func New(spec Spec) *Plugin {
	p := &Plugin{
		exe:     spec.Executable,
		release: spec.Release,
		exit:    spec.Exit,
		start:   spec.Start,
	}
	if spec.Args != nil {
		p.hasArgs = true
		p.args = append([]string(nil), spec.Args...)
	}
	return p
}

// SetRelease replaces the hook that drops the single-instance lock.
func (p *Plugin) SetRelease(fn func()) {
	if p != nil {
		p.release = fn
	}
}

// SetExit replaces the function that ends this process.
func (p *Plugin) SetExit(fn func(int)) {
	if p != nil {
		p.exit = fn
	}
}

// SetStart replaces the function that launches the new process.
func (p *Plugin) SetStart(fn Starter) {
	if p != nil {
		p.start = fn
	}
}

// Name is the builtin plugin id.
func (p *Plugin) Name() string { return "process" }

// Permissions are the capability ids an app must grant.
func (p *Plugin) Permissions() []string {
	return []string{"process:exit", "process:relaunch"}
}

// Setup registers process:exit and process:relaunch.
func (p *Plugin) Setup(app plugin.Host) error {
	app.Register("process:exit", p.exitCmd)
	app.Register("process:relaunch", p.relaunchCmd)
	return nil
}

// Teardown is a no-op.
func (p *Plugin) Teardown(plugin.Host) error { return nil }

func (p *Plugin) exitCmd(_ ipc.Context, argsJSON []byte) (any, error) {
	code, err := parseExit(argsJSON)
	if err != nil {
		return nil, err
	}
	p.doExit(code)
	return nil, nil
}

func (p *Plugin) relaunchCmd(_ ipc.Context, argsJSON []byte) (any, error) {
	if err := noArgs(argsJSON); err != nil {
		return nil, err
	}
	exe, argv, err := p.plan()
	if err != nil {
		return nil, ipc.Fail("process.relaunch", "process: %s", err.Error())
	}
	if p.release != nil {
		p.release()
	}
	start := p.start
	if start == nil {
		start = defaultStart
	}
	if err := start(exe, argv); err != nil {
		return nil, ipc.Fail("process.relaunch", "process: %s", err.Error())
	}
	p.doExit(0)
	return nil, nil
}

func (p *Plugin) doExit(code int) {
	fn := p.exit
	if fn == nil {
		fn = os.Exit
	}
	fn(code)
}

func (p *Plugin) plan() (string, []string, error) {
	exe := p.exe
	if exe == "" {
		found, err := os.Executable()
		if err != nil {
			return "", nil, err
		}
		exe = found
	}
	abs, err := filepath.Abs(exe)
	if err != nil {
		return "", nil, err
	}
	info, err := os.Stat(abs)
	if err != nil {
		return "", nil, fmt.Errorf("executable %q: %w", abs, err)
	}
	if info.IsDir() {
		return "", nil, fmt.Errorf("executable %q is a directory", abs)
	}
	args := p.args
	if !p.hasArgs {
		args = os.Args
	}
	return abs, relaunchArgs(abs, args), nil
}

// relaunchArgs keeps the original argv but replaces argv[0] and drops CEF
// subprocess switches. A relaunch must come back as the browser process.
func relaunchArgs(exe string, args []string) []string {
	out := []string{exe}
	skip := false
	for i, arg := range args {
		if i == 0 || skip {
			skip = false
			continue
		}
		if arg == "--type" || arg == "--utility-sub-type" {
			skip = true
			continue
		}
		if strings.HasPrefix(arg, "--type=") || strings.HasPrefix(arg, "--utility-sub-type=") {
			continue
		}
		out = append(out, arg)
	}
	return out
}

func parseExit(argsJSON []byte) (int, error) {
	raw, err := objectArgs(argsJSON)
	if err != nil || extraKey(raw, "code") {
		return 0, invalid("process: exit code must be an integer from 0 to 255")
	}
	body, ok := raw["code"]
	if !ok || string(bytes.TrimSpace(body)) == "null" {
		return 0, nil
	}
	var num float64
	if err := json.Unmarshal(body, &num); err != nil || num != float64(int(num)) || num < 0 || num > 255 {
		return 0, invalid("process: exit code must be an integer from 0 to 255")
	}
	return int(num), nil
}

func noArgs(argsJSON []byte) error {
	raw, err := objectArgs(argsJSON)
	if err != nil || len(raw) != 0 {
		return invalid("process: relaunch takes no arguments")
	}
	return nil
}

func objectArgs(argsJSON []byte) (map[string]json.RawMessage, error) {
	trimmed := bytes.TrimSpace(argsJSON)
	if len(trimmed) == 0 || string(trimmed) == "null" {
		return map[string]json.RawMessage{}, nil
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(trimmed, &raw); err != nil || raw == nil {
		return nil, err
	}
	return raw, nil
}

func extraKey(raw map[string]json.RawMessage, allow string) bool {
	for key := range raw {
		if key != allow {
			return true
		}
	}
	return false
}

func invalid(message string) error {
	return ipc.Fail("process.invalid", "%s", message)
}
