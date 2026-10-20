package runtime

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/zigslice/xgoef/acl"
	"github.com/zigslice/xgoef/conf"
)

func TestBuiltinPluginsFromConfig(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "note.txt"), []byte("scoped"), 0o644); err != nil {
		t.Fatal(err)
	}
	app := New(conf.File{
		Plugins: []string{"fs", "os"},
		Plugin:  conf.PluginSection{FS: conf.FSConfig{Allow: []string{"$APP"}}},
		Capabilities: []acl.Capability{{
			Identifier:  "main",
			Windows:     []string{"main"},
			Permissions: []string{"fs:read_text", "os:platform"},
		}},
	}, nil)
	app.SetBase(dir)
	if err := app.OpenPlugins(); err != nil {
		t.Fatal(err)
	}
	if err := app.Plugins.Setup(app); err != nil {
		t.Fatal(err)
	}
	out := handleMessage(app.ACL, app.Router, nil, "main",
		`{"type":"invoke","id":1,"cmd":"fs:read_text","args":{"path":"note.txt"}}`)
	if !strings.Contains(out, `"ok":true`) || !strings.Contains(out, "scoped") {
		t.Fatalf("%s", out)
	}
	denied := handleMessage(app.ACL, app.Router, nil, "main",
		`{"type":"invoke","id":2,"cmd":"fs:write_text","args":{"path":"note.txt","contents":"x"}}`)
	if !strings.Contains(denied, `"ok":false`) || !strings.Contains(denied, "not permitted") {
		t.Fatalf("%s", denied)
	}
	plat := handleMessage(app.ACL, app.Router, nil, "main",
		`{"type":"invoke","id":3,"cmd":"os:platform","args":{}}`)
	if !strings.Contains(plat, `"ok":true`) {
		t.Fatalf("%s", plat)
	}
}

func TestUnknownBuiltin(t *testing.T) {
	app := New(conf.File{Plugins: []string{"dialog"}}, nil)
	if err := app.OpenPlugins(); err == nil || !strings.Contains(err.Error(), "dialog") {
		t.Fatalf("got %v", err)
	}
}
