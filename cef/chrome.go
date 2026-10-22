package cef

import (
	"fmt"
	"strings"
)

// WindowChrome is the frame, color scheme, and page background for one window.
// The zero value is decorated, follows the system theme, and is opaque.
// Theme is "", "system", "light", or "dark".
type WindowChrome struct {
	Frameless   bool
	Theme       string
	Transparent bool
}

// Encode writes decorations,theme,transparent. Empty Theme is system.
// Example: "1,system,0" or "0,dark,1".
func (c WindowChrome) Encode() (string, error) {
	theme := c.Theme
	if theme == "" {
		theme = "system"
	}
	switch theme {
	case "system", "light", "dark":
	default:
		return "", fmt.Errorf("cef: window theme %q", c.Theme)
	}
	decorations := "1"
	if c.Frameless {
		decorations = "0"
	}
	transparent := "0"
	if c.Transparent {
		transparent = "1"
	}
	return decorations + "," + theme + "," + transparent, nil
}

// ParseChrome reads the text produced by Encode.
// An empty string is the default chrome.
func ParseChrome(text string) (WindowChrome, error) {
	if text == "" {
		return WindowChrome{}, nil
	}
	parts := strings.Split(text, ",")
	if len(parts) != 3 {
		return WindowChrome{}, fmt.Errorf("cef: window chrome %q", text)
	}
	var c WindowChrome
	switch parts[0] {
	case "1":
	case "0":
		c.Frameless = true
	default:
		return WindowChrome{}, fmt.Errorf("cef: window chrome %q", text)
	}
	switch parts[1] {
	case "system", "light", "dark":
		c.Theme = parts[1]
	default:
		return WindowChrome{}, fmt.Errorf("cef: window chrome %q", text)
	}
	switch parts[2] {
	case "0":
	case "1":
		c.Transparent = true
	default:
		return WindowChrome{}, fmt.Errorf("cef: window chrome %q", text)
	}
	return c, nil
}
