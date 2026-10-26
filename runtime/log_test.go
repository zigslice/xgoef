package runtime

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/zigslice/xgoef/acl"
	"github.com/zigslice/xgoef/conf"
	"github.com/zigslice/xgoef/ipc"
	"github.com/zigslice/xgoef/log"
)

func TestLogWritePanicAndCrashSwitch(t *testing.T) {
	dir := t.TempDir()
	app := New(conf.File{
		ProductName: "Hello",
		Identifier:  "com.example.log",
		Plugins:     []string{"log"},
		Plugin:      conf.PluginSection{Log: conf.LogConfig{Level: "info"}},
		Capabilities: []acl.Capability{{
			Identifier:  "main",
			Windows:     []string{"main"},
			Permissions: []string{"log:write", "boom"},
		}},
	}, nil)
	state, err := log.UserDir()
	if err != nil {
		t.Fatal(err)
	}
	before := strings.Join(app.buildArgs("about:blank"), "\n")
	if !strings.Contains(before, "--xgoef-crash-dumps="+state) {
		t.Fatalf("%s", before)
	}
	plain := New(conf.File{ProductName: "Hello"}, nil)
	if strings.Contains(strings.Join(plain.buildArgs("about:blank"), "\n"), "xgoef-crash-dumps") {
		t.Fatal("crash switch without the log plugin")
	}
	if err := app.OpenPlugins(); err != nil {
		t.Fatal(err)
	}
	var plug interface {
		SetDir(string)
		SetStderr(io.Writer)
	}
	for _, p := range app.Plugins.Plugins() {
		if p.Name() != "log" {
			continue
		}
		got, ok := p.(interface {
			SetDir(string)
			SetStderr(io.Writer)
		})
		if !ok {
			t.Fatalf("%T", p)
		}
		plug = got
	}
	if plug == nil {
		t.Fatal("log plugin was not loaded")
	}
	plug.SetDir(dir)
	plug.SetStderr(nil)
	if got := app.crashDumpsDir(); got != dir {
		t.Fatalf("crash dir %s", got)
	}
	switched := strings.Join(app.buildArgs("about:blank"), "\n")
	if !strings.Contains(switched, "--xgoef-crash-dumps="+dir) || strings.Contains(switched, state) {
		t.Fatalf("%s", switched)
	}
	if err := app.Plugins.Setup(app); err != nil {
		t.Fatal(err)
	}
	denied := handleMessage(app.ACL, app.Router, nil, "main",
		`{"type":"invoke","id":1,"cmd":"log:write","args":{"level":"info","message":"x","path":"/tmp/evil"}}`)
	if !strings.Contains(denied, `"code":"log.denied"`) {
		t.Fatalf("%s", denied)
	}
	if entries, err := os.ReadDir(dir); err != nil || len(entries) != 0 {
		t.Fatalf("%v %v", entries, err)
	}
	wrote := handleMessage(app.ACL, app.Router, nil, "main",
		`{"type":"invoke","id":2,"cmd":"log:write","args":{"message":"hello"}}`)
	if !strings.Contains(wrote, `"ok":true`) {
		t.Fatalf("%s", wrote)
	}
	body, err := os.ReadFile(filepath.Join(dir, "xgoef-com.example.log.log"))
	if err != nil || !strings.Contains(string(body), " INFO hello\n") {
		t.Fatalf("%q %v", body, err)
	}
	other := handleMessage(app.ACL, app.Router, nil, "aux",
		`{"type":"invoke","id":3,"cmd":"log:write","args":{"message":"no"}}`)
	if !strings.Contains(other, `"code":"acl.denied"`) {
		t.Fatalf("%s", other)
	}
	app.Handle("boom", func(ipc.Context, []byte) (any, error) {
		panic("a\nb")
	})
	box := newReplyBox()
	dispatchFrontend(app.ACL, app.Router, nil, app.invokes, "main",
		`{"type":"invoke","id":4,"cmd":"boom","args":{"text":"from-page"}}`, 7, box.add)
	got := box.wait(t)
	if !strings.Contains(got, `"code":"ipc.panic"`) {
		t.Fatalf("%s", got)
	}
	crash, err := os.ReadFile(filepath.Join(dir, "xgoef-com.example.log.crash"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(crash)
	if !strings.Contains(text, " CRASH a b\n") || strings.Contains(text, "from-page") {
		t.Fatalf("%q", text)
	}
	body, err = os.ReadFile(filepath.Join(dir, "xgoef-com.example.log.log"))
	if err != nil || !strings.Contains(string(body), " CRASH a b\n") || strings.Contains(string(body), "from-page") {
		t.Fatalf("%q %v", body, err)
	}
}

func TestCrashDumpsDirRejectsRelativeOverride(t *testing.T) {
	app := New(conf.File{
		ProductName: "Hello",
		Identifier:  "app",
		Plugins:     []string{"log"},
	}, nil)
	if err := app.OpenPlugins(); err != nil {
		t.Fatal(err)
	}
	found := false
	for _, p := range app.Plugins.Plugins() {
		if p.Name() != "log" {
			continue
		}
		p.(interface{ SetDir(string) }).SetDir("relative")
		found = true
	}
	if !found {
		t.Fatal("missing log plugin")
	}
	if got := app.crashDumpsDir(); got != "" {
		t.Fatalf("relative dir %s", got)
	}
	args := strings.Join(app.buildArgs("about:blank"), "\n")
	if strings.Contains(args, "xgoef-crash-dumps") {
		t.Fatalf("%s", args)
	}
}
