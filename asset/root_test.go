package asset

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestResolveInsideRoot(t *testing.T) {
	dir := t.TempDir()
	web := filepath.Join(dir, "web")
	if err := os.MkdirAll(filepath.Join(web, "css"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(web, "index.html"), []byte("hi"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(web, "css", "app.css"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(web, "my file.txt"), []byte("s"), 0o644); err != nil {
		t.Fatal(err)
	}
	root, err := Open(web)
	if err != nil {
		t.Fatal(err)
	}
	got, mime, err := root.Resolve("xgoef://app/index.html")
	if err != nil {
		t.Fatal(err)
	}
	if got != filepath.Join(root.Dir(), "index.html") || mime != "text/html" {
		t.Fatalf("got %s %s", got, mime)
	}
	if _, mime, err = root.Resolve("xgoef://app/"); err != nil || mime != "text/html" {
		t.Fatalf("dir url: %v %s", err, mime)
	}
	if _, mime, err = root.Resolve("xgoef://app"); err != nil || mime != "text/html" {
		t.Fatalf("bare url: %v %s", err, mime)
	}
	css, mime, err := root.Resolve("xgoef://app/css/../index.html?x=1#y")
	if err != nil {
		t.Fatal(err)
	}
	if css != filepath.Join(root.Dir(), "index.html") || mime != "text/html" {
		t.Fatalf("clean %s %s", css, mime)
	}
	spaced, _, err := root.Resolve("xgoef://app/my%20file.txt")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(spaced, "my file.txt") {
		t.Fatalf("space %s", spaced)
	}
	u, err := root.FileURL(filepath.Join(web, "index.html"))
	if err != nil || u != "xgoef://app/index.html" {
		t.Fatalf("file url %q %v", u, err)
	}
}

func TestRejectEscape(t *testing.T) {
	dir := t.TempDir()
	web := filepath.Join(dir, "web")
	outside := filepath.Join(dir, "secret.txt")
	if err := os.MkdirAll(web, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(web, "index.html"), []byte("hi"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(outside, []byte("no"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(web, "link.txt")); err != nil {
		t.Fatal(err)
	}
	root, err := Open(web)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := root.Resolve("xgoef://app/link.txt"); err == nil || !strings.Contains(err.Error(), "outside") {
		t.Fatalf("symlink: %v", err)
	}
	if _, err := root.FileURL(outside); err == nil || !strings.Contains(err.Error(), "outside") {
		t.Fatalf("file url escape: %v", err)
	}
	if _, _, err := root.Resolve("xgoef://other/index.html"); err == nil {
		t.Fatal("other host should fail")
	}
	if _, _, err := root.Resolve("file:///etc/passwd"); err == nil {
		t.Fatal("file scheme should fail")
	}
	// Dot segments stay inside the root; a missing file is not an escape.
	if _, _, err := root.Resolve("xgoef://app/../../secret.txt"); err == nil || strings.Contains(err.Error(), "outside") {
		t.Fatalf("dot segments: %v", err)
	}
}
