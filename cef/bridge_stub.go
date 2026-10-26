//go:build !llgo

package cef

// NativeBridge is a no-op stub for gc builds / unit tests.
type NativeBridge struct {
	handler     QueryHandler
	onCancel    func(queryID int64)
	menu        string
	trayTip     string
	tray        string
	onMenu      func(windowLabel, kind, id string)
	winState    string
	onState     func(label string, x, y, width, height int, maximized, minimized bool)
	lastCreate  WindowSpec
	displays    string
	moved       bool
	moveLabel   string
	moveX       int
	moveY       int
	printed     bool
	printLabel  string
	devAction   string
	devLabel    string
	devOpen     bool
	initSet     bool
	initLabel   string
	initAllowed string
	initEncoded string
	evalSet     bool
	evalLabel   string
	evalAllowed string
	evalSource  string
	onLoad      func(label, url string)
	winOps      []WindowOpRecord
	winFlags    WindowFlags
	winDown     bool
	limits      map[string]WindowLimits
	limitSets   []WindowLimitRecord
}

// WindowLimitRecord is one successful WindowLimit or SetWindowLimits call.
type WindowLimitRecord struct {
	Label   string
	Field   int
	A       int
	B       int
	Encoded string
}

// WindowOpRecord is one successful WindowOp. Tests use it.
type WindowOpRecord struct {
	Label string
	Op    int
	A     int
	B     int
	Text  string
}

// WindowFlags are the stub's answers for WindowQuery.
type WindowFlags struct {
	Minimized   bool
	Maximized   bool
	Fullscreen  bool
	AlwaysOnTop bool
	Visible     bool
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

func (b *NativeBridge) SetWindowStateNote(fn func(label string, x, y, width, height int, maximized, minimized bool)) {
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

func (b *NativeBridge) WindowOp(label string, op, argA, argB int, text string) error {
	if b.winDown {
		return errBindings("cef: window unavailable")
	}
	if !windowOpAccept(op, argA, argB, text) {
		return ErrWindowRejected
	}
	b.winOps = append(b.winOps, WindowOpRecord{Label: label, Op: op, A: argA, B: argB, Text: text})
	b.winFlags.apply(op, argA)
	return nil
}

// LastWindowOp is the most recent successful WindowOp.
func (b *NativeBridge) LastWindowOp() (WindowOpRecord, bool) {
	if len(b.winOps) == 0 {
		return WindowOpRecord{}, false
	}
	return b.winOps[len(b.winOps)-1], true
}

// WindowOps is every successful WindowOp, in order.
func (b *NativeBridge) WindowOps() []WindowOpRecord {
	out := make([]WindowOpRecord, len(b.winOps))
	copy(out, b.winOps)
	return out
}

// SetWindowFlags replaces the stub query answers.
func (b *NativeBridge) SetWindowFlags(flags WindowFlags) { b.winFlags = flags }

// SetWindowUnavailable makes later window verbs and queries fail.
func (b *NativeBridge) SetWindowUnavailable(down bool) { b.winDown = down }

func (b *NativeBridge) SetWindowLimits(label, encoded string) error {
	lim, err := ParseWindowLimits(encoded)
	if err != nil {
		return err
	}
	if label == "" {
		label = "main"
	}
	if b.limits == nil {
		b.limits = map[string]WindowLimits{}
	}
	b.limits[label] = lim
	b.limitSets = append(b.limitSets, WindowLimitRecord{Label: label, Encoded: encoded})
	return nil
}

func (b *NativeBridge) WindowLimit(label string, field, argA, argB int) error {
	if b.winDown {
		return errBindings("cef: window unavailable")
	}
	if label == "" {
		label = "main"
	}
	lim, ok := b.limits[label]
	if !ok {
		lim = DefaultWindowLimits()
	}
	next, err := lim.Apply(field, argA, argB)
	if err != nil {
		return ErrWindowRejected
	}
	if b.limits == nil {
		b.limits = map[string]WindowLimits{}
	}
	b.limits[label] = next
	b.limitSets = append(b.limitSets, WindowLimitRecord{Label: label, Field: field, A: argA, B: argB})
	return nil
}

// LastWindowLimit is the most recent SetWindowLimits or WindowLimit call.
func (b *NativeBridge) LastWindowLimit() (WindowLimitRecord, bool) {
	if len(b.limitSets) == 0 {
		return WindowLimitRecord{}, false
	}
	return b.limitSets[len(b.limitSets)-1], true
}

func (b *NativeBridge) WindowLimitQuery(label string, field int) (int, int, error) {
	if b.winDown {
		return 0, 0, errBindings("cef: window unavailable")
	}
	if label == "" {
		label = "main"
	}
	lim, ok := b.limits[label]
	if !ok {
		lim = DefaultWindowLimits()
	}
	switch field {
	case WindowLimitResizable:
		return limitBit(lim.Resizable), 0, nil
	case WindowLimitMinimizable:
		return limitBit(lim.Minimizable), 0, nil
	case WindowLimitMaximizable:
		return limitBit(lim.Maximizable), 0, nil
	case WindowLimitClosable:
		return limitBit(lim.Closable), 0, nil
	case WindowLimitMinSize:
		return lim.MinWidth, lim.MinHeight, nil
	case WindowLimitMaxSize:
		return lim.MaxWidth, lim.MaxHeight, nil
	default:
		return 0, 0, ErrWindowRejected
	}
}

func limitBit(on bool) int {
	if on {
		return 1
	}
	return 0
}

func (b *NativeBridge) WindowQuery(label string, query int) (bool, error) {
	_ = label
	if b.winDown {
		return false, errBindings("cef: window unavailable")
	}
	switch query {
	case WindowQueryMinimized:
		return b.winFlags.Minimized, nil
	case WindowQueryMaximized:
		return b.winFlags.Maximized, nil
	case WindowQueryFullscreen:
		return b.winFlags.Fullscreen, nil
	case WindowQueryAlwaysOnTop:
		return b.winFlags.AlwaysOnTop, nil
	case WindowQueryVisible:
		return b.winFlags.Visible, nil
	default:
		return false, errBindings("cef: window query failed")
	}
}

func windowOpAccept(op, a, b int, text string) bool {
	switch op {
	case WindowOpSetTitle:
		return WindowTitleOK(text)
	case WindowOpSetSize:
		return WindowSizeOK(a, b)
	default:
		return WindowOpKnown(op)
	}
}

func (f *WindowFlags) apply(op, a int) {
	switch op {
	case WindowOpMinimize:
		f.Minimized = true
	case WindowOpMaximize:
		f.Maximized = true
		f.Minimized = false
	case WindowOpUnminimize:
		f.Minimized = false
	case WindowOpUnmaximize:
		f.Maximized = false
	case WindowOpToggleMaximize:
		f.Maximized = !f.Maximized
		if f.Maximized {
			f.Minimized = false
		}
	case WindowOpShow:
		f.Visible = true
	case WindowOpHide:
		f.Visible = false
	case WindowOpSetAlwaysOnTop:
		f.AlwaysOnTop = a != 0
	case WindowOpSetFullscreen:
		f.Fullscreen = a != 0
	}
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

func (b *NativeBridge) SetInitScripts(label, allowedURL, encoded string) error {
	b.initSet = true
	b.initLabel = label
	b.initAllowed = allowedURL
	b.initEncoded = encoded
	return nil
}

// LastInit is the most recent SetInitScripts argument.
func (b *NativeBridge) LastInit() (label, allowed, encoded string, ok bool) {
	return b.initLabel, b.initAllowed, b.initEncoded, b.initSet
}

func (b *NativeBridge) Eval(label, allowedURL, source string) error {
	b.evalSet = true
	b.evalLabel = label
	b.evalAllowed = allowedURL
	b.evalSource = source
	return nil
}

// LastEval is the most recent Eval argument.
func (b *NativeBridge) LastEval() (label, allowed, source string, ok bool) {
	return b.evalLabel, b.evalAllowed, b.evalSource, b.evalSet
}

func (b *NativeBridge) SetPageLoad(fn func(label, url string)) { b.onLoad = fn }

// NotePageLoad delivers one load the way OnLoadEnd does after the policy accepts it.
// Tests use it. A nil callback does nothing.
func (b *NativeBridge) NotePageLoad(label, url string) {
	if b == nil || b.onLoad == nil {
		return
	}
	b.onLoad(label, url)
}
