package runtime

import (
	"fmt"
	"strings"
	"testing"

	"github.com/zigslice/xgoef/acl"
	"github.com/zigslice/xgoef/conf"
	"github.com/zigslice/xgoef/ipc"
)

func TestMenuAndTrayClicks(t *testing.T) {
	app := New(conf.File{
		App: conf.AppSection{
			Menu: []conf.MenuItem{{
				ID:   "file",
				Text: "File",
				Items: []conf.MenuItem{
					{ID: "greet", Text: "Greet", Command: "greet"},
					{ID: "secret", Text: "Secret", Command: "secret"},
					{ID: "off", Text: "Off", Command: "greet", Enabled: boolPtr(false)},
				},
			}},
			Tray: &conf.TrayConfig{
				Tooltip: "xgoef",
				Menu:    []conf.MenuItem{{ID: "greet", Text: "Greet", Command: "greet"}},
			},
		},
		Capabilities: []acl.Capability{{
			Identifier:  "main",
			Windows:     []string{"main"},
			Permissions: []string{"greet", "menu:popup", "tray:popup", "tray:click"},
		}},
	}, nil)
	window := ""
	app.Handle("greet", func(ctx ipc.Context, _ []byte) (any, error) {
		window = ctx.Window
		return "hi", nil
	})
	var menuEv, trayEv string
	app.Events.Listen("menu", func(payload any) { menuEv = fmt.Sprint(payload) })
	app.Events.Listen("tray", func(payload any) { trayEv = fmt.Sprint(payload) })
	if err := app.OpenChrome(); err != nil {
		t.Fatal(err)
	}
	if err := app.ClickMenu("main", "greet"); err != nil {
		t.Fatal(err)
	}
	if window != "main" || !strings.Contains(menuEv, "greet") {
		t.Fatalf("window %q event %q", window, menuEv)
	}
	if err := app.ClickMenu("main", "secret"); err == nil || !strings.Contains(err.Error(), "not permitted") {
		t.Fatalf("acl: %v", err)
	}
	if err := app.ClickMenu("main", "off"); err == nil || !strings.Contains(err.Error(), "disabled") {
		t.Fatalf("disabled: %v", err)
	}
	if err := app.Bridge.PopupMenu("main"); err != nil {
		t.Fatal(err)
	}
	if err := app.Bridge.PopupTray("main"); err != nil {
		t.Fatal(err)
	}
	if err := app.ClickTray("main", ""); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(trayEv, "id") {
		t.Fatalf("tray event %q", trayEv)
	}
	if err := app.ClickTray("", "greet"); err != nil {
		t.Fatal(err)
	}
	if window != "main" || !strings.Contains(trayEv, "greet") {
		t.Fatalf("tray item window %q event %q", window, trayEv)
	}
	out := handleMessage(app.ACL, app.Router, app.Events, "main", `{"type":"invoke","id":1,"cmd":"tray:click","args":{}}`)
	if !strings.Contains(out, `"ok":true`) {
		t.Fatalf("tray click %s", out)
	}
}

func boolPtr(v bool) *bool { return &v }
