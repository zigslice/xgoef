package runtime

import (
	"strings"
	"testing"

	"github.com/zigslice/xgoef/acl"
	"github.com/zigslice/xgoef/cef"
	"github.com/zigslice/xgoef/conf"
)

func TestWindowLimitsFromConfig(t *testing.T) {
	off := false
	app := New(conf.File{
		App: conf.AppSection{Windows: []conf.Window{{
			Label:     "main",
			Title:     "t",
			Width:     800,
			Height:    600,
			URL:       "about:blank",
			Resizable: &off,
			MinWidth:  320,
			MinHeight: 240,
		}}},
		Plugins: []string{"window"},
		Capabilities: []acl.Capability{{
			Identifier: "main",
			Windows:    []string{"main"},
			Permissions: []string{
				"window:set_closable",
				"window:is_closable",
				"window:set_min_size",
				"window:min_size",
				"window:set_max_size",
			},
		}},
	}, nil)
	args := strings.Join(app.buildArgs("about:blank"), "\n")
	if !strings.Contains(args, "--xgoef-limits=0,1,1,1,320,240,0,0") {
		t.Fatalf("%s", args)
	}
	if err := app.OpenPlugins(); err != nil {
		t.Fatal(err)
	}
	if err := app.Plugins.Setup(app); err != nil {
		t.Fatal(err)
	}
	closed := handleMessage(app.ACL, app.Router, nil, "main",
		`{"type":"invoke","id":1,"cmd":"window:set_closable","args":{"closable":false}}`)
	if !strings.Contains(closed, `"ok":true`) {
		t.Fatalf("%s", closed)
	}
	flag := handleMessage(app.ACL, app.Router, nil, "main",
		`{"type":"invoke","id":2,"cmd":"window:is_closable","args":{}}`)
	if !strings.Contains(flag, `"payload":false`) {
		t.Fatalf("%s", flag)
	}
	sized := handleMessage(app.ACL, app.Router, nil, "main",
		`{"type":"invoke","id":3,"cmd":"window:set_min_size","args":{"width":400,"height":300}}`)
	if !strings.Contains(sized, `"ok":true`) {
		t.Fatalf("%s", sized)
	}
	got := handleMessage(app.ACL, app.Router, nil, "main",
		`{"type":"invoke","id":4,"cmd":"window:min_size","args":{}}`)
	if !strings.Contains(got, `"width":400`) || !strings.Contains(got, `"height":300`) {
		t.Fatalf("%s", got)
	}
	bad := handleMessage(app.ACL, app.Router, nil, "main",
		`{"type":"invoke","id":5,"cmd":"window:set_max_size","args":{"width":100,"height":100}}`)
	if !strings.Contains(bad, `"code":"window.invalid"`) {
		t.Fatalf("%s", bad)
	}
	denied := handleMessage(app.ACL, app.Router, nil, "aux",
		`{"type":"invoke","id":6,"cmd":"window:set_closable","args":{"closable":true}}`)
	if !strings.Contains(denied, `"code":"acl.denied"`) {
		t.Fatalf("%s", denied)
	}
	bridge := app.Bridge.(*cef.NativeBridge)
	bridge.SetWindowUnavailable(true)
	down := handleMessage(app.ACL, app.Router, nil, "main",
		`{"type":"invoke","id":7,"cmd":"window:is_closable","args":{}}`)
	if !strings.Contains(down, `"code":"window.unavailable"`) {
		t.Fatalf("%s", down)
	}
}
