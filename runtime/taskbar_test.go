package runtime

import (
	"strings"
	"testing"

	"github.com/zigslice/xgoef/acl"
	"github.com/zigslice/xgoef/cef"
	"github.com/zigslice/xgoef/conf"
)

func TestTaskbarFromConfig(t *testing.T) {
	launch := New(conf.File{
		App: conf.AppSection{Windows: []conf.Window{{
			Label: "main", Title: "t", Width: 800, Height: 600, SkipTaskbar: true,
		}}},
	}, nil)
	args := strings.Join(launch.buildArgs("about:blank"), "\n")
	if !strings.Contains(args, "--xgoef-skip-taskbar") {
		t.Fatalf("args %s", args)
	}
	app := New(conf.File{
		Plugins: []string{"taskbar"},
		Capabilities: []acl.Capability{{
			Identifier: "main",
			Windows:    []string{"main"},
			Permissions: []string{
				"taskbar:skip",
				"taskbar:progress",
			},
		}},
	}, nil)
	if err := app.OpenPlugins(); err != nil {
		t.Fatal(err)
	}
	if err := app.Plugins.Setup(app); err != nil {
		t.Fatal(err)
	}
	if err := app.CreateWindow(cef.WindowSpec{
		Label: "aux", Title: "t", Width: 100, Height: 80, SkipTaskbar: true,
	}); err != nil {
		t.Fatal(err)
	}
	skipped := handleMessage(app.ACL, app.Router, nil, "main",
		`{"type":"invoke","id":1,"cmd":"taskbar:skip","args":{"skip":false}}`)
	if !strings.Contains(skipped, `"ok":true`) {
		t.Fatalf("%s", skipped)
	}
	progress := handleMessage(app.ACL, app.Router, nil, "main",
		`{"type":"invoke","id":2,"cmd":"taskbar:progress","args":{"progress":40}}`)
	if !strings.Contains(progress, `"ok":true`) {
		t.Fatalf("%s", progress)
	}
	cleared := handleMessage(app.ACL, app.Router, nil, "main",
		`{"type":"invoke","id":3,"cmd":"taskbar:progress","args":{"progress":null}}`)
	if !strings.Contains(cleared, `"ok":true`) {
		t.Fatalf("%s", cleared)
	}
	denied := handleMessage(app.ACL, app.Router, nil, "aux",
		`{"type":"invoke","id":4,"cmd":"taskbar:skip","args":{"skip":true}}`)
	if !strings.Contains(denied, `"code":"acl.denied"`) {
		t.Fatalf("%s", denied)
	}
	bridge := app.Bridge.(*cef.NativeBridge)
	got := bridge.TaskbarLog()
	want := []string{
		"expect\taux\t1",
		"skip\tmain\t0",
		"progress\tmain\t40",
		"progress\tmain\t-1",
	}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Fatalf("%q", got)
	}

	shown := New(conf.File{App: conf.AppSection{Windows: []conf.Window{{
		Label: "main", Title: "t", Width: 800, Height: 600,
	}}}}, nil)
	plain := strings.Join(shown.buildArgs("about:blank"), "\n")
	if strings.Contains(plain, "--xgoef-skip-taskbar") {
		t.Fatalf("plain %s", plain)
	}
}
