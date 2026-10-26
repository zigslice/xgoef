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
	app := New(conf.File{Build: conf.Build{
		FrontendDist: "web",
		DevURL:       "http://127.0.0.1:5173",
	}}, nil)
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
	if _, err = app.NavigationURL("https://example.com/"); err == nil || !strings.Contains(err.Error(), "not allowed") {
		t.Fatalf("remote: %v", err)
	}
	if _, err = app.NavigationURL("file://" + outside); err == nil || !strings.Contains(err.Error(), "outside") {
		t.Fatalf("outside: %v", err)
	}
	args := app.buildArgs("xgoef://app/index.html")
	joined := strings.Join(args, "\n")
	if !strings.Contains(joined, "--url=xgoef://app/index.html") || !strings.Contains(joined, "--xgoef-asset-root=") {
		t.Fatalf("args %v", args)
	}
	if !hasExactArg(args, "--xgoef-remote=http://127.0.0.1:5173") {
		t.Fatalf("remote args %v", args)
	}
}

func TestRemoteOriginAllowlist(t *testing.T) {
	dir := t.TempDir()
	web := filepath.Join(dir, "web")
	if err := os.MkdirAll(web, 0o755); err != nil {
		t.Fatal(err)
	}
	open := New(conf.File{Build: conf.Build{FrontendDist: "web"}}, nil)
	open.SetBase(dir)
	if _, err := open.NavigationURL("https://example.com/path"); err == nil || !strings.Contains(err.Error(), "not allowed") {
		t.Fatalf("open remote: %v", err)
	}
	if joined := strings.Join(open.buildArgs("about:blank"), "\n"); strings.Contains(joined, "--xgoef-remote=") {
		t.Fatalf("empty list %s", joined)
	}

	app := New(conf.File{
		Build: conf.Build{FrontendDist: "web", DevURL: "http://127.0.0.1:5173/"},
		App: conf.AppSection{Remote: conf.RemoteConfig{URLs: []string{
			"https://example.com",
		}}},
	}, nil)
	app.SetBase(dir)
	for _, raw := range []string{
		"https://example.com/app?x=1",
		"https://Example.COM:443/index.html",
		"http://127.0.0.1:5173/src/main.tsx",
		"about:blank",
	} {
		if _, err := app.NavigationURL(raw); err != nil {
			t.Fatalf("%s %v", raw, err)
		}
	}
	for _, raw := range []string{
		"http://example.com/",
		"http://127.0.0.1/",
		"https://example.com:8443/",
	} {
		if _, err := app.NavigationURL(raw); err == nil || !strings.Contains(err.Error(), "not allowed") {
			t.Fatalf("%s %v", raw, err)
		}
	}
	want := "--xgoef-remote=https://example.com,http://127.0.0.1:5173"
	if !hasExactArg(app.buildArgs("about:blank"), want) {
		t.Fatalf("%v", app.buildArgs("about:blank"))
	}
}

func hasExactArg(args []string, want string) bool {
	for _, arg := range args {
		if arg == want {
			return true
		}
	}
	return false
}
