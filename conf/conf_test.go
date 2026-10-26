package conf

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadHelloConfig(t *testing.T) {
	path := filepath.Join("..", "examples", "hello", "xgoef.conf.json")
	f, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if f.ProductName != "xgoef-hello" {
		t.Fatalf("productName=%q", f.ProductName)
	}
	w := f.FirstWindow()
	if w.Width != 960 || w.Height != 640 {
		t.Fatalf("window=%+v", w)
	}
	if w.Label != "main" {
		t.Fatalf("label=%q", w.Label)
	}
	if f.CEF.RootEnv != "CEF_ROOT" {
		t.Fatalf("rootEnv=%q", f.CEF.RootEnv)
	}
	if !f.Plugin.WebView.DevTools {
		t.Fatal("hello enables devtools")
	}
	if !f.App.InjectBridge() {
		t.Fatal("hello injects the bridge")
	}
	if len(f.App.Windows[0].InitScripts) != 1 || !strings.Contains(f.App.Windows[0].InitScripts[0], "__xgoefInjected") {
		t.Fatalf("init %#v", f.App.Windows[0].InitScripts)
	}
	if f.Plugin.Eval.Scripts["mark"] == "" || strings.Contains(f.Plugin.Eval.Scripts["mark"], "alert") {
		t.Fatalf("mark %#v", f.Plugin.Eval.Scripts)
	}
	foundAuto := false
	foundLog := false
	for _, name := range f.Plugins {
		if name == "autostart" {
			foundAuto = true
		}
		if name == "log" {
			foundLog = true
		}
	}
	if !foundAuto || !foundLog {
		t.Fatalf("plugins %v", f.Plugins)
	}
	for _, perm := range []string{"autostart:enable", "autostart:disable", "autostart:is_enabled", "log:write"} {
		ok := false
		for _, c := range f.Capabilities {
			for _, p := range c.Permissions {
				if p == perm {
					ok = true
				}
			}
		}
		if !ok {
			t.Fatalf("missing %s in %+v", perm, f.Capabilities)
		}
	}
	if len(f.Capabilities) == 0 {
		t.Fatal("expected capabilities")
	}
	found := false
	for _, c := range f.Capabilities {
		for _, p := range c.Permissions {
			if p == "greet" {
				found = true
			}
		}
	}
	if !found {
		t.Fatalf("capabilities=%+v", f.Capabilities)
	}
}

func TestLoadCapabilitiesArray(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "ok.json")
	body := `{
	  "productName":"t",
	  "app":{"windows":[{"label":"main","title":"t","width":1,"height":1}]},
	  "capabilities":[{"identifier":"main","windows":["main"],"permissions":["greet"]}]
	}`
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	f, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(f.Capabilities) != 1 || f.Capabilities[0].Permissions[0] != "greet" {
		t.Fatalf("%+v", f.Capabilities)
	}
}

func TestLoadRejectsCommandType(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bad.json")
	body := `{
	  "productName":"t",
	  "app":{"windows":[{"title":"t","width":1,"height":1}]},
	  "commands":[{"name":"greet","result":"nope"}]
	}`
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(path); err == nil || !strings.Contains(err.Error(), "unsupported type") {
		t.Fatal(err)
	}
}

func TestLoadMissingProductName(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bad.json")
	if err := os.WriteFile(path, []byte(`{"app":{}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(path); err == nil {
		t.Fatal("expected error")
	}
}

func TestLoadWindowChrome(t *testing.T) {
	dir := t.TempDir()
	okPath := filepath.Join(dir, "ok.json")
	body := `{
	  "productName":"t",
	  "app":{"windows":[
	    {"title":"plain","width":1,"height":1},
	    {"label":"aux","title":"aux","width":2,"height":2,"decorations":false,"theme":"Dark","transparent":true}
	  ]}
	}`
	if err := os.WriteFile(okPath, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	f, err := Load(okPath)
	if err != nil {
		t.Fatal(err)
	}
	plain, err := f.App.Windows[0].Chrome().Encode()
	if err != nil || plain != "1,system,0" {
		t.Fatalf("%q %v", plain, err)
	}
	aux, err := f.App.Windows[1].Chrome().Encode()
	if err != nil || aux != "0,dark,1" || f.App.Windows[1].Theme != "dark" {
		t.Fatalf("%q theme %q %v", aux, f.App.Windows[1].Theme, err)
	}

	badPath := filepath.Join(dir, "bad.json")
	bad := `{
	  "productName":"t",
	  "app":{"windows":[{"title":"t","width":1,"height":1,"theme":"blue"}]}
	}`
	if err := os.WriteFile(badPath, []byte(bad), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err = Load(badPath)
	if err == nil || err.Error() != `conf: window main theme "blue"` {
		t.Fatal(err)
	}
}

func TestLoadWithGlobalXgoef(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "ok.json")
	body := `{
	  "productName":"t",
	  "app":{"withGlobalXgoef": false, "windows":[{"title":"t","width":1,"height":1}]}
	}`
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	f, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if f.App.InjectBridge() || f.App.WithGlobalXgoef == nil || *f.App.WithGlobalXgoef {
		t.Fatalf("%v", f.App.WithGlobalXgoef)
	}
	on := true
	f.App.WithGlobalXgoef = &on
	if !f.App.InjectBridge() {
		t.Fatal("explicit true")
	}
	f.App.WithGlobalXgoef = nil
	if !f.App.InjectBridge() {
		t.Fatal("omitted")
	}
}

func TestLoadRejectsEvalScript(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bad.json")
	body := `{
	  "productName":"t",
	  "app":{"windows":[{"title":"t","width":1,"height":1,"initScripts":["   "]}]}
	}`
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(path); err == nil || !strings.Contains(err.Error(), "window main") {
		t.Fatal(err)
	}
	body = `{
	  "productName":"t",
	  "plugin":{"eval":{"scripts":{"Alert":"window.ok = 1;"}}}
	}`
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(path); err == nil || !strings.Contains(err.Error(), "name is invalid") {
		t.Fatal(err)
	}
}

func TestLoadRejectsAutostartArgs(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "ok.json")
	body := `{
	  "productName":"t",
	  "plugin":{"autostart":{"args":["--minimized","a b"]}}
	}`
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	f, err := Load(path)
	if err != nil || len(f.Plugin.Autostart.Args) != 2 || f.Plugin.Autostart.Args[0] != "--minimized" {
		t.Fatalf("%v %v", f.Plugin.Autostart.Args, err)
	}
	long := strings.Repeat("a", 129)
	for _, args := range []string{
		`["ok",""]`,
		`["   "]`,
		`["` + long + `"]`,
		`["line\n"]`,
		`["a","b","c","d","e","f","g","h","i"]`,
	} {
		body = `{
		  "productName":"t",
		  "plugin":{"autostart":{"args":` + args + `}}
		}`
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
		if _, err := Load(path); err == nil || !strings.Contains(err.Error(), "autostart:") {
			t.Fatalf("%s %v", args, err)
		}
	}
}

func TestLoadRejectsLogLevel(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "ok.json")
	body := `{
	  "productName":"t",
	  "plugin":{"log":{"level":"debug"}}
	}`
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	f, err := Load(path)
	if err != nil || f.Plugin.Log.Level != "debug" || len(f.Plugins) != 0 {
		t.Fatalf("%q %v %v", f.Plugin.Log.Level, f.Plugins, err)
	}
	body = `{
	  "productName":"t",
	  "plugin":{"log":{"level":"verbose"}}
	}`
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(path); err == nil || !strings.Contains(err.Error(), "level is invalid") {
		t.Fatal(err)
	}
}

func TestLoadRejectsShellCommand(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bad.json")
	body := `{
	  "productName":"t",
	  "build":{"beforeDevCommand":"npm run dev\nnpm test"}
	}`
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(path); err == nil || !strings.Contains(err.Error(), "beforeDevCommand") {
		t.Fatal(err)
	}
	body = `{
	  "productName":"t",
	  "build":{"beforeBuildCommand":"` + strings.Repeat("a", 1025) + `"}
	}`
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(path); err == nil || !strings.Contains(err.Error(), "beforeBuildCommand") {
		t.Fatal(err)
	}
}
