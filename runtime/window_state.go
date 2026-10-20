package runtime

import (
	"fmt"
	"path/filepath"

	"github.com/zigslice/xgoef/cef"
	"github.com/zigslice/xgoef/windowstate"
)

// OpenWindowState loads .xgoef/window-state.json and publishes it to the shim.
// A missing file starts empty. Broken JSON is replaced the next time a window
// is saved. app.windowState false turns remembering off.
func (a *App) OpenWindowState() error {
	if !a.Config.App.RememberWindows() {
		a.winState = nil
		a.publishWindowState()
		return nil
	}
	path := filepath.Join(a.baseDir(), ".xgoef", "window-state.json")
	st, err := windowstate.Load(path)
	if err != nil {
		st = windowstate.New(path)
	}
	a.winState = st
	a.publishWindowState()
	return nil
}

func (a *App) publishWindowState() {
	if a.Bridge == nil {
		return
	}
	encoded := ""
	if a.winState != nil {
		encoded = a.winState.Encode()
	}
	a.Bridge.SetWindowState(encoded)
	if a.winState == nil {
		a.Bridge.SetWindowStateNote(nil)
		return
	}
	a.Bridge.SetWindowStateNote(func(label string, x, y, width, height int, maximized bool) {
		_ = a.noteWindow(label, x, y, width, height, maximized)
	})
}

func (a *App) noteWindow(label string, x, y, width, height int, maximized bool) error {
	if a.winState == nil {
		return fmt.Errorf("runtime: window state is off")
	}
	if err := a.winState.Note(label, x, y, width, height, maximized); err != nil {
		return err
	}
	if a.Bridge != nil {
		a.Bridge.SetWindowState(a.winState.Encode())
	}
	return nil
}

func (a *App) applySavedSize(spec *cef.WindowSpec) {
	if a.winState == nil || spec == nil {
		return
	}
	label := spec.Label
	if label == "" {
		label = "main"
	}
	p, ok := a.winState.Get(label)
	if !ok || !p.Positioned() {
		return
	}
	spec.Width = p.Width
	spec.Height = p.Height
}

func (a *App) launchSize(label string, width, height int) (int, int) {
	if a.winState != nil {
		if p, ok := a.winState.Get(label); ok && p.Positioned() {
			return p.Width, p.Height
		}
	}
	return width, height
}
