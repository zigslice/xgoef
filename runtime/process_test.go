package runtime

import (
	"os"
	"strings"
	"testing"

	"github.com/zigslice/xgoef/acl"
	"github.com/zigslice/xgoef/conf"
	"github.com/zigslice/xgoef/plugins/process"
)

func TestProcessRelaunchReleasesInstance(t *testing.T) {
	on := true
	app := New(conf.File{
		Identifier: "com.example.process",
		App:        conf.AppSection{SingleInstance: &on},
		Plugins:    []string{"process"},
		Capabilities: []acl.Capability{{
			Identifier:  "main",
			Windows:     []string{"main"},
			Permissions: []string{"process:exit", "process:relaunch"},
		}},
	}, nil)
	app.SetInstanceDir(t.TempDir())
	app.SetArgs([]string{"xgoef-hello", "--url=https://example.test", "--type=renderer", "keep"})
	if err := app.acquireInstance(); err != nil {
		t.Fatal(err)
	}
	sock := app.inst.Path()
	if _, err := os.Stat(sock); err != nil {
		t.Fatal(err)
	}
	if err := app.OpenPlugins(); err != nil {
		t.Fatal(err)
	}
	var proc *process.Plugin
	for _, p := range app.Plugins.Plugins() {
		if got, ok := p.(*process.Plugin); ok {
			proc = got
		}
	}
	if proc == nil {
		t.Fatal("process plugin was not loaded")
	}
	var argv []string
	var code int
	exited := false
	proc.SetStart(func(_ string, got []string) error {
		argv = append([]string(nil), got...)
		return nil
	})
	proc.SetExit(func(c int) {
		exited = true
		code = c
	})
	if err := app.Plugins.Setup(app); err != nil {
		t.Fatal(err)
	}
	out := handleMessage(app.ACL, app.Router, nil, "main",
		`{"type":"invoke","id":1,"cmd":"process:relaunch","args":{}}`)
	if !strings.Contains(out, `"ok":true`) || !exited || code != 0 {
		t.Fatalf("%s exited %v code %d", out, exited, code)
	}
	if _, err := os.Stat(sock); !os.IsNotExist(err) {
		t.Fatalf("instance lock still present: %v", err)
	}
	if len(argv) != 3 || argv[1] != "--url=https://example.test" || argv[2] != "keep" {
		t.Fatalf("argv %q", argv)
	}
	if strings.Contains(strings.Join(argv, "\n"), "--type") {
		t.Fatalf("cef switch kept: %q", argv)
	}
	denied := handleMessage(app.ACL, app.Router, nil, "aux",
		`{"type":"invoke","id":2,"cmd":"process:exit","args":{"code":1}}`)
	if !strings.Contains(denied, `"code":"acl.denied"`) {
		t.Fatalf("%s", denied)
	}
}
