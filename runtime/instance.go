package runtime

import (
	"errors"
	"os"

	"github.com/zigslice/xgoef/instance"
)

func (a *App) acquireInstance() error {
	on, err := a.Config.WantSingleInstance()
	if err != nil || !on {
		return err
	}
	dir := a.instDir
	if dir == "" {
		if d := os.Getenv("XDG_RUNTIME_DIR"); d != "" {
			dir = d
		} else {
			dir = os.TempDir()
		}
	}
	label := a.Config.FirstWindow().Label
	if label == "" {
		label = "main"
	}
	ln, err := instance.Acquire(dir, a.Config.Identifier, os.Args, func(args []string) {
		_ = a.FocusWindow(label)
		_ = a.Emit("single-instance", map[string]any{"args": args})
	})
	if errors.Is(err, instance.ErrRunning) {
		return &ExitError{Code: 0}
	}
	if err != nil {
		return err
	}
	a.inst = ln
	return nil
}

func (a *App) closeInstance() {
	if a.inst != nil {
		_ = a.inst.Close()
		a.inst = nil
	}
}
