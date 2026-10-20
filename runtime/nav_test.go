package runtime

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/zigslice/xgoef/conf"
)

func TestNavigationURLRewritesFilesInsideRoot(t *testing.T) {
	dir := t.TempDir()
	web := filepath.Join(dir, "web")
	if err := os.MkdirAll(web, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(web, "index.html"), []byte("hi"), 0o644); err != nil {
		t.Fatal(err)
	}
	outside := filepath.Join(dir, "secret.txt")
	if err := os.WriteFile(outside, []byte("no"), 0o644); err != nil {
		t.Fatal(err)
	}
	app := New(conf.File{Build: conf.Build{FrontendDist: "web"}}, nil)
	app.SetBase(dir)

	got, err := app.NavigationURL("xgoef://app/index.html")
	if err != nil || got != "xgoef://app/index.html" {
		t.Fatalf("%q %v", got, err)
	}
	got, err = app.NavigationURL("file://web/index.html")
	if err != nil || got != "xgoef://app/index.html" {
		t.Fatalf("rewrite %q %v", got, err)
	}
	got, err = app.NavigationURL("http://127.0.0.1:5173")
	if err != nil || got != "http://127.0.0.1:5173" {
		t.Fatalf("dev %q %v", got, err)
	}
	if _, err = app.NavigationURL("file://" + outside); err == nil || !strings.Contains(err.Error(), "outside") {
		t.Fatalf("outside: %v", err)
	}
	args := app.buildArgs("xgoef://app/index.html")
	joined := strings.Join(args, "\n")
	if !strings.Contains(joined, "--url=xgoef://app/index.html") || !strings.Contains(joined, "--xgoef-asset-root=") {
		t.Fatalf("args %v", args)
	}
}
