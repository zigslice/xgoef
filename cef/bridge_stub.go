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
	displays   string
	moved      bool
	moveLabel  string
	moveX      int
	moveY      int
	printed    bool
	printLabel string
	devAction  string
	devLabel   string
	devOpen    bool
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

func (b *NativeBridge) Displays(windowLabel string) (string, error) {
	_ = windowLabel
	if b.displays != "" {
		return b.displays, nil
	}
	return DefaultDisplayTable(), nil
}

// SetDisplays replaces the stub monitor table. Tests use it.
func (b *NativeBridge) SetDisplays(text string) { b.displays = text }

func (b *NativeBridge) MoveWindow(label string, x, y int) error {
	b.moved = true
	b.moveLabel = label
	b.moveX = x
	b.moveY = y
	return nil
}

// LastMove is the most recent MoveWindow argument.
func (b *NativeBridge) LastMove() (label string, x, y int, ok bool) {
	return b.moveLabel, b.moveX, b.moveY, b.moved
}

func (b *NativeBridge) Print(windowLabel string) error {
	b.printed = true
	b.printLabel = windowLabel
	return nil
}

// LastPrint is the most recent Print argument.
func (b *NativeBridge) LastPrint() (label string, ok bool) {
	return b.printLabel, b.printed
}

func (b *NativeBridge) OpenDevTools(windowLabel string) error {
	b.devAction = "open"
	b.devLabel = windowLabel
	b.devOpen = true
	return nil
}

func (b *NativeBridge) CloseDevTools(windowLabel string) error {
	b.devAction = "close"
	b.devLabel = windowLabel
	b.devOpen = false
	return nil
}

func (b *NativeBridge) DevToolsOpen(windowLabel string) (bool, error) {
	b.devAction = "query"
	b.devLabel = windowLabel
	return b.devOpen, nil
}

// LastDevTools is the most recent developer-tools call.
func (b *NativeBridge) LastDevTools() (action, label string, open bool) {
	return b.devAction, b.devLabel, b.devOpen
}
