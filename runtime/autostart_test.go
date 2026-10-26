package runtime

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/zigslice/xgoef/acl"
	"github.com/zigslice/xgoef/conf"
	"github.com/zigslice/xgoef/plugins/autostart"
)

func TestAutostartUsesConfigArgs(t *testing.T) {
	dir := t.TempDir()
	exe := "/usr/bin/xgoef-hello"
	app := New(conf.File{
		ProductName: "Hello",
		Identifier:  "com.example.auto",
		Plugins:     []string{"autostart"},
		Plugin: conf.PluginSection{
			Autostart: conf.AutostartConfig{Args: []string{"--from-config"}},
		},
		Capabilities: []acl.Capability{{
			Identifier:  "main",
			Windows:     []string{"main"},
			Permissions: []string{"autostart:enable", "autostart:disable", "autostart:is_enabled"},
		}},
	}, nil)
	app.SetExecutable(exe)
	if err := app.OpenPlugins(); err != nil {
		t.Fatal(err)
	}
	var plug *autostart.Plugin
	for _, p := range app.Plugins.Plugins() {
		if got, ok := p.(*autostart.Plugin); ok {
			plug = got
		}
	}
	if plug == nil {
		t.Fatal("autostart plugin was not loaded")
	}
	plug.SetDir(dir)
	if err := app.Plugins.Setup(app); err != nil {
		t.Fatal(err)
	}
	deniedArgs := handleMessage(app.ACL, app.Router, nil, "main",
		`{"type":"invoke","id":1,"cmd":"autostart:enable","args":{"args":["--evil"]}}`)
	if !strings.Contains(deniedArgs, `"code":"autostart.invalid"`) {
		t.Fatalf("%s", deniedArgs)
	}
	entries, err := os.ReadDir(dir)
	if err != nil || len(entries) != 0 {
		t.Fatalf("%v %v", entries, err)
	}
	out := handleMessage(app.ACL, app.Router, nil, "main",
		`{"type":"invoke","id":2,"cmd":"autostart:enable","args":{}}`)
	if !strings.Contains(out, `"ok":true`) {
		t.Fatalf("%s", out)
	}
	body, err := os.ReadFile(filepath.Join(dir, "xgoef-com.example.auto.desktop"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(body)
	if !strings.Contains(text, `Exec="/usr/bin/xgoef-hello" "--from-config"`) || strings.Contains(text, "--evil") || !strings.Contains(text, "Name=Hello") {
		t.Fatalf("%s", text)
	}
	state := handleMessage(app.ACL, app.Router, nil, "main",
		`{"type":"invoke","id":3,"cmd":"autostart:is_enabled","args":{}}`)
	if !strings.Contains(state, `"payload":true`) {
		t.Fatalf("%s", state)
	}
	off := handleMessage(app.ACL, app.Router, nil, "main",
		`{"type":"invoke","id":4,"cmd":"autostart:disable","args":{}}`)
	if !strings.Contains(off, `"ok":true`) {
		t.Fatalf("%s", off)
	}
	state = handleMessage(app.ACL, app.Router, nil, "main",
		`{"type":"invoke","id":5,"cmd":"autostart:is_enabled"}`)
	if !strings.Contains(state, `"payload":false`) {
		t.Fatalf("%s", state)
	}
	other := handleMessage(app.ACL, app.Router, nil, "aux",
		`{"type":"invoke","id":6,"cmd":"autostart:enable","args":{}}`)
	if !strings.Contains(other, `"code":"acl.denied"`) {
		t.Fatalf("%s", other)
	}
}
