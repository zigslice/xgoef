package cli

import (
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestMain(m *testing.M) {
	if os.Getenv("XGOEF_DEV_HELPER") == "1" {
		runDevHelper()
		os.Exit(0)
	}
	os.Exit(m.Run())
}

func runDevHelper() {
	time.AfterFunc(5*time.Second, func() { os.Exit(1) })
	sock := os.Getenv("XGOEF_DEV_SOCK")
	var conn net.Conn
	var err error
	deadline := time.Now().Add(2 * time.Second)
	for {
		conn, err = net.Dial("unix", sock)
		if err == nil {
			break
		}
		if time.Now().After(deadline) {
			os.Exit(1)
		}
		time.Sleep(20 * time.Millisecond)
	}
	defer conn.Close()
	ready := os.Getenv("XGOEF_DEV_READY")
	if ready != "" {
		_ = os.WriteFile(ready, []byte("up"), 0o644)
	}
	buf := make([]byte, 64)
	n, err := conn.Read(buf)
	if err != nil || !strings.Contains(string(buf[:n]), "reload") {
		os.Exit(1)
	}
	if ready != "" {
		_ = os.WriteFile(ready, []byte("reloaded"), 0o644)
	}
}

func TestDevReloadsAndLaunches(t *testing.T) {
	devPoll = 30 * time.Millisecond
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("XGOEF_JS", filepath.Join(wd, "..", "api", "js", "xgoef.js"))
	app := filepath.Join(t.TempDir(), "demo")
	if err := os.MkdirAll(filepath.Join(app, "web"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(app, "web", "index.html"), []byte("v1"), 0o644); err != nil {
		t.Fatal(err)
	}
	raw := []byte(`{
  "productName": "demo",
  "build": {"frontendDist": "web"},
  "app": {"windows": [{"title": "demo", "url": "xgoef://app/index.html", "width": 800, "height": 600}]}
}
`)
	if err := os.WriteFile(filepath.Join(app, "xgoef.conf.json"), raw, 0o644); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(t.TempDir(), "dist")
	if err := os.MkdirAll(out, 0o755); err != nil {
		t.Fatal(err)
	}
	script := "#!/bin/sh\nexport XGOEF_DEV_HELPER=1\nexec " + shellQuote(os.Args[0]) + "\n"
	if err := os.WriteFile(filepath.Join(out, "demo"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	ready := filepath.Join(t.TempDir(), "ready")
	t.Setenv("XGOEF_DEV_READY", ready)
	go func() {
		deadline := time.Now().Add(3 * time.Second)
		for time.Now().Before(deadline) {
			body, err := os.ReadFile(ready)
			if err == nil && string(body) == "up" {
				_ = os.WriteFile(filepath.Join(app, "web", "index.html"), []byte("v2-page"), 0o644)
				return
			}
			time.Sleep(20 * time.Millisecond)
		}
	}()
	if code := Run([]string{"dev", "--assets-only", "--app", app, "--out", out}); code != 0 {
		t.Fatalf("dev exit %d", code)
	}
	body, err := os.ReadFile(filepath.Join(out, "web", "index.html"))
	if err != nil {
		t.Fatal(err)
	}
	if string(body) != "v2-page" {
		t.Fatalf("%q", body)
	}
	state, err := os.ReadFile(ready)
	if err != nil || string(state) != "reloaded" {
		t.Fatalf("%q %v", state, err)
	}
}

func shellQuote(path string) string {
	return "'" + strings.ReplaceAll(path, "'", `'\''`) + "'"
}
