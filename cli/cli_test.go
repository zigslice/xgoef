package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/zigslice/xgoef/conf"
)

func TestCreateAndBuild(t *testing.T) {
	root := t.TempDir()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(wd) })
	if err := os.Chdir(root); err != nil {
		t.Fatal(err)
	}
	js := filepath.Join(wd, "..", "api", "js", "xgoef.js")
	t.Setenv("XGOEF_JS", js)

	if code := Run([]string{"create", "demo"}); code != 0 {
		t.Fatalf("create exit %d", code)
	}
	if _, err := os.Stat(filepath.Join(root, "demo", "main.xgo")); err != nil {
		t.Fatal(err)
	}
	body, err := os.ReadFile(filepath.Join(root, "demo", "web", "index.html"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(body), "demo") || strings.Contains(string(body), "__NAME__") {
		t.Fatalf("template not substituted: %s", body)
	}
	if code := Run([]string{"create", "demo"}); code == 0 {
		t.Fatal("second create should fail")
	}
	if code := Run([]string{"create", "../evil"}); code == 0 {
		t.Fatal("bad name should fail")
	}

	out := filepath.Join(root, "out")
	if code := Run([]string{"build", "--assets-only", "--app", filepath.Join(root, "demo"), "--out", out}); code != 0 {
		t.Fatalf("build exit %d", code)
	}
	cfg, err := conf.Load(filepath.Join(out, "xgoef.conf.json"))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.ProductName != "demo" {
		t.Fatalf("product %q", cfg.ProductName)
	}
	if _, err := os.Stat(filepath.Join(out, "api", "js", "xgoef.js")); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(out, "web", "note.txt")); err != nil {
		t.Fatal(err)
	}

	devOut := filepath.Join(root, "devout")
	confPath := filepath.Join(root, "demo", "xgoef.conf.json")
	raw, err := os.ReadFile(confPath)
	if err != nil {
		t.Fatal(err)
	}
	patched := strings.Replace(string(raw), `"devUrl": ""`, `"devUrl": "http://127.0.0.1:5173"`, 1)
	if err := os.WriteFile(confPath, []byte(patched), 0o644); err != nil {
		t.Fatal(err)
	}
	if code := Run([]string{"dev", "--assets-only", "--app", filepath.Join(root, "demo"), "--out", devOut}); code != 0 {
		t.Fatalf("dev exit %d", code)
	}
	devCfg, err := conf.Load(filepath.Join(devOut, "xgoef.conf.json"))
	if err != nil {
		t.Fatal(err)
	}
	if devCfg.FirstWindow().URL != "http://127.0.0.1:5173" {
		t.Fatalf("url %q", devCfg.FirstWindow().URL)
	}
}

func TestBuildRewritesHelloScript(t *testing.T) {
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	repo := filepath.Join(wd, "..")
	out := t.TempDir()
	if code := Run([]string{"build", "--assets-only", "--app", filepath.Join(repo, "examples", "hello"), "--out", out}); code != 0 {
		t.Fatalf("build exit %d", code)
	}
	html, err := os.ReadFile(filepath.Join(out, "web", "index.html"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(html), `src="xgoef.js"`) {
		t.Fatalf("script src not rewritten:\n%s", html)
	}
	if strings.Contains(string(html), "../../../api/js/xgoef.js") {
		t.Fatal("old script src remains")
	}
	if _, err := os.Stat(filepath.Join(out, "web", "xgoef.js")); err != nil {
		t.Fatal(err)
	}
}

func TestStageWritesDesktopFile(t *testing.T) {
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("XGOEF_JS", filepath.Join(wd, "..", "api", "js", "xgoef.js"))
	app := filepath.Join(t.TempDir(), "demo")
	if err := os.MkdirAll(filepath.Join(app, "web"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(app, "web", "index.html"), []byte("ok"), 0o644); err != nil {
		t.Fatal(err)
	}
	raw := []byte(`{
  "productName": "demo",
  "identifier": "com.example.demo",
  "app": {
    "windows": [{"title": "demo", "width": 800, "height": 600}],
    "deepLink": {"schemes": ["demo"]}
  }
}
`)
	if err := os.WriteFile(filepath.Join(app, "xgoef.conf.json"), raw, 0o644); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(t.TempDir(), "dist")
	if code := Run([]string{"build", "--assets-only", "--app", app, "--out", out}); code != 0 {
		t.Fatalf("build exit %d", code)
	}
	body, err := os.ReadFile(filepath.Join(out, "xgoef-com.example.demo.desktop"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(body)
	if !strings.Contains(text, "x-scheme-handler/demo;") || !strings.Contains(text, "%u") || !strings.Contains(text, filepath.Join(out, "demo")) {
		t.Fatalf("%s", text)
	}
	raw = []byte(strings.Replace(string(raw), `"identifier"`, `"version": "0.0.1",
  "identifier"`, 1))
	if err := os.WriteFile(filepath.Join(app, "xgoef.conf.json"), raw, 0o644); err != nil {
		t.Fatal(err)
	}
	bundled := filepath.Join(t.TempDir(), "dist")
	if code := Run([]string{"build", "--assets-only", "--app", app, "--out", bundled}); code != 0 {
		t.Fatalf("bundle build exit %d", code)
	}
	deb, err := filepath.Glob(filepath.Join(bundled, "bundle", "*.deb"))
	if err != nil || len(deb) != 1 {
		t.Fatalf("%v %v", deb, err)
	}
	msi, err := filepath.Glob(filepath.Join(bundled, "bundle", "*.msi"))
	if err != nil || len(msi) != 1 {
		t.Fatalf("%v %v", msi, err)
	}
	dmg, err := filepath.Glob(filepath.Join(bundled, "bundle", "*.dmg"))
	if err != nil || len(dmg) != 1 {
		t.Fatalf("%v %v", dmg, err)
	}
}
