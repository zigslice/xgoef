package runtime

import (
	"strings"
	"testing"

	"github.com/zigslice/xgoef/acl"
	"github.com/zigslice/xgoef/cef"
	"github.com/zigslice/xgoef/conf"
)

func TestWindowOpsFromConfig(t *testing.T) {
	app := New(conf.File{
		Plugins: []string{"window"},
		Capabilities: []acl.Capability{{
			Identifier: "main",
			Windows:    []string{"main"},
			Permissions: []string{
				"window:minimize",
				"window:toggle_maximize",
				"window:set_title",
				"window:set_size",
				"window:set_always_on_top",
				"window:is_maximized",
				"window:is_always_on_top",
			},
		}},
	}, nil)
	if err := app.OpenPlugins(); err != nil {
		t.Fatal(err)
	}
	if err := app.Plugins.Setup(app); err != nil {
		t.Fatal(err)
	}
	title := handleMessage(app.ACL, app.Router, nil, "main",
		`{"type":"invoke","id":1,"cmd":"window:set_title","args":{"title":"hello"}}`)
	if !strings.Contains(title, `"ok":true`) {
		t.Fatalf("%s", title)
	}
	sized := handleMessage(app.ACL, app.Router, nil, "main",
		`{"type":"invoke","id":2,"cmd":"window:set_size","args":{"width":800,"height":600}}`)
	if !strings.Contains(sized, `"ok":true`) {
		t.Fatalf("%s", sized)
	}
	top := handleMessage(app.ACL, app.Router, nil, "main",
		`{"type":"invoke","id":3,"cmd":"window:set_always_on_top","args":{"alwaysOnTop":true}}`)
	if !strings.Contains(top, `"ok":true`) {
		t.Fatalf("%s", top)
	}
	toggled := handleMessage(app.ACL, app.Router, nil, "main",
		`{"type":"invoke","id":4,"cmd":"window:toggle_maximize","args":{}}`)
	if !strings.Contains(toggled, `"ok":true`) {
		t.Fatalf("%s", toggled)
	}
	maxed := handleMessage(app.ACL, app.Router, nil, "main",
		`{"type":"invoke","id":5,"cmd":"window:is_maximized","args":{}}`)
	if !strings.Contains(maxed, `"payload":true`) {
		t.Fatalf("%s", maxed)
	}
	ontop := handleMessage(app.ACL, app.Router, nil, "main",
		`{"type":"invoke","id":6,"cmd":"window:is_always_on_top","args":{}}`)
	if !strings.Contains(ontop, `"payload":true`) {
		t.Fatalf("%s", ontop)
	}
	bridge := app.Bridge.(*cef.NativeBridge)
	ops := bridge.WindowOps()
	if len(ops) != 4 || ops[0].Text != "hello" || ops[0].Op != cef.WindowOpSetTitle ||
		ops[1].A != 800 || ops[1].B != 600 || ops[2].A != 1 || ops[3].Op != cef.WindowOpToggleMaximize ||
		ops[3].Label != "main" {
		t.Fatalf("%+v", ops)
	}
	denied := handleMessage(app.ACL, app.Router, nil, "aux",
		`{"type":"invoke","id":7,"cmd":"window:minimize","args":{}}`)
	if !strings.Contains(denied, `"code":"acl.denied"`) {
		t.Fatalf("%s", denied)
	}
	bad := handleMessage(app.ACL, app.Router, nil, "main",
		`{"type":"invoke","id":8,"cmd":"window:set_size","args":{"width":0,"height":1}}`)
	if !strings.Contains(bad, `"code":"window.invalid"`) {
		t.Fatalf("%s", bad)
	}
	bridge.SetWindowUnavailable(true)
	down := handleMessage(app.ACL, app.Router, nil, "main",
		`{"type":"invoke","id":9,"cmd":"window:minimize","args":{}}`)
	if !strings.Contains(down, `"code":"window.unavailable"`) {
		t.Fatalf("%s", down)
	}
}
