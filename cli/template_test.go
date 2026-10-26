package cli

import (
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/zigslice/xgoef/conf"
)

func TestCreateTemplates(t *testing.T) {
	root := t.TempDir()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(wd) })
	if err := os.Chdir(root); err != nil {
		t.Fatal(err)
	}
	t.Setenv("XGOEF_JS", filepath.Join(wd, "..", "api", "js", "xgoef.js"))

	if code := Run([]string{"create", "--template", "html", "plain"}); code != 0 {
		t.Fatalf("html exit %d", code)
	}
	html, err := os.ReadFile(filepath.Join(root, "plain", "web", "index.html"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(html), "plain") || strings.Contains(string(html), "__NAME__") {
		t.Fatalf("%s", html)
	}
	if _, err := os.Stat(filepath.Join(root, "plain", "package.json")); !os.IsNotExist(err) {
		t.Fatal(err)
	}

	if code := Run([]string{"create", "ViteApp", "--template", "vite"}); code != 0 {
		t.Fatalf("vite exit %d", code)
	}
	vite := filepath.Join(root, "ViteApp")
	assertHas(t, vite, "package.json", "vite.config.js", "index.html", "src/main.js", ".gitignore", "web/note.txt", "public/note.txt")
	pkg, err := os.ReadFile(filepath.Join(vite, "package.json"))
	if err != nil || !strings.Contains(string(pkg), `"name": "viteapp"`) || strings.Contains(string(pkg), "__PKG__") {
		t.Fatalf("%s %v", pkg, err)
	}
	cfg, err := conf.Load(filepath.Join(vite, "xgoef.conf.json"))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Build.DevURL != "http://127.0.0.1:5173" || cfg.Build.BeforeDevCommand != "npm run dev" || cfg.Build.BeforeBuildCommand != "npm run build" {
		t.Fatalf("%+v", cfg.Build)
	}
	page, err := os.ReadFile(filepath.Join(vite, "src", "main.js"))
	if err != nil || !strings.Contains(string(page), `invoke("greet"`) || strings.Contains(string(page), "__NAME__") {
		t.Fatalf("%s %v", page, err)
	}
	vconf, err := os.ReadFile(filepath.Join(vite, "vite.config.js"))
	if err != nil || !strings.Contains(string(vconf), `base: "./"`) || !strings.Contains(string(vconf), `outDir: "web"`) || !strings.Contains(string(vconf), "5173") {
		t.Fatalf("%s %v", vconf, err)
	}

	if code := Run([]string{"create", "--template", "react", "ui"}); code != 0 {
		t.Fatalf("react exit %d", code)
	}
	react := filepath.Join(root, "ui")
	assertHas(t, react, "src/App.jsx", "src/main.jsx", "vite.config.js")
	app, err := os.ReadFile(filepath.Join(react, "src", "App.jsx"))
	if err != nil || !strings.Contains(string(app), "__XGOEF__") || !strings.Contains(string(app), "ui") || strings.Contains(string(app), "__NAME__") {
		t.Fatalf("%s %v", app, err)
	}
	rconf, err := os.ReadFile(filepath.Join(react, "vite.config.js"))
	if err != nil || !strings.Contains(string(rconf), "@vitejs/plugin-react") || !strings.Contains(string(rconf), "react()") {
		t.Fatalf("%s %v", rconf, err)
	}

	if code := Run([]string{"create", "nope", "--template", "vue"}); code == 0 {
		t.Fatal("unknown template")
	}
	if _, err := os.Stat(filepath.Join(root, "nope")); !os.IsNotExist(err) {
		t.Fatal(err)
	}
	if code := Run([]string{"create", "nope", "--template"}); code == 0 {
		t.Fatal("missing template value")
	}
	if _, err := os.Stat(filepath.Join(root, "nope")); !os.IsNotExist(err) {
		t.Fatal(err)
	}
}

func TestBeforeBuildCommand(t *testing.T) {
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("XGOEF_JS", filepath.Join(wd, "..", "api", "js", "xgoef.js"))
	app := filepath.Join(t.TempDir(), "demo")
	if err := os.MkdirAll(app, 0o755); err != nil {
		t.Fatal(err)
	}
	script := filepath.Join(app, "prebuild.sh")
	body := "#!/bin/sh\nmkdir -p web\nprintf built > web/built.txt\nprintf '<h1>page</h1>' > web/index.html\n"
	if err := os.WriteFile(script, []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
	raw := []byte(`{
  "productName": "demo",
  "build": {"frontendDist": "web", "beforeBuildCommand": "./prebuild.sh"}
}
`)
	if err := os.WriteFile(filepath.Join(app, "xgoef.conf.json"), raw, 0o644); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(t.TempDir(), "dist")
	if code := Run([]string{"build", "--assets-only", "--app", app, "--out", out}); code != 0 {
		t.Fatalf("build exit %d", code)
	}
	got, err := os.ReadFile(filepath.Join(out, "web", "built.txt"))
	if err != nil || string(got) != "built" {
		t.Fatalf("%q %v", got, err)
	}

	bad := filepath.Join(t.TempDir(), "bad")
	if err := os.MkdirAll(bad, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(bad, "xgoef.conf.json"), []byte(`{"productName":"bad","build":{"beforeBuildCommand":"exit 9"}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if code := Run([]string{"build", "--assets-only", "--app", bad, "--out", filepath.Join(t.TempDir(), "badout")}); code == 0 {
		t.Fatal("failing beforeBuildCommand should fail the build")
	}
}

func TestDevSkipsBeforeBuild(t *testing.T) {
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("XGOEF_JS", filepath.Join(wd, "..", "api", "js", "xgoef.js"))
	app := filepath.Join(t.TempDir(), "demo")
	if err := os.MkdirAll(filepath.Join(app, "web"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(app, "web", "index.html"), []byte("page"), 0o644); err != nil {
		t.Fatal(err)
	}
	marker := filepath.Join(app, "ran")
	raw := `{
  "productName": "demo",
  "build": {"frontendDist": "web", "beforeBuildCommand": "touch ` + marker + `"},
  "app": {"windows": [{"title": "demo", "url": "xgoef://app/index.html", "width": 800, "height": 600}]}
}
`
	if err := os.WriteFile(filepath.Join(app, "xgoef.conf.json"), []byte(raw), 0o644); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(t.TempDir(), "dist")
	if err := os.MkdirAll(out, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(out, "demo"), []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	if code := Run([]string{"dev", "--assets-only", "--app", app, "--out", out}); code != 0 {
		t.Fatalf("dev exit %d", code)
	}
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Fatal(err)
	}
}

func TestWaitDevURL(t *testing.T) {
	prev := devReadyTimeout
	t.Cleanup(func() { devReadyTimeout = prev })

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	proc, err := startShell(t.TempDir(), "sleep 30")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		proc.stop()
		_ = proc.wait()
	})
	url := "http://" + ln.Addr().String()
	if err := waitDevURL(url, proc); err != nil {
		t.Fatal(err)
	}

	gone, err := startShell(t.TempDir(), "exit 7")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		gone.stop()
		_ = gone.wait()
	})
	devReadyTimeout = time.Second
	if err := waitDevURL("http://127.0.0.1:1", gone); err == nil || !strings.Contains(err.Error(), "exited") {
		t.Fatal(err)
	}

	hung, err := startShell(t.TempDir(), "sleep 30")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		hung.stop()
		_ = hung.wait()
	})
	devReadyTimeout = 200 * time.Millisecond
	if err := waitDevURL("http://127.0.0.1:1", hung); err == nil || !strings.Contains(err.Error(), "did not listen") {
		t.Fatal(err)
	}
	if _, err := startBeforeDev(t.TempDir(), conf.File{Build: conf.Build{BeforeDevCommand: "sleep 30", DevURL: "nope"}}); err == nil || !strings.Contains(err.Error(), "http") {
		t.Fatal(err)
	}
}

func assertHas(t *testing.T, root string, names ...string) {
	t.Helper()
	for _, name := range names {
		if _, err := os.Stat(filepath.Join(root, name)); err != nil {
			t.Fatal(err)
		}
	}
}
