package runtime

import (
	"fmt"

	"github.com/zigslice/xgoef/acl"
	"github.com/zigslice/xgoef/conf"
	"github.com/zigslice/xgoef/ipc"
	"github.com/zigslice/xgoef/menu"
)

func (a *App) loadChrome() error {
	var err error
	if len(a.Config.App.Menu) > 0 {
		a.appMenu, err = menu.Parse(menuItems(a.Config.App.Menu))
		if err != nil {
			return err
		}
	}
	if a.Config.App.Tray != nil {
		a.trayTip = a.Config.App.Tray.Tooltip
		a.trayIcon = a.Config.App.Tray.Icon
		if len(a.Config.App.Tray.Menu) > 0 {
			a.trayMenu, err = menu.Parse(menuItems(a.Config.App.Tray.Menu))
			if err != nil {
				return err
			}
		}
	}
	return nil
}

// OpenChrome publishes the configured menu and tray and registers
// menu:popup, tray:popup, and tray:click. Those commands still need capabilities.
func (a *App) OpenChrome() error {
	if err := a.loadChrome(); err != nil {
		return err
	}
	a.Router.Handle("menu:popup", a.popupHandler("menu"))
	a.Router.Handle("tray:popup", a.popupHandler("tray"))
	a.Router.Handle("tray:click", func(ctx ipc.Context, _ []byte) (any, error) {
		if err := a.ClickTray(ctx.Window, ""); err != nil {
			return nil, err
		}
		return "ok", nil
	})
	if a.Bridge == nil {
		return nil
	}
	a.Bridge.SetMenuClick(func(window, kind, id string) {
		if kind == "tray" {
			_ = a.ClickTray(window, id)
			return
		}
		_ = a.ClickMenu(window, id)
	})
	if a.appMenu != nil {
		a.Bridge.SetMenu(a.appMenu.Encode())
	}
	if a.trayMenu != nil || a.trayTip != "" || a.trayIcon != "" {
		flat := ""
		if a.trayMenu != nil {
			flat = a.trayMenu.Encode()
		}
		a.Bridge.SetTray(a.trayTip, a.trayIcon, flat)
	}
	return nil
}

// ClickMenu runs the command bound to a menu id in the given window.
func (a *App) ClickMenu(window, id string) error {
	return a.activate(window, a.appMenu, id, "menu")
}

// ClickTray runs a tray menu command. An empty id is an icon click and only emits.
func (a *App) ClickTray(window, id string) error {
	if id == "" {
		if a.trayMenu == nil && a.trayTip == "" && a.trayIcon == "" {
			return fmt.Errorf("runtime: tray is not configured")
		}
		return a.EmitTo(windowLabel(window), "tray", map[string]any{"id": ""})
	}
	return a.activate(window, a.trayMenu, id, "tray")
}

func (a *App) activate(window string, m *menu.Menu, id, event string) error {
	if m == nil {
		return fmt.Errorf("runtime: %s is not configured", event)
	}
	window = windowLabel(window)
	cmd, err := m.Activate(id)
	if err != nil {
		return err
	}
	if a.ACL == nil || !a.ACL.Allows(window, cmd) {
		return &acl.DenyError{Window: window, Command: cmd}
	}
	if _, err := a.Router.InvokeFrom(window, cmd, []byte("{}")); err != nil {
		return err
	}
	return a.EmitTo(window, event, map[string]any{"id": id, "command": cmd})
}

func (a *App) popupHandler(kind string) ipc.Handler {
	return func(ctx ipc.Context, _ []byte) (any, error) {
		if a.Bridge == nil {
			return nil, fmt.Errorf("runtime: no bridge")
		}
		var err error
		if kind == "tray" {
			if a.trayMenu == nil && a.trayTip == "" {
				return nil, fmt.Errorf("runtime: tray is not configured")
			}
			err = a.Bridge.PopupTray(windowLabel(ctx.Window))
		} else {
			if a.appMenu == nil {
				return nil, fmt.Errorf("runtime: menu is not configured")
			}
			err = a.Bridge.PopupMenu(windowLabel(ctx.Window))
		}
		if err != nil {
			return nil, err
		}
		return "ok", nil
	}
}

func menuItems(in []conf.MenuItem) []menu.Item {
	out := make([]menu.Item, len(in))
	for i, it := range in {
		out[i] = menu.Item{
			ID:        it.ID,
			Text:      it.Text,
			Command:   it.Command,
			Disabled:  it.Enabled != nil && !*it.Enabled,
			Separator: it.Separator,
			Items:     menuItems(it.Items),
		}
	}
	return out
}

func windowLabel(label string) string {
	if label == "" {
		return "main"
	}
	return label
}
