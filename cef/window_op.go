package cef

import (
	"errors"
	"unicode/utf8"
)

// Views window verbs. The numbers match cef/shim/xgoef_window_op.h.
const (
	WindowOpMinimize       = 1
	WindowOpMaximize       = 2
	WindowOpUnminimize     = 3
	WindowOpUnmaximize     = 4
	WindowOpToggleMaximize = 5
	WindowOpShow           = 6
	WindowOpHide           = 7
	WindowOpCenter         = 8
	WindowOpSetTitle       = 9
	WindowOpSetAlwaysOnTop = 10
	WindowOpSetFullscreen  = 11
	WindowOpSetSize        = 12

	WindowQueryMinimized   = 1
	WindowQueryMaximized   = 2
	WindowQueryFullscreen  = 3
	WindowQueryAlwaysOnTop = 4
	WindowQueryVisible     = 5

	// WindowTitleMax is the longest title, in bytes.
	WindowTitleMax = 256
	// WindowSizeMin and WindowSizeMax bound one side of a window, in DIP.
	WindowSizeMin = 1
	WindowSizeMax = 16384
)

// ErrWindowRejected means the verb, title, or size was rejected.
var ErrWindowRejected = errors.New("cef: window op rejected")

// WindowTitleOK reports whether title can be passed to a Views window.
// Empty is allowed. The text must be UTF-8, at most WindowTitleMax bytes,
// with no byte below 0x20.
func WindowTitleOK(title string) bool {
	if len(title) > WindowTitleMax || !utf8.ValidString(title) {
		return false
	}
	for i := 0; i < len(title); i++ {
		if title[i] < 0x20 {
			return false
		}
	}
	return true
}

// WindowSizeOK reports whether width and height are inside the DIP limits.
func WindowSizeOK(width, height int) bool {
	return width >= WindowSizeMin && width <= WindowSizeMax &&
		height >= WindowSizeMin && height <= WindowSizeMax
}

// WindowOpKnown reports whether op is a verb this bridge understands.
func WindowOpKnown(op int) bool {
	return op >= WindowOpMinimize && op <= WindowOpSetSize
}

// WindowQueryKnown reports whether query is a flag this bridge understands.
func WindowQueryKnown(query int) bool {
	return query >= WindowQueryMinimized && query <= WindowQueryVisible
}
