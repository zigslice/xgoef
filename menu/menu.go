// Package menu is the app menu and tray menu model.
// Items are addressed by id. Activating an item yields the command to run.
package menu

import (
	"fmt"
	"strings"
)

// Item is one menu row. Separator rows have no command.
// A row with Items is a submenu; it may also carry a command.
// The zero value of Disabled leaves the row enabled.
type Item struct {
	ID        string
	Text      string
	Command   string
	Disabled  bool
	Separator bool
	Items     []Item
}

// Menu is a validated tree with unique ids.
type Menu struct {
	items []Item
	byID  map[string]*Item
}

// Parse checks the tree and indexes every id.
func Parse(items []Item) (*Menu, error) {
	m := &Menu{byID: map[string]*Item{}}
	cloned, err := m.clone(items, "")
	if err != nil {
		return nil, err
	}
	m.items = cloned
	return m, nil
}

// Items is the root list.
func (m *Menu) Items() []Item {
	if m == nil {
		return nil
	}
	return append([]Item(nil), m.items...)
}

// Activate returns the command for id.
// Separators, disabled rows, unknown ids, and submenus without a command fail.
func (m *Menu) Activate(id string) (string, error) {
	if m == nil {
		return "", fmt.Errorf("menu: not configured")
	}
	it := m.byID[id]
	if it == nil {
		return "", fmt.Errorf("menu: unknown item %q", id)
	}
	if it.Separator {
		return "", fmt.Errorf("menu: %s is a separator", id)
	}
	if it.Disabled {
		return "", fmt.Errorf("menu: %s is disabled", id)
	}
	if it.Command == "" {
		return "", fmt.Errorf("menu: %s has no command", id)
	}
	return it.Command, nil
}

// Encode writes a line-oriented form the native shim can parse.
// Columns are id, text, command, flags, parent. flags is "s" and/or "d".
func (m *Menu) Encode() string {
	if m == nil {
		return ""
	}
	var b strings.Builder
	var walk func(items []Item, parent string)
	walk = func(items []Item, parent string) {
		for _, it := range items {
			flags := ""
			if it.Separator {
				flags += "s"
			}
			if it.Disabled {
				flags += "d"
			}
			fmt.Fprintf(&b, "%s\t%s\t%s\t%s\t%s\n", it.ID, it.Text, it.Command, flags, parent)
			if len(it.Items) > 0 {
				walk(it.Items, it.ID)
			}
		}
	}
	walk(m.items, "")
	return b.String()
}

func (m *Menu) clone(items []Item, parent string) ([]Item, error) {
	out := make([]Item, len(items))
	for i, it := range items {
		it.ID = strings.TrimSpace(it.ID)
		it.Text = strings.TrimSpace(it.Text)
		it.Command = strings.TrimSpace(it.Command)
		if it.Separator {
			if it.ID == "" {
				it.ID = fmt.Sprintf("sep-%s-%d", parent, i)
			}
			it.Text = ""
			it.Command = ""
			it.Items = nil
			it.Disabled = true
		} else {
			if it.ID == "" {
				return nil, fmt.Errorf("menu: item under %q is missing an id", parentName(parent))
			}
			if it.Text == "" {
				return nil, fmt.Errorf("menu: %s is missing text", it.ID)
			}
		}
		if strings.ContainsAny(it.ID, "\t\r\n") || strings.ContainsAny(it.Text, "\t\r\n") || strings.ContainsAny(it.Command, "\t\r\n") {
			return nil, fmt.Errorf("menu: %s contains a tab or newline", it.ID)
		}
		if _, ok := m.byID[it.ID]; ok {
			return nil, fmt.Errorf("menu: duplicate id %q", it.ID)
		}
		children, err := m.clone(it.Items, it.ID)
		if err != nil {
			return nil, err
		}
		it.Items = children
		out[i] = it
		m.byID[it.ID] = &out[i]
	}
	return out, nil
}

func parentName(parent string) string {
	if parent == "" {
		return "root"
	}
	return parent
}
