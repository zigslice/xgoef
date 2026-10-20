package bundle

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestPackages(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "web"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "web", "index.html"), []byte("hello-page"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "xgoef-hello"), []byte("bin"), 0o755); err != nil {
		t.Fatal(err)
	}
	paths, err := Write(root, Spec{
		Product:    "xgoef-hello",
		Version:    "0.0.1",
		Identifier: "com.example.hello",
		Arch:       "amd64",
		Schemes:    []string{"hello"},
	})
	if err != nil {
		t.Fatal(err)
	}
	info, err := exec.Command("dpkg-deb", "-I", paths.Deb).CombinedOutput()
	if err != nil {
		t.Fatalf("%v\n%s", err, info)
	}
	text := string(info)
	if !strings.Contains(text, "Package: xgoef-hello") || !strings.Contains(text, "Version: 0.0.1") || !strings.Contains(text, "Architecture: amd64") {
		t.Fatalf("%s", text)
	}
	list, err := exec.Command("dpkg-deb", "-c", paths.Deb).CombinedOutput()
	if err != nil {
		t.Fatalf("%v\n%s", err, list)
	}
	listing := string(list)
	for _, needle := range []string{
		"usr/lib/xgoef-hello/web/index.html",
		"usr/lib/xgoef-hello/xgoef-hello",
		"usr/bin/xgoef-hello",
		"usr/share/applications/xgoef-hello.desktop",
	} {
		if !strings.Contains(listing, needle) {
			t.Fatalf("missing %s\n%s", needle, listing)
		}
	}
	if !strings.Contains(listing, "755") && !strings.Contains(listing, "rwxr-xr-x") {
		t.Fatalf("binary mode\n%s", listing)
	}

	dest := t.TempDir()
	out, err := exec.Command("msiextract", "-C", dest, paths.MSI).CombinedOutput()
	if err != nil {
		t.Fatalf("%v\n%s", err, out)
	}
	page, err := os.ReadFile(filepath.Join(dest, "Program Files", "xgoef-hello", "web", "index.html"))
	if err != nil {
		t.Fatalf("%v\n%s", err, out)
	}
	if string(page) != "hello-page" {
		t.Fatalf("%q", page)
	}

	dmg, err := os.ReadFile(paths.DMG)
	if err != nil {
		t.Fatal(err)
	}
	if string(dmg[len(dmg)-512:len(dmg)-508]) != "koly" {
		t.Fatal("dmg missing koly trailer")
	}
	plist, err := readImageFile(dmg, "xgoef-hello.app/Contents/Info.plist")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(plist), "com.example.hello") || !strings.Contains(string(plist), "hello") {
		t.Fatalf("%s", plist)
	}
	got, err := readImageFile(dmg, "xgoef-hello.app/Contents/Resources/web/index.html")
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "hello-page" {
		t.Fatalf("%q", got)
	}
	bin, err := readImageFile(dmg, "xgoef-hello.app/Contents/MacOS/xgoef-hello")
	if err != nil {
		t.Fatal(err)
	}
	if string(bin) != "bin" {
		t.Fatalf("%q", bin)
	}
}
