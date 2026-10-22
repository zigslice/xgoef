//go:build !llgo

package cef

// NativeBridge is a no-op stub for gc builds / unit tests.
type NativeBridge struct {
	handler    QueryHandler
	onCancel   func(queryID int64)
	menu       string
	trayTip    string
	tray       string
	onMenu     func(windowLabel, kind, id string)
	winState   string
	onState    func(label string, x, y, width, height int, maximized bool)
	lastCreate WindowSpec
}

// NewNativeBridge returns a stub bridge when not building with LLGo.
func NewNativeBridge() *NativeBridge {
	return &NativeBridge{}
}

func (b *NativeBridge) SetQueryHandler(h QueryHandler) { b.handler = h }

func (b *NativeBridge) SetCancelHandler(h func(queryID int64)) { b.onCancel = h }

func (b *NativeBridge) Respond(queryID int64, ok bool, resultUTF8 string) {}

func (b *NativeBridge) EmitTo(windowLabel, jsonUTF8 string) error { return nil }

func (b *NativeBridge) EmitAll(jsonUTF8 string) error { return nil }

func (b *NativeBridge) SetPendingLabel(label string) {}

func (b *NativeBridge) CreateWindow(spec WindowSpec) error {
	if _, err := spec.Chrome.Encode(); err != nil {
		return err
	}
	b.lastCreate = spec
	return nil
}

// LastCreate is the most recent CreateWindow argument. Tests use it.
func (b *NativeBridge) LastCreate() WindowSpec { return b.lastCreate }

func (b *NativeBridge) CloseWindow(label string) error { return nil }

func (b *NativeBridge) FocusWindow(label string) error { return nil }

func (b *NativeBridge) SetMenu(encoded string) { b.menu = encoded }

func (b *NativeBridge) SetTray(tooltip, icon, encoded string) {
	_ = icon
	b.trayTip = tooltip
	b.tray = encoded
}

func (b *NativeBridge) PopupMenu(windowLabel string) error {
	_ = windowLabel
	if b.menu == "" {
		return errBindings("cef: menu is empty")
	}
	return nil
}

func (b *NativeBridge) PopupTray(windowLabel string) error {
	_ = windowLabel
	if b.tray == "" && b.trayTip == "" {
		return errBindings("cef: tray is empty")
	}
	return nil
}

func (b *NativeBridge) SetMenuClick(fn func(windowLabel, kind, id string)) { b.onMenu = fn }

func (b *NativeBridge) SetWindowState(encoded string) { b.winState = encoded }

// WindowState is the encoded table last published to the stub.
func (b *NativeBridge) WindowState() string { return b.winState }

func (b *NativeBridge) SetWindowStateNote(fn func(label string, x, y, width, height int, maximized bool)) {
	b.onState = fn
}
