package bundle

import (
	"bytes"
	"encoding/binary"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
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

	raw, err := os.ReadFile(paths.AppImage)
	if err != nil {
		t.Fatal(err)
	}
	st, err := os.Stat(paths.AppImage)
	if err != nil {
		t.Fatal(err)
	}
	if st.Mode().Perm()&0o111 == 0 {
		t.Fatalf("appimage mode %o", st.Mode().Perm())
	}
	destImage := filepath.Join(t.TempDir(), "image")
	out, err = exec.Command("unsquashfs", "-d", destImage, "-offset", appImageOffset(raw), paths.AppImage).CombinedOutput()
	if err != nil {
		t.Fatalf("%v\n%s", err, out)
	}
	page, err = os.ReadFile(filepath.Join(destImage, "usr", "lib", "xgoef-hello", "web", "index.html"))
	if err != nil {
		t.Fatalf("%v\n%s", err, out)
	}
	if string(page) != "hello-page" {
		t.Fatalf("%q", page)
	}
	desktop, err := os.ReadFile(filepath.Join(destImage, "xgoef-hello.desktop"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(desktop), "x-scheme-handler/hello;") || !strings.Contains(string(desktop), "Icon=xgoef-hello") {
		t.Fatalf("%s", desktop)
	}
	run, err := os.ReadFile(filepath.Join(destImage, "AppRun"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(run), "/usr/bin/xgoef-hello") {
		t.Fatalf("%s", run)
	}
	target, err := os.Readlink(filepath.Join(destImage, "usr", "bin", "xgoef-hello"))
	if err != nil {
		t.Fatal(err)
	}
	if target != "../lib/xgoef-hello/xgoef-hello" {
		t.Fatalf("%s", target)
	}
}

func TestAppImageRuns(t *testing.T) {
	root := t.TempDir()
	script := "#!/bin/sh\necho appimage-ok\n"
	if err := os.WriteFile(filepath.Join(root, "xgoef-hello"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	blob := bytes.Repeat([]byte{'Z'}, sqBlockSize+10)
	if err := os.WriteFile(filepath.Join(root, "blob"), blob, 0o644); err != nil {
		t.Fatal(err)
	}
	paths, err := Write(root, Spec{
		Product:    "xgoef-hello",
		Version:    "0.0.1",
		Identifier: "com.example.hello",
		Arch:       "amd64",
	})
	if err != nil {
		t.Fatal(err)
	}
	cache := t.TempDir()
	cmd := exec.Command(paths.AppImage)
	cmd.Env = append(os.Environ(), "XDG_CACHE_HOME="+cache)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("%v\n%s", err, out)
	}
	if string(out) != "appimage-ok\n" {
		t.Fatalf("%q", out)
	}
	got, err := os.ReadFile(filepath.Join(cache, "xgoef-appimage", mustCacheDir(t, cache), "usr", "lib", "xgoef-hello", "blob"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, blob) {
		t.Fatalf("blob %d", len(got))
	}
}

func appImageOffset(raw []byte) string {
	marker := []byte("XGOEF_SQ_OFFSET!")
	at := bytes.Index(raw, marker)
	if at < 0 || at+24 > len(raw) {
		return "0"
	}
	return strconv.FormatUint(binary.LittleEndian.Uint64(raw[at+16:]), 10)
}

func mustCacheDir(t *testing.T, cache string) string {
	t.Helper()
	matches, err := filepath.Glob(filepath.Join(cache, "xgoef-appimage", "*"))
	if err != nil || len(matches) != 1 {
		t.Fatalf("%v %v", matches, err)
	}
	return filepath.Base(matches[0])
}
