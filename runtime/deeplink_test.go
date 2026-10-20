package runtime

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/zigslice/xgoef/acl"
	"github.com/zigslice/xgoef/conf"
)

func deepCfg() conf.File {
	return conf.File{
		ProductName: "hello",
		Identifier:  "com.example.hello",
		App: conf.AppSection{DeepLink: &conf.DeepLinkConfig{
			Schemes: []string{"hello"},
		}},
		Capabilities: []acl.Capability{{
			Identifier:  "main",
			Windows:     []string{"main"},
			Permissions: []string{"deeplink:get_current"},
		}},
	}
}

func TestDeepLinkStartupURL(t *testing.T) {
	apps := t.TempDir()
	cfgDir := t.TempDir()
	exe := filepath.Join(t.TempDir(), "xgoef-hello")
	app := New(deepCfg(), nil)
	app.SetDeepLinkDirs(apps, cfgDir)
	app.SetExecutable(exe)
	var got string
	app.Events.Listen("deep-link", func(payload any) {
		got = fmt.Sprint(payload)
	})
	if err := app.openDeepLink([]string{"bin", "hello://item/1", "https://example"}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got, "hello://item/1") || strings.Contains(got, "https://example") {
		t.Fatalf("%q", got)
	}
	out := handleMessage(app.ACL, app.Router, nil, "main",
		`{"type":"invoke","id":1,"cmd":"deeplink:get_current","args":{}}`)
	if !strings.Contains(out, "hello://item/1") || strings.Contains(out, "https://example") {
		t.Fatalf("%s", out)
	}
	if _, err := os.Stat(filepath.Join(apps, "xgoef-com.example.hello.desktop")); err != nil {
		t.Fatal(err)
	}
	mime, err := os.ReadFile(filepath.Join(cfgDir, "mimeapps.list"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(mime), "x-scheme-handler/hello=") {
		t.Fatalf("%s", mime)
	}
	denied := New(conf.File{
		Identifier: "com.example.hello",
		App:        conf.AppSection{DeepLink: &conf.DeepLinkConfig{Schemes: []string{"hello"}}},
	}, nil)
	denied.SetDeepLinkDirs(t.TempDir(), t.TempDir())
	denied.SetExecutable(exe)
	if err := denied.openDeepLink(nil); err != nil {
		t.Fatal(err)
	}
	blocked := handleMessage(denied.ACL, denied.Router, nil, "main",
		`{"type":"invoke","id":1,"cmd":"deeplink:get_current","args":{}}`)
	if !strings.Contains(blocked, `"ok":false`) {
		t.Fatalf("%s", blocked)
	}
}

func TestDeepLinkFromSecondInstance(t *testing.T) {
	dir := t.TempDir()
	exe := filepath.Join(t.TempDir(), "xgoef-hello")
	first := New(deepCfg(), nil)
	first.SetInstanceDir(dir)
	first.SetDeepLinkDirs(t.TempDir(), t.TempDir())
	first.SetExecutable(exe)
	first.SetArgs([]string{"hello"})
	var got string
	first.Events.Listen("deep-link", func(payload any) {
		got = fmt.Sprint(payload)
	})
	if err := first.acquireInstance(); err != nil {
		t.Fatal(err)
	}
	defer first.closeInstance()
	if err := first.openDeepLink(first.programArgs()); err != nil {
		t.Fatal(err)
	}

	second := New(deepCfg(), nil)
	second.SetInstanceDir(dir)
	second.SetArgs([]string{"hello", "hello://again"})
	err := second.acquireInstance()
	ee, ok := err.(*ExitError)
	if !ok || ee.Code != 0 {
		t.Fatalf("%v", err)
	}
	if !strings.Contains(got, "hello://again") {
		t.Fatalf("%q", got)
	}
	out := handleMessage(first.ACL, first.Router, nil, "main",
		`{"type":"invoke","id":1,"cmd":"deeplink:get_current","args":{}}`)
	if !strings.Contains(out, "hello://again") {
		t.Fatalf("%s", out)
	}
}

func TestDeepLinkOffWithoutSchemes(t *testing.T) {
	app := New(conf.File{Identifier: "com.example.hello"}, nil)
	if err := app.openDeepLink([]string{"hello://item"}); err != nil {
		t.Fatal(err)
	}
	if _, err := app.Router.Invoke("deeplink:get_current", []byte("{}")); err == nil {
		t.Fatal("command registered without schemes")
	}
}

func TestDeepLinkRequiresIdentifier(t *testing.T) {
	app := New(conf.File{App: conf.AppSection{DeepLink: &conf.DeepLinkConfig{Schemes: []string{"hello"}}}}, nil)
	if err := app.openDeepLink(nil); err == nil || !strings.Contains(err.Error(), "identifier") {
		t.Fatal(err)
	}
}
