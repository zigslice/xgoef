//go:build llgo

package cef

import (
	"fmt"
	"unsafe"

	"github.com/goplus/lib/c"
	"github.com/zigslice/xgoef/eval"
)

var _ = unsafe.Sizeof(0)

//llgo:cdecl
type ipcQueryFn func(queryID int64, windowLabel, request *c.Char)

// NativeBridge is the LLGo-backed IPC bridge to the C++ MessageRouter shim.
type NativeBridge struct {
	handler  QueryHandler
	onCancel func(queryID int64)
}

// activeBridge is the process-wide bridge registered with the C shim.
// C function pointers cannot carry a Go receiver, so we keep one global.
var activeBridge *NativeBridge

// NewNativeBridge prepares the CEF IPC bridge (call SetQueryHandler before RunLoop).
func NewNativeBridge() *NativeBridge {
	return &NativeBridge{}
}

// SetQueryHandler registers the Go callback invoked for each cefQuery.
func (b *NativeBridge) SetQueryHandler(h QueryHandler) {
	b.handler = h
	activeBridge = b
	if h == nil {
		xgoefIPCSetHandler(nil)
		return
	}
	xgoefIPCSetHandler(nativeOnQuery)
}

// SetCancelHandler registers the callback for CEF OnQueryCanceled.
func (b *NativeBridge) SetCancelHandler(h func(queryID int64)) {
	b.onCancel = h
	activeBridge = b
	if h == nil {
		xgoefIPCSetCancelHandler(nil)
		return
	}
	xgoefIPCSetCancelHandler(nativeOnCancel)
}

//llgo:cdecl
func nativeOnCancel(queryID int64) {
	b := activeBridge
	if b == nil || b.onCancel == nil {
		return
	}
	b.onCancel(queryID)
}

//llgo:cdecl
func nativeOnQuery(queryID int64, windowLabel, request *c.Char) {
	b := activeBridge
	if b == nil || b.handler == nil {
		xgoefIPCRespond(queryID, 0, c.AllocaCStr("xgoef host bridge not connected"))
		return
	}
	label := "main"
	if windowLabel != nil {
		if s := c.GoString(windowLabel); s != "" {
			label = s
		}
	}
	req := ""
	if request != nil {
		req = c.GoString(request)
	}
	b.handler(queryID, label, req)
}

// Respond completes a pending query (ok → Success JSON; else Failure message).
func (b *NativeBridge) Respond(queryID int64, ok bool, resultUTF8 string) {
	flag := c.Int(0)
	if ok {
		flag = 1
	}
	xgoefIPCRespond(queryID, flag, c.AllocaCStr(resultUTF8))
}

func (b *NativeBridge) EmitTo(windowLabel, jsonUTF8 string) error {
	if rc := xgoefIPCEmitTo(c.AllocaCStr(windowLabel), c.AllocaCStr(jsonUTF8)); rc != 0 {
		return fmt.Errorf("cef: emit_to %q failed (%d)", windowLabel, int(rc))
	}
	return nil
}

func (b *NativeBridge) EmitAll(jsonUTF8 string) error {
	if rc := xgoefIPCEmitAll(c.AllocaCStr(jsonUTF8)); rc != 0 {
		return fmt.Errorf("cef: emit_all failed (%d)", int(rc))
	}
	return nil
}

func (b *NativeBridge) SetPendingLabel(label string) {
	xgoefWindowSetPendingLabel(c.AllocaCStr(label))
}

func (b *NativeBridge) CreateWindow(spec WindowSpec) error {
	if spec.Label == "" {
		spec.Label = "main"
	}
	if spec.Width <= 0 {
		spec.Width = 960
	}
	if spec.Height <= 0 {
		spec.Height = 640
	}
	chrome, err := spec.Chrome.Encode()
	if err != nil {
		return err
	}
	rc := xgoefWindowCreate(
		c.AllocaCStr(spec.Label),
		c.AllocaCStr(spec.Title),
		c.AllocaCStr(spec.URL),
		c.Int(spec.Width),
		c.Int(spec.Height),
		c.AllocaCStr(chrome),
	)
	if rc != 0 {
		return fmt.Errorf("cef: create window %q failed (%d)", spec.Label, int(rc))
	}
	return nil
}

func (b *NativeBridge) CloseWindow(label string) error {
	if rc := xgoefWindowClose(c.AllocaCStr(label)); rc != 0 {
		return fmt.Errorf("cef: close window %q failed (%d)", label, int(rc))
	}
	return nil
}

func (b *NativeBridge) FocusWindow(label string) error {
	if rc := xgoefWindowFocus(c.AllocaCStr(label)); rc != 0 {
		return fmt.Errorf("cef: focus window %q failed (%d)", label, int(rc))
	}
	return nil
}

func (b *NativeBridge) SetMenu(encoded string) {
	xgoefMenuSet(c.AllocaCStr(encoded))
}

func (b *NativeBridge) SetTray(tooltip, icon, encoded string) {
	xgoefTraySet(c.AllocaCStr(tooltip), c.AllocaCStr(icon), c.AllocaCStr(encoded))
}

func (b *NativeBridge) PopupMenu(windowLabel string) error {
	if rc := xgoefMenuPopup(c.AllocaCStr(windowLabel)); rc != 0 {
		return fmt.Errorf("cef: menu popup failed (%d)", int(rc))
	}
	return nil
}

func (b *NativeBridge) PopupTray(windowLabel string) error {
	if rc := xgoefTrayPopup(c.AllocaCStr(windowLabel)); rc != 0 {
		return fmt.Errorf("cef: tray popup failed (%d)", int(rc))
	}
	return nil
}

func (b *NativeBridge) SetMenuClick(fn func(windowLabel, kind, id string)) {
	menuClick = fn
	if fn == nil {
		xgoefMenuSetClick(nil)
		return
	}
	xgoefMenuSetClick(nativeMenuClick)
}

func (b *NativeBridge) SetWindowState(encoded string) {
	xgoefWindowStateSet(c.AllocaCStr(encoded))
}

func (b *NativeBridge) Displays(windowLabel string) (string, error) {
	text := xgoefDisplays(c.AllocaCStr(windowLabel))
	if text == nil {
		return "", fmt.Errorf("cef: displays unavailable")
	}
	defer xgoefDisplaysFree(text)
	return c.GoString(text), nil
}

func (b *NativeBridge) MoveWindow(label string, x, y int) error {
	if rc := xgoefWindowMove(c.AllocaCStr(label), c.Int(x), c.Int(y)); rc != 0 {
		return fmt.Errorf("cef: move window %q failed", label)
	}
	return nil
}

func (b *NativeBridge) Print(windowLabel string) error {
	if rc := xgoefWindowPrint(c.AllocaCStr(windowLabel)); rc != 0 {
		return fmt.Errorf("cef: print %q failed", windowLabel)
	}
	return nil
}

func (b *NativeBridge) OpenDevTools(windowLabel string) error {
	if rc := xgoefDevToolsOpen(c.AllocaCStr(windowLabel)); rc != 0 {
		return fmt.Errorf("cef: open devtools %q failed", windowLabel)
	}
	return nil
}

func (b *NativeBridge) CloseDevTools(windowLabel string) error {
	if rc := xgoefDevToolsClose(c.AllocaCStr(windowLabel)); rc != 0 {
		return fmt.Errorf("cef: close devtools %q failed", windowLabel)
	}
	return nil
}

func (b *NativeBridge) DevToolsOpen(windowLabel string) (bool, error) {
	switch xgoefDevToolsIsOpen(c.AllocaCStr(windowLabel)) {
	case 1:
		return true, nil
	case 0:
		return false, nil
	default:
		return false, fmt.Errorf("cef: devtools query %q failed", windowLabel)
	}
}

func (b *NativeBridge) SetInitScripts(label, allowedURL, encoded string) error {
	if rc := xgoefEvalSetInit(c.AllocaCStr(label), c.AllocaCStr(allowedURL), c.AllocaCStr(encoded)); rc != 0 {
		return fmt.Errorf("cef: eval init failed")
	}
	return nil
}

func (b *NativeBridge) Eval(label, allowedURL, source string) error {
	switch xgoefEvalRun(c.AllocaCStr(label), c.AllocaCStr(allowedURL), c.AllocaCStr(source)) {
	case 0:
		return nil
	case -2:
		return eval.ErrOrigin
	case -3:
		return eval.ErrRejected
	default:
		return eval.ErrUnavailable
	}
}

func (b *NativeBridge) SetWindowStateNote(fn func(label string, x, y, width, height int, maximized bool)) {
	windowStateNote = fn
	if fn == nil {
		xgoefWindowStateSetNote(nil)
		return
	}
	xgoefWindowStateSetNote(nativeWindowState)
}

var windowStateNote func(label string, x, y, width, height int, maximized bool)

//llgo:cdecl
func nativeWindowState(label *c.Char, x, y, width, height, maximized c.Int) {
	fn := windowStateNote
	if fn == nil {
		return
	}
	fn(goStr(label), int(x), int(y), int(width), int(height), maximized != 0)
}

var menuClick func(windowLabel, kind, id string)

//llgo:cdecl
func nativeMenuClick(window, kind, id *c.Char) {
	fn := menuClick
	if fn == nil {
		return
	}
	fn(goStr(window), goStr(kind), goStr(id))
}

func goStr(p *c.Char) string {
	if p == nil {
		return ""
	}
	return c.GoString(p)
}

//go:linkname xgoefIPCSetHandler C.xgoef_ipc_set_handler
func xgoefIPCSetHandler(fn ipcQueryFn)

//llgo:cdecl
type ipcCancelFn func(queryID int64)

//go:linkname xgoefIPCSetCancelHandler C.xgoef_ipc_set_cancel_handler
func xgoefIPCSetCancelHandler(fn ipcCancelFn)

//go:linkname xgoefIPCRespond C.xgoef_ipc_respond
func xgoefIPCRespond(queryID int64, ok c.Int, result *c.Char)

//go:linkname xgoefIPCEmitTo C.xgoef_ipc_emit_to
func xgoefIPCEmitTo(label, json *c.Char) c.Int

//go:linkname xgoefIPCEmitAll C.xgoef_ipc_emit_all
func xgoefIPCEmitAll(json *c.Char) c.Int

//go:linkname xgoefWindowSetPendingLabel C.xgoef_window_set_pending_label
func xgoefWindowSetPendingLabel(label *c.Char)

//go:linkname xgoefWindowCreate C.xgoef_window_create
func xgoefWindowCreate(label, title, url *c.Char, width, height c.Int, chrome *c.Char) c.Int

//go:linkname xgoefWindowClose C.xgoef_window_close
func xgoefWindowClose(label *c.Char) c.Int

//go:linkname xgoefWindowFocus C.xgoef_window_focus
func xgoefWindowFocus(label *c.Char) c.Int

//llgo:cdecl
type menuClickFn func(window, kind, id *c.Char)

//go:linkname xgoefMenuSet C.xgoef_menu_set
func xgoefMenuSet(encoded *c.Char)

//go:linkname xgoefTraySet C.xgoef_tray_set
func xgoefTraySet(tooltip, icon, encoded *c.Char)

//go:linkname xgoefMenuPopup C.xgoef_menu_popup
func xgoefMenuPopup(windowLabel *c.Char) c.Int

//go:linkname xgoefTrayPopup C.xgoef_tray_popup
func xgoefTrayPopup(windowLabel *c.Char) c.Int

//go:linkname xgoefMenuSetClick C.xgoef_menu_set_click
func xgoefMenuSetClick(fn menuClickFn)

//llgo:cdecl
type windowStateFn func(label *c.Char, x, y, width, height, maximized c.Int)

//go:linkname xgoefWindowStateSet C.xgoef_window_state_set
func xgoefWindowStateSet(encoded *c.Char)

//go:linkname xgoefWindowStateSetNote C.xgoef_window_state_set_note
func xgoefWindowStateSetNote(fn windowStateFn)

//go:linkname xgoefDisplays C.xgoef_displays
func xgoefDisplays(windowLabel *c.Char) *c.Char

//go:linkname xgoefDisplaysFree C.xgoef_displays_free
func xgoefDisplaysFree(text *c.Char)

//go:linkname xgoefWindowMove C.xgoef_window_move
func xgoefWindowMove(label *c.Char, x, y c.Int) c.Int

//go:linkname xgoefWindowPrint C.xgoef_window_print
func xgoefWindowPrint(label *c.Char) c.Int

//go:linkname xgoefDevToolsOpen C.xgoef_devtools_open
func xgoefDevToolsOpen(label *c.Char) c.Int

//go:linkname xgoefDevToolsClose C.xgoef_devtools_close
func xgoefDevToolsClose(label *c.Char) c.Int

//go:linkname xgoefDevToolsIsOpen C.xgoef_devtools_is_open
func xgoefDevToolsIsOpen(label *c.Char) c.Int

//go:linkname xgoefEvalSetInit C.xgoef_eval_set_init
func xgoefEvalSetInit(label, allowedURL, encoded *c.Char) c.Int

//go:linkname xgoefEvalRun C.xgoef_eval_run
func xgoefEvalRun(label, allowedURL, source *c.Char) c.Int
