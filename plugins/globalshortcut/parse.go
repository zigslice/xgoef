package globalshortcut

import (
	"errors"
	"runtime"
	"strconv"
	"strings"
)

const maxShortcutLen = 64

// Shortcut is one accelerator after aliases are folded into a canonical form.
// String always lists modifiers as Control, Alt, Shift, Super, then the key.
type Shortcut struct {
	Control bool
	Alt     bool
	Shift   bool
	Super   bool
	Key     string
}

// String is the canonical accelerator, such as Control+Shift+X.
func (s Shortcut) String() string {
	parts := make([]string, 0, 5)
	if s.Control {
		parts = append(parts, "Control")
	}
	if s.Alt {
		parts = append(parts, "Alt")
	}
	if s.Shift {
		parts = append(parts, "Shift")
	}
	if s.Super {
		parts = append(parts, "Super")
	}
	parts = append(parts, s.Key)
	return strings.Join(parts, "+")
}

var errBadShortcut = errors.New("shortcut: shortcut is not valid")

const (
	modControl = iota
	modAlt
	modShift
	modSuper
)

// Parse reads an accelerator. Matching is case-insensitive, and spaces
// around "+" are ignored. At least one modifier is required so a letter
// key cannot be taken away from typing. CommandOrControl is Control on
// Linux and Super on macOS.
func Parse(raw string) (Shortcut, error) {
	return parseShortcut(raw, runtime.GOOS == "darwin")
}

func parseShortcut(raw string, darwin bool) (Shortcut, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" || len(raw) > maxShortcutLen || strings.ContainsFunc(raw, func(r rune) bool {
		return r < 0x20 || r > 0x7e
	}) {
		return Shortcut{}, errBadShortcut
	}
	var sc Shortcut
	var mods [4]bool
	key := ""
	for _, part := range strings.Split(raw, "+") {
		token := strings.TrimSpace(part)
		if token == "" {
			return Shortcut{}, errBadShortcut
		}
		if mod, ok := modifier(token, darwin); ok {
			if mods[mod] {
				return Shortcut{}, errBadShortcut
			}
			mods[mod] = true
			continue
		}
		if key != "" {
			return Shortcut{}, errBadShortcut
		}
		canon, ok := canonicalKey(token)
		if !ok {
			return Shortcut{}, errBadShortcut
		}
		key = canon
	}
	if key == "" || !(mods[modControl] || mods[modAlt] || mods[modShift] || mods[modSuper]) {
		return Shortcut{}, errBadShortcut
	}
	sc.Control = mods[modControl]
	sc.Alt = mods[modAlt]
	sc.Shift = mods[modShift]
	sc.Super = mods[modSuper]
	sc.Key = key
	return sc, nil
}

func modifier(token string, darwin bool) (int, bool) {
	switch strings.ToLower(token) {
	case "control", "ctrl":
		return modControl, true
	case "alt", "option":
		return modAlt, true
	case "shift":
		return modShift, true
	case "super", "meta", "win", "command", "cmd":
		return modSuper, true
	case "commandorcontrol", "cmdorctrl", "commandorctrl", "cmdorcontrol":
		if darwin {
			return modSuper, true
		}
		return modControl, true
	default:
		return 0, false
	}
}

func canonicalKey(token string) (string, bool) {
	if len(token) == 1 {
		c := token[0]
		switch {
		case c >= 'a' && c <= 'z':
			return strings.ToUpper(token), true
		case c >= 'A' && c <= 'Z':
			return token, true
		case c >= '0' && c <= '9':
			return token, true
		default:
			return "", false
		}
	}
	lower := strings.ToLower(token)
	if strings.HasPrefix(lower, "f") && len(lower) <= 3 {
		n, err := strconv.Atoi(lower[1:])
		if err == nil && n >= 1 && n <= 24 {
			return "F" + strconv.Itoa(n), true
		}
	}
	if strings.HasPrefix(lower, "key") && len(lower) == 4 {
		c := lower[3]
		if c >= 'a' && c <= 'z' {
			return strings.ToUpper(lower[3:]), true
		}
	}
	if strings.HasPrefix(lower, "digit") && len(lower) == 6 {
		c := lower[5]
		if c >= '0' && c <= '9' {
			return lower[5:], true
		}
	}
	if name, ok := keyAliases[lower]; ok {
		return name, true
	}
	return "", false
}

var keyAliases = map[string]string{
	"escape":       "Escape",
	"esc":          "Escape",
	"tab":          "Tab",
	"space":        "Space",
	"spacebar":     "Space",
	"enter":        "Enter",
	"return":       "Enter",
	"backspace":    "Backspace",
	"arrowup":      "ArrowUp",
	"up":           "ArrowUp",
	"arrowdown":    "ArrowDown",
	"down":         "ArrowDown",
	"arrowleft":    "ArrowLeft",
	"left":         "ArrowLeft",
	"arrowright":   "ArrowRight",
	"right":        "ArrowRight",
	"home":         "Home",
	"end":          "End",
	"pageup":       "PageUp",
	"pgup":         "PageUp",
	"pagedown":     "PageDown",
	"pgdn":         "PageDown",
	"insert":       "Insert",
	"ins":          "Insert",
	"delete":       "Delete",
	"del":          "Delete",
	"minus":        "Minus",
	"equal":        "Equal",
	"comma":        "Comma",
	"period":       "Period",
	"dot":          "Period",
	"slash":        "Slash",
	"semicolon":    "Semicolon",
	"quote":        "Quote",
	"apostrophe":   "Quote",
	"bracketleft":  "BracketLeft",
	"bracketright": "BracketRight",
	"backslash":    "Backslash",
	"backquote":    "Backquote",
	"grave":        "Backquote",
}
