package cef

import (
	"fmt"
	"strings"
)

// Window limit fields. The numbers match cef/shim/xgoef_window_limits.h.
const (
	WindowLimitResizable   = 1
	WindowLimitMinimizable = 2
	WindowLimitMaximizable = 3
	WindowLimitClosable    = 4
	WindowLimitMinSize     = 5
	WindowLimitMaxSize     = 6
)

// WindowLimits is what a Views window delegate reports for resize and size bounds.
// A zero minimum or maximum means that edge is unset.
// The zero struct is not the default: DefaultWindowLimits allows every action.
type WindowLimits struct {
	Resizable   bool
	Minimizable bool
	Maximizable bool
	Closable    bool
	MinWidth    int
	MinHeight   int
	MaxWidth    int
	MaxHeight   int
}

// DefaultWindowLimits allows resize, minimize, maximize, and close, with no size bounds.
func DefaultWindowLimits() WindowLimits {
	return WindowLimits{Resizable: true, Minimizable: true, Maximizable: true, Closable: true}
}

// Encode writes eight comma-separated fields.
// Example: "1,1,1,1,0,0,0,0".
func (l WindowLimits) Encode() (string, error) {
	if err := l.Validate(); err != nil {
		return "", err
	}
	bit := func(on bool) string {
		if on {
			return "1"
		}
		return "0"
	}
	return fmt.Sprintf("%s,%s,%s,%s,%d,%d,%d,%d",
		bit(l.Resizable), bit(l.Minimizable), bit(l.Maximizable), bit(l.Closable),
		l.MinWidth, l.MinHeight, l.MaxWidth, l.MaxHeight), nil
}

// ParseWindowLimits reads Encode. An empty string is DefaultWindowLimits.
func ParseWindowLimits(text string) (WindowLimits, error) {
	if text == "" {
		return DefaultWindowLimits(), nil
	}
	parts := strings.Split(text, ",")
	if len(parts) != 8 {
		return WindowLimits{}, fmt.Errorf("cef: window limits")
	}
	flags := [4]bool{}
	for i := 0; i < 4; i++ {
		switch parts[i] {
		case "1":
			flags[i] = true
		case "0":
		default:
			return WindowLimits{}, fmt.Errorf("cef: window limits")
		}
	}
	nums := [4]int{}
	for i := 0; i < 4; i++ {
		n, err := canonicalInt(parts[i+4])
		if err != nil {
			return WindowLimits{}, fmt.Errorf("cef: window limits")
		}
		nums[i] = n
	}
	lim := WindowLimits{
		Resizable:   flags[0],
		Minimizable: flags[1],
		Maximizable: flags[2],
		Closable:    flags[3],
		MinWidth:    nums[0],
		MinHeight:   nums[1],
		MaxWidth:    nums[2],
		MaxHeight:   nums[3],
	}
	if err := lim.Validate(); err != nil {
		return WindowLimits{}, err
	}
	return lim, nil
}

// Validate checks size pairs and that a set minimum does not exceed a set maximum.
func (l WindowLimits) Validate() error {
	if !limitSizeOK(l.MinWidth, l.MinHeight) || !limitSizeOK(l.MaxWidth, l.MaxHeight) {
		return fmt.Errorf("cef: window limits size")
	}
	if l.MinWidth > 0 && l.MaxWidth > 0 && l.MinWidth > l.MaxWidth {
		return fmt.Errorf("cef: window limits min above max")
	}
	if l.MinHeight > 0 && l.MaxHeight > 0 && l.MinHeight > l.MaxHeight {
		return fmt.Errorf("cef: window limits min above max")
	}
	return nil
}

// Apply changes one field. a and b are 0 or 1 for a flag (b must be 0),
// or the width and height for a size. 0,0 clears that size bound.
func (l WindowLimits) Apply(field, a, b int) (WindowLimits, error) {
	next := l
	switch field {
	case WindowLimitResizable, WindowLimitMinimizable, WindowLimitMaximizable, WindowLimitClosable:
		if (a != 0 && a != 1) || b != 0 {
			return l, ErrWindowRejected
		}
		on := a == 1
		switch field {
		case WindowLimitResizable:
			next.Resizable = on
		case WindowLimitMinimizable:
			next.Minimizable = on
		case WindowLimitMaximizable:
			next.Maximizable = on
		case WindowLimitClosable:
			next.Closable = on
		}
	case WindowLimitMinSize:
		next.MinWidth, next.MinHeight = a, b
	case WindowLimitMaxSize:
		next.MaxWidth, next.MaxHeight = a, b
	default:
		return l, ErrWindowRejected
	}
	if err := next.Validate(); err != nil {
		return l, ErrWindowRejected
	}
	return next, nil
}

func limitSizeOK(width, height int) bool {
	if width == 0 && height == 0 {
		return true
	}
	return width >= WindowSizeMin && width <= WindowSizeMax && height >= WindowSizeMin && height <= WindowSizeMax
}

func canonicalInt(s string) (int, error) {
	if s == "0" {
		return 0, nil
	}
	if s == "" || s[0] == '0' || len(s) > 5 {
		return 0, fmt.Errorf("cef: window limits")
	}
	n := 0
	for _, c := range s {
		if c < '0' || c > '9' {
			return 0, fmt.Errorf("cef: window limits")
		}
		n = n*10 + int(c-'0')
	}
	return n, nil
}
