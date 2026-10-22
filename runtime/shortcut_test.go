package runtime

import (
	"strings"
	"testing"

	"github.com/zigslice/xgoef/acl"
	"github.com/zigslice/xgoef/conf"
	"github.com/zigslice/xgoef/plugins/globalshortcut"
)

func TestShortcutFromConfig(t *testing.T) {
	app := New(conf.File{
		Plugins: []string{"global-shortcut"},
		Plugin: conf.PluginSection{
			GlobalShortcut: conf.ShortcutConfig{Allow: []string{"ctrl+shift+x"}},
		},
		Capabilities: []acl.Capability{{
			Identifier: "main",
			Windows:    []string{"main"},
			Permissions: []string{
				"global-shortcut:register",
				"global-shortcut:is_registered",
			},
		}},
	}, nil)
	if err := app.OpenPlugins(); err != nil {
		t.Fatal(err)
	}
	var plug *globalshortcut.Plugin
	for _, p := range app.Plugins.Plugins() {
		if got, ok := p.(*globalshortcut.Plugin); ok {
			plug = got
		}
	}
	if plug == nil {
		t.Fatal("global-shortcut plugin was not loaded")
	}
	mem := globalshortcut.NewMemory()
	plug.SetSession(mem)
	if err := app.Plugins.Setup(app); err != nil {
		t.Fatal(err)
	}
	var got []string
	app.Events.Listen(globalshortcut.Event, func(payload any) {
		m := payload.(map[string]string)
		got = append(got, m["shortcut"]+" "+m["state"])
	})
	denied := handleMessage(app.ACL, app.Router, nil, "main",
		`{"type":"invoke","id":1,"cmd":"global-shortcut:register","args":{"shortcut":"Alt+F4"}}`)
	if !strings.Contains(denied, `"code":"shortcut.denied"`) || len(mem.Grabs()) != 0 {
		t.Fatalf("%s", denied)
	}
	ok := handleMessage(app.ACL, app.Router, nil, "main",
		`{"type":"invoke","id":2,"cmd":"global-shortcut:register","args":{"shortcut":"Control+Shift+X"}}`)
	if !strings.Contains(ok, `"ok":true`) || len(mem.Grabs()) != 1 {
		t.Fatalf("%s %v", ok, mem.Grabs())
	}
	sc, err := globalshortcut.Parse("Control+Shift+X")
	if err != nil {
		t.Fatal(err)
	}
	mem.Press(sc)
	if len(got) != 1 || got[0] != "Control+Shift+X Pressed" {
		t.Fatalf("%v", got)
	}
	registered := handleMessage(app.ACL, app.Router, nil, "main",
		`{"type":"invoke","id":3,"cmd":"global-shortcut:is_registered","args":{"shortcut":"Control+Shift+X"}}`)
	if !strings.Contains(registered, `"payload":true`) {
		t.Fatalf("%s", registered)
	}
	blocked := handleMessage(app.ACL, app.Router, nil, "aux",
		`{"type":"invoke","id":4,"cmd":"global-shortcut:register","args":{"shortcut":"Control+Shift+X"}}`)
	if !strings.Contains(blocked, `"code":"acl.denied"`) {
		t.Fatalf("%s", blocked)
	}
}

func TestShortcutAllowMustParse(t *testing.T) {
	app := New(conf.File{
		Plugins: []string{"global-shortcut"},
		Plugin:  conf.PluginSection{GlobalShortcut: conf.ShortcutConfig{Allow: []string{"nope"}}},
	}, nil)
	if err := app.OpenPlugins(); err == nil || !strings.Contains(err.Error(), "not valid") {
		t.Fatal(err)
	}
}
