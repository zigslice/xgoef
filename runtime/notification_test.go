package runtime

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/zigslice/xgoef/acl"
	"github.com/zigslice/xgoef/conf"
	"github.com/zigslice/xgoef/plugins/notification"
)

func TestNotificationFromConfig(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "icon.png"), []byte("png"), 0o644); err != nil {
		t.Fatal(err)
	}
	app := New(conf.File{
		ProductName: "xgoef-hello",
		Plugins:     []string{"notification"},
		Plugin: conf.PluginSection{
			Notification: conf.NotificationConfig{Allow: []string{"$APP"}},
		},
		Capabilities: []acl.Capability{{
			Identifier:  "main",
			Windows:     []string{"main"},
			Permissions: []string{"notification:request_permission", "notification:show"},
		}},
	}, nil)
	app.SetBase(dir)
	if err := app.OpenPlugins(); err != nil {
		t.Fatal(err)
	}
	var plug *notification.Plugin
	for _, p := range app.Plugins.Plugins() {
		if got, ok := p.(*notification.Plugin); ok {
			plug = got
		}
	}
	if plug == nil {
		t.Fatal("notification plugin was not loaded")
	}
	var notes []notification.Note
	plug.SetSender(func(n notification.Note) error {
		notes = append(notes, n)
		return nil
	})
	if err := app.Plugins.Setup(app); err != nil {
		t.Fatal(err)
	}
	denied := handleMessage(app.ACL, app.Router, nil, "main",
		`{"type":"invoke","id":1,"cmd":"notification:show","args":{"title":"early"}}`)
	if !strings.Contains(denied, `"code":"notification.denied"`) || len(notes) != 0 {
		t.Fatalf("%s", denied)
	}
	got := handleMessage(app.ACL, app.Router, nil, "main",
		`{"type":"invoke","id":2,"cmd":"notification:request_permission","args":{}}`)
	if !strings.Contains(got, `"granted"`) {
		t.Fatalf("%s", got)
	}
	got = handleMessage(app.ACL, app.Router, nil, "main",
		`{"type":"invoke","id":3,"cmd":"notification:show","args":{"title":"hello","icon":"icon.png"}}`)
	if !strings.Contains(got, `"ok":true`) || len(notes) != 1 || notes[0].AppName != "xgoef-hello" || notes[0].Icon != filepath.Join(dir, "icon.png") {
		t.Fatalf("%s %+v", got, notes)
	}
	blocked := handleMessage(app.ACL, app.Router, nil, "aux",
		`{"type":"invoke","id":4,"cmd":"notification:show","args":{"title":"no"}}`)
	if !strings.Contains(blocked, `"code":"acl.denied"`) {
		t.Fatalf("%s", blocked)
	}
}
