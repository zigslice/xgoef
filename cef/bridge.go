package cef

// QueryHandler is invoked on the CEF UI thread for each cefQuery request.
// windowLabel is the browser's bound label. Implementations must eventually
// call Respond (any thread).
type QueryHandler func(queryID int64, windowLabel, requestJSON string)

// WindowSpec describes a window to create at runtime.
// Chrome's zero value keeps the title bar, the system theme, and an opaque page.
type WindowSpec struct {
	Label  string
	Title  string
	URL    string
	Width  int
	Height int
	Chrome WindowChrome
	// InitScripts are trusted host sources injected on each main-frame load
	// that is still on this window's URL origin. The page cannot set them.
	// Empty leaves scripts already registered for the label unchanged.
	InitScripts []string
}

// Bridge wires the C MessageRouter shim to Go IPC and window control.
type Bridge interface {
	SetQueryHandler(h QueryHandler)
	// SetCancelHandler is invoked when CEF drops a query (navigation or
	// cefQueryCancel). queryID is the CEF id, not the JSON invoke id.
	SetCancelHandler(h func(queryID int64))
	Respond(queryID int64, ok bool, resultUTF8 string)

	// Events (host → renderer). jsonUTF8 is a full envelope.
	EmitTo(windowLabel, jsonUTF8 string) error
	EmitAll(jsonUTF8 string) error

	// Window control (P1).
	SetPendingLabel(label string)
	CreateWindow(spec WindowSpec) error
	CloseWindow(label string) error
	FocusWindow(label string) error

	// Menu and tray (P2). Encoded text is menu.Menu.Encode.
	// Click is invoked on the UI thread for a native selection.
	SetMenu(encoded string)
	SetTray(tooltip, icon, encoded string)
	PopupMenu(windowLabel string) error
	PopupTray(windowLabel string) error
	SetMenuClick(fn func(windowLabel, kind, id string))

	// Window state (P2). Encoded text is windowstate.Store.Encode.
	// Note runs on the UI thread when a labeled window moves or closes.
	SetWindowState(encoded string)
	SetWindowStateNote(fn func(label string, x, y, width, height int, maximized bool))

	// Displays returns the monitor table for a window label.
	// The text is ParseDisplayTable. DIP coordinates match window-state.
	Displays(windowLabel string) (string, error)
	// MoveWindow moves a Views window's origin to DIP x, y.
	MoveWindow(label string, x, y int) error

	// Print opens the system print dialog for a window label.
	Print(windowLabel string) error
	// OpenDevTools opens developer tools. Already open is success.
	OpenDevTools(windowLabel string) error
	// CloseDevTools closes developer tools. Already closed is success.
	CloseDevTools(windowLabel string) error
	// DevToolsOpen reports whether developer tools are open for a window.
	DevToolsOpen(windowLabel string) (bool, error)

	// SetInitScripts replaces the init scripts for a label.
	// encoded is eval.Encode. allowedURL is the pinned origin.
	// An encoded empty table clears that label.
	SetInitScripts(label, allowedURL, encoded string) error
	// Eval runs source in the main frame when its live URL is still allowedURL.
	// source is trusted host text, never renderer input.
	Eval(label, allowedURL, source string) error

	// SetPageLoad registers the callback for a finished main-frame load.
	// The callback runs on the CEF UI thread. Nil clears it.
	// The label and URL come from the browser. The page cannot set them.
	SetPageLoad(fn func(label, url string))
}
