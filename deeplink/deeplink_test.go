package deeplink

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNormalizeRejectsReservedAndDuplicates(t *testing.T) {
	got, err := Normalize([]string{"Hello", "notes"})
	if err != nil || len(got) != 2 || got[0] != "hello" || got[1] != "notes" {
		t.Fatalf("%v %v", got, err)
	}
	if _, err := Normalize(nil); err != nil {
		t.Fatal(err)
	}
	for _, bad := range [][]string{{"http"}, {"xgoef"}, {"../no"}, {"hello", "Hello"}, {" "}} {
		if _, err := Normalize(bad); err == nil {
			t.Fatalf("accepted %q", bad)
		}
	}
}

func TestFromArgsKeepsRegisteredSchemes(t *testing.T) {
	args := []string{"bin", "--url=hello://ignored", "hello://item/1", "https://example", "notes://a"}
	got := FromArgs(args, []string{"hello", "notes"})
	if len(got) != 2 || got[0] != "hello://item/1" || got[1] != "notes://a" {
		t.Fatalf("%q", got)
	}
}

func TestInstallWritesDesktopAndMIME(t *testing.T) {
	apps := t.TempDir()
	cfg := t.TempDir()
	exec := filepath.Join(t.TempDir(), "xgoef-hello")
	if err := os.WriteFile(filepath.Join(cfg, "mimeapps.list"), []byte("[Default Applications]\nx-scheme-handler/other=other.desktop\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	path, err := Install(apps, cfg, "com.example.hello", "Hello", exec, []string{"Hello"})
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Dir(path) != apps {
		t.Fatalf("path %s", path)
	}
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	text := string(body)
	if !strings.Contains(text, "x-scheme-handler/hello;") || !strings.Contains(text, "%u") || !strings.Contains(text, exec) {
		t.Fatalf("%s", text)
	}
	st, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if st.Mode().Perm() != 0o644 {
		t.Fatalf("mode %o", st.Mode().Perm())
	}
	mime, err := os.ReadFile(filepath.Join(cfg, "mimeapps.list"))
	if err != nil {
		t.Fatal(err)
	}
	mimeText := string(mime)
	if !strings.Contains(mimeText, "x-scheme-handler/hello=xgoef-com.example.hello.desktop") || !strings.Contains(mimeText, "x-scheme-handler/other=other.desktop") {
		t.Fatalf("%s", mimeText)
	}
	if _, err := Install(apps, cfg, "../escape", "Hello", exec, []string{"hello"}); err != nil {
		t.Fatal(err)
	}
	escaped := filepath.Join(apps, "xgoef-.._escape.desktop")
	if _, err := os.Stat(escaped); err != nil {
		t.Fatal(err)
	}
}

func TestFileRejectsRelativeExecutable(t *testing.T) {
	if _, err := File("Hello", "xgoef-hello", []string{"hello"}); err == nil {
		t.Fatal("relative executable accepted")
	}
}
