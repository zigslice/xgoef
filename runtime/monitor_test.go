package runtime

import (
	"strings"
	"testing"

	"github.com/zigslice/xgoef/acl"
	"github.com/zigslice/xgoef/cef"
	"github.com/zigslice/xgoef/conf"
)

func TestMonitorFromConfig(t *testing.T) {
	app := New(conf.File{
		Plugins: []string{"monitor"},
		Capabilities: []acl.Capability{{
			Identifier: "main",
			Windows:    []string{"main"},
			Permissions: []string{
				"monitor:list",
				"monitor:scale",
				"monitor:move",
			},
		}},
	}, nil)
	if err := app.OpenPlugins(); err != nil {
		t.Fatal(err)
	}
	if err := app.Plugins.Setup(app); err != nil {
		t.Fatal(err)
	}
	listed := handleMessage(app.ACL, app.Router, nil, "main",
		`{"type":"invoke","id":1,"cmd":"monitor:list","args":{}}`)
	if !strings.Contains(listed, `"ok":true`) || !strings.Contains(listed, `"scaleFactor":1`) || !strings.Contains(listed, `"width":1920`) {
		t.Fatalf("%s", listed)
	}
	scale := handleMessage(app.ACL, app.Router, nil, "main",
		`{"type":"invoke","id":2,"cmd":"monitor:scale","args":{}}`)
	if !strings.Contains(scale, `"payload":1`) {
		t.Fatalf("%s", scale)
	}
	denied := handleMessage(app.ACL, app.Router, nil, "aux",
		`{"type":"invoke","id":3,"cmd":"monitor:list","args":{}}`)
	if !strings.Contains(denied, `"code":"acl.denied"`) {
		t.Fatalf("%s", denied)
	}
	moved := handleMessage(app.ACL, app.Router, nil, "main",
		`{"type":"invoke","id":4,"cmd":"monitor:move","args":{"x":10,"y":20}}`)
	if !strings.Contains(moved, `"ok":true`) {
		t.Fatalf("%s", moved)
	}
	bridge := app.Bridge.(*cef.NativeBridge)
	label, x, y, ok := bridge.LastMove()
	if !ok || label != "main" || x != 10 || y != 20 {
		t.Fatalf("%s %d %d %v", label, x, y, ok)
	}
}
