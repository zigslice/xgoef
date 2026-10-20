package cli

import (
	"crypto/ed25519"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/zigslice/xgoef/asset"
	"github.com/zigslice/xgoef/conf"
)

func TestLinkHostLayout(t *testing.T) {
	module := t.TempDir()
	app := filepath.Join(module, "examples", "hello")
	web := filepath.Join(app, "web")
	if err := os.MkdirAll(filepath.Join(module, "cef", "shim"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(web, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(module, "api", "js"), 0o755); err != nil {
		t.Fatal(err)
	}
	cef := t.TempDir()
	if err := os.MkdirAll(filepath.Join(cef, "include"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(cef, "build", "libcef_dll_wrapper"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cef, "build", "libcef_dll_wrapper", "libcef_dll_wrapper.a"), []byte("wrap"), 0o644); err != nil {
		t.Fatal(err)
	}
	llgo := t.TempDir()
	if err := os.MkdirAll(filepath.Join(llgo, "runtime"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(module, "go.mod"), []byte("module github.com/zigslice/xgoef\n\ngo 1.27\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(app, "main.xgo"), []byte("echo 1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	confBody := `{
	  "productName": "xgoef-hello",
	  "app": {"windows": [{"title": "t", "width": 1, "height": 1}]},
	  "build": {"frontendDist": "web"}
	}`
	if err := os.WriteFile(filepath.Join(app, "xgoef.conf.json"), []byte(confBody), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(web, "index.html"), []byte(`<script src="../../../api/js/xgoef.js"></script>`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(module, "api", "js", "xgoef.js"), []byte("/*bridge*/"), 0o644); err != nil {
		t.Fatal(err)
	}

	t.Setenv("CEF_ROOT", cef)
	t.Setenv("LLGO_ROOT", llgo)
	t.Setenv("XGOEF_JS", filepath.Join(module, "api", "js", "xgoef.js"))

	oldRun, oldLook, oldClear := runCommand, lookBin, clearLLGoCache
	t.Cleanup(func() {
		runCommand = oldRun
		lookBin = oldLook
		clearLLGoCache = oldClear
	})
	lookBin = func(name string) (string, error) { return name, nil }
	clearLLGoCache = func(string) {}
	var shimB string
	var log []string
	runCommand = func(name string, args []string, dir string, extra []string) error {
		log = append(log, name+" "+strings.Join(args, " "))
		switch name {
		case "cmake":
			for i, a := range args {
				if a == "-B" && i+1 < len(args) {
					shimB = args[i+1]
				}
			}
			if len(args) > 0 && args[0] == "--build" {
				if err := os.WriteFile(filepath.Join(shimB, "libxgoef_cef.a"), []byte("lib"), 0o644); err != nil {
					return err
				}
				if err := os.WriteFile(filepath.Join(shimB, "libcef.so"), []byte("so"), 0o644); err != nil {
					return err
				}
				helper := filepath.Join(shimB, "xgoef-helper")
				if err := os.WriteFile(helper, []byte("#!/bin/sh\n"), 0o755); err != nil {
					return err
				}
			}
		case "xgo":
			for i, a := range args {
				if a == "-o" && i+1 < len(args) {
					return os.WriteFile(args[i+1], []byte("bin"), 0o755)
				}
			}
		}
		joined := strings.Join(extra, "\n")
		if name == "xgo" || name == "pkg-config" || name == "go" {
			if !strings.Contains(joined, "CGO_ENABLED=0") || !strings.Contains(joined, "PKG_CONFIG_PATH=") {
				t.Errorf("%s env missing toolchain flags: %s", name, joined)
			}
		}
		return nil
	}

	out := filepath.Join(module, "build", "hello")
	if code := Run([]string{"build", "--app", app, "--out", out}); code != 0 {
		t.Fatalf("build exit %d\n%s", code, strings.Join(log, "\n"))
	}
	for _, name := range []string{"xgoef-hello", "xgoef-helper", "libxgoef_cef.a", "libcef_dll_wrapper.a", "libcef.so", "web/index.html", "web/xgoef.js"} {
		if _, err := os.Stat(filepath.Join(out, name)); err != nil {
			t.Fatalf("missing %s: %v", name, err)
		}
	}
	html, err := os.ReadFile(filepath.Join(out, "web", "index.html"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(html), `src="xgoef.js"`) {
		t.Fatalf("script src: %s", html)
	}
	pc, err := os.ReadFile(filepath.Join(module, "build", "pkgconfig", "xgoef-cef.pc"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(pc), "-lxgoef_cef") || strings.Contains(string(pc), "$ORIGIN") {
		t.Fatalf("pc file:\n%s", pc)
	}
	if !strings.Contains(strings.Join(log, "\n"), "xgo build -o") {
		t.Fatalf("commands:\n%s", strings.Join(log, "\n"))
	}
	pub := sealedPublicKey(t, filepath.Join(out, "xgoef.conf.json"))
	payload, err := asset.ReadSealedPayload(filepath.Join(out, "xgoef-hello"), pub)
	if err != nil {
		t.Fatal(err)
	}
	sibling, err := asset.ReadSealedPayload(filepath.Join(out, "xgoef.assets"), pub)
	if err != nil || string(sibling) != string(payload) {
		t.Fatalf("standalone seal %v", err)
	}
	src, err := os.ReadFile(filepath.Join(app, "xgoef.conf.json"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(src), "assetPublicKey") {
		t.Fatal("source conf gained assetPublicKey")
	}

	devOut := filepath.Join(module, "build", "devapp")
	if code := Run([]string{"dev", "--app", app, "--out", devOut}); code != 0 {
		t.Fatalf("dev exit %d", code)
	}
	if _, err := os.Stat(filepath.Join(devOut, "xgoef.assets")); !os.IsNotExist(err) {
		t.Fatalf("dev wrote a seal: %v", err)
	}
}

func sealedPublicKey(t *testing.T, confPath string) ed25519.PublicKey {
	t.Helper()
	cfg, err := conf.Load(confPath)
	if err != nil {
		t.Fatal(err)
	}
	pub, err := asset.ParsePublicKey(cfg.Build.AssetPublicKey)
	if err != nil {
		t.Fatal(err)
	}
	return pub
}

func TestLinkMissingToolchain(t *testing.T) {
	module := t.TempDir()
	if err := os.MkdirAll(filepath.Join(module, "cef", "shim"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(module, "go.mod"), []byte("module m\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	app := filepath.Join(module, "app")
	if err := os.MkdirAll(app, 0o755); err != nil {
		t.Fatal(err)
	}
	cef := t.TempDir()
	if err := os.MkdirAll(filepath.Join(cef, "include"), 0o755); err != nil {
		t.Fatal(err)
	}
	llgo := t.TempDir()
	if err := os.MkdirAll(filepath.Join(llgo, "runtime"), 0o755); err != nil {
		t.Fatal(err)
	}
	oldLook := lookBin
	t.Cleanup(func() { lookBin = oldLook })
	lookBin = func(name string) (string, error) {
		return "", os.ErrNotExist
	}
	t.Setenv("CEF_ROOT", cef)
	t.Setenv("LLGO_ROOT", llgo)
	if _, err := linkHost(app, filepath.Join(module, "out")); err == nil || !strings.Contains(err.Error(), "xgo not found") {
		t.Fatalf("got %v", err)
	}
}

func TestClearLLGoCache(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	module := t.TempDir()
	if err := os.WriteFile(filepath.Join(module, "go.mod"), []byte("module github.com/zigslice/xgoef\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	stale := filepath.Join(home, ".cache", "llgo", "build", "abc", "github.com", "zigslice", "xgoef", "cef", "pkg")
	keep := filepath.Join(home, ".cache", "llgo", "build", "abc", "other.com", "pkg", "cef")
	if err := os.MkdirAll(stale, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(keep, 0o755); err != nil {
		t.Fatal(err)
	}
	clearLLGoCache(module)
	if _, err := os.Stat(filepath.Dir(stale)); !os.IsNotExist(err) {
		t.Fatalf("stale cache still present: %v", err)
	}
	if _, err := os.Stat(keep); err != nil {
		t.Fatalf("unrelated cache removed: %v", err)
	}
}

func TestBuildScriptDelegates(t *testing.T) {
	b, err := os.ReadFile(filepath.Join("..", "scripts", "build-xgoef-hello.sh"))
	if err != nil {
		t.Fatal(err)
	}
	body := string(b)
	if !strings.Contains(body, "exec go run ./cmd/xgoef build") {
		t.Fatalf("script no longer delegates to xgoef build:\n%s", body)
	}
	if strings.Contains(body, "cmake -S") || strings.Contains(body, "xgo build") {
		t.Fatal("link steps belong in xgoef build")
	}
}
