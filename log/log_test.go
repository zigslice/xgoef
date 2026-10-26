package log

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestParseLevelAndNames(t *testing.T) {
	info, err := ParseLevel("")
	if err != nil || info != LevelInfo {
		t.Fatalf("%v %v", info, err)
	}
	for _, item := range []struct {
		text string
		lvl  Level
	}{
		{"trace", LevelTrace},
		{"debug", LevelDebug},
		{"info", LevelInfo},
		{"warn", LevelWarn},
		{"error", LevelError},
	} {
		got, err := ParseLevel(item.text)
		if err != nil || got != item.lvl {
			t.Fatalf("%s %v %v", item.text, got, err)
		}
	}
	if err := ValidateLevel("verbose"); err == nil || !strings.Contains(err.Error(), "level is invalid") {
		t.Fatal(err)
	}
	if _, err := FileName(""); err == nil || !strings.Contains(err.Error(), "identifier is required") {
		t.Fatal(err)
	}
	if _, err := FileName("."); err == nil || !strings.Contains(err.Error(), "identifier is empty") {
		t.Fatal(err)
	}
	name, err := FileName("com.example/hello")
	if err != nil || name != "xgoef-com.example_hello.log" {
		t.Fatalf("%s %v", name, err)
	}
	crash, err := CrashFileName("com.example.hello")
	if err != nil || crash != "xgoef-com.example.hello.crash" {
		t.Fatalf("%s %v", crash, err)
	}
}

func TestUserDir(t *testing.T) {
	got, err := userDir("linux", "/var/state", "/home/me")
	if err != nil || got != filepath.Join("/var/state", "xgoef") {
		t.Fatalf("%s %v", got, err)
	}
	got, err = userDir("linux", "", "/home/me")
	if err != nil || got != filepath.Join("/home/me", ".local", "state", "xgoef") {
		t.Fatalf("%s %v", got, err)
	}
	if _, err := userDir("linux", "relative", "/home/me"); err == nil || !strings.Contains(err.Error(), "unavailable") {
		t.Fatal(err)
	}
	if _, err := userDir("darwin", "", "/Users/me"); err == nil || !strings.Contains(err.Error(), "unavailable") {
		t.Fatal(err)
	}
}

func TestWriteFiltersFormatsAndRotates(t *testing.T) {
	dir := t.TempDir()
	var stderr bytes.Buffer
	w, err := Open(dir, "com.example.hello", LevelInfo, &stderr)
	if err != nil {
		t.Fatal(err)
	}
	w.SetNow(func() time.Time {
		return time.Date(2026, 10, 26, 16, 11, 0, 0, time.FixedZone("CST", 8*3600))
	})
	if err := w.Write(LevelDebug, "hidden"); err != nil {
		t.Fatal(err)
	}
	logPath := filepath.Join(dir, "xgoef-com.example.hello.log")
	if _, err := os.Stat(logPath); !os.IsNotExist(err) {
		t.Fatal(err)
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr %q", stderr.String())
	}
	if err := w.Write(LevelInfo, "hello"); err != nil {
		t.Fatal(err)
	}
	body, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatal(err)
	}
	want := "2026-10-26T08:11:00.000Z INFO hello\n"
	if string(body) != want || stderr.String() != want {
		t.Fatalf("file %q stderr %q", body, stderr.String())
	}
	info, err := os.Stat(logPath)
	if err != nil || info.Mode().Perm() != 0o644 {
		t.Fatalf("%v %v", info, err)
	}
	w.SetMaxBytes(len(want))
	if err := w.Write(LevelWarn, "next"); err != nil {
		t.Fatal(err)
	}
	backup, err := os.ReadFile(logPath + ".1")
	if err != nil || string(backup) != want {
		t.Fatalf("%q %v", backup, err)
	}
	body, err = os.ReadFile(logPath)
	if err != nil || !strings.Contains(string(body), " WARN next\n") || strings.Contains(string(body), "hello") {
		t.Fatalf("%q %v", body, err)
	}
	if err := w.Write(LevelError, "third"); err != nil {
		t.Fatal(err)
	}
	backup, err = os.ReadFile(logPath + ".1")
	if err != nil || !strings.Contains(string(backup), " WARN next\n") || strings.Contains(string(backup), "hello") {
		t.Fatalf("replaced backup %q %v", backup, err)
	}
}

func TestWriteRejectsAndRefusesSymlinks(t *testing.T) {
	dir := t.TempDir()
	w, err := Open(dir, "app", LevelTrace, nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, msg := range []string{"", "   ", "line\n", "has\x00nul", strings.Repeat("a", MaxMessage+1)} {
		if err := w.Write(LevelError, msg); err == nil || !strings.Contains(err.Error(), "log:") {
			t.Fatalf("%q %v", msg, err)
		}
	}
	if err := w.Write(LevelTrace, strings.Repeat("a", MaxMessage)); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(dir, "secret")
	if err := os.WriteFile(target, []byte("secret"), 0o644); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(dir, "xgoef-link.log")
	lw, err := Open(dir, "link", LevelInfo, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, link); err != nil {
		t.Fatal(err)
	}
	err = lw.Write(LevelInfo, "nope")
	if err == nil || !strings.Contains(err.Error(), "symlink") {
		t.Fatal(err)
	}
	body, err := os.ReadFile(target)
	if err != nil || string(body) != "secret" {
		t.Fatalf("%q %v", body, err)
	}
	nested := filepath.Join(dir, "xgoef-dir.log")
	dw, err := Open(dir, "dir", LevelInfo, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(nested, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := dw.Write(LevelInfo, "nope"); err == nil || !strings.Contains(err.Error(), "directory") {
		t.Fatal(err)
	}
	if _, err := Open("relative", "app", LevelInfo, nil); err == nil || !strings.Contains(err.Error(), "unavailable") {
		t.Fatal(err)
	}
	if _, err := Open("", "app", LevelInfo, nil); err == nil || !strings.Contains(err.Error(), "unavailable") {
		t.Fatal(err)
	}
}

func TestCrashReplacesSidecar(t *testing.T) {
	dir := t.TempDir()
	var stderr bytes.Buffer
	w, err := Open(dir, "com.example.hello", LevelError, &stderr)
	if err != nil {
		t.Fatal(err)
	}
	w.SetNow(func() time.Time {
		return time.Date(2026, 10, 26, 8, 11, 0, 0, time.UTC)
	})
	if err := w.Write(LevelInfo, "hidden"); err != nil {
		t.Fatal(err)
	}
	if err := w.Crash("a\nb"); err != nil {
		t.Fatal(err)
	}
	if err := w.Crash("latest"); err != nil {
		t.Fatal(err)
	}
	crash, err := os.ReadFile(filepath.Join(dir, "xgoef-com.example.hello.crash"))
	if err != nil {
		t.Fatal(err)
	}
	want := "2026-10-26T08:11:00.000Z CRASH latest\n"
	if string(crash) != want {
		t.Fatalf("%q", crash)
	}
	logBody, err := os.ReadFile(filepath.Join(dir, "xgoef-com.example.hello.log"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(logBody)
	if strings.Contains(text, "hidden") || !strings.Contains(text, "CRASH a b\n") || !strings.HasSuffix(text, want) {
		t.Fatalf("%q", text)
	}
	if !strings.Contains(stderr.String(), "CRASH a b\n") || !strings.HasSuffix(stderr.String(), want) {
		t.Fatalf("%q", stderr.String())
	}
	info, err := os.Stat(filepath.Join(dir, "xgoef-com.example.hello.crash"))
	if err != nil || info.Mode().Perm() != 0o644 {
		t.Fatalf("%v %v", info, err)
	}
}

func TestStderrWithoutFile(t *testing.T) {
	var stderr bytes.Buffer
	w, err := Open("", "app", LevelWarn, &stderr)
	if err != nil {
		t.Fatal(err)
	}
	if err := w.Write(LevelInfo, "skip"); err != nil {
		t.Fatal(err)
	}
	if err := w.Write(LevelWarn, "shown"); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(stderr.String(), " WARN shown\n") || strings.Contains(stderr.String(), "skip") {
		t.Fatalf("%q", stderr.String())
	}
}

func TestRotateRefusesSymlinkBackup(t *testing.T) {
	dir := t.TempDir()
	w, err := Open(dir, "app", LevelInfo, nil)
	if err != nil {
		t.Fatal(err)
	}
	w.SetMaxBytes(1)
	if err := w.Write(LevelInfo, "first"); err != nil {
		t.Fatal(err)
	}
	logPath := filepath.Join(dir, "xgoef-app.log")
	secret := filepath.Join(dir, "secret")
	if err := os.WriteFile(secret, []byte("secret"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(secret, logPath+".1"); err != nil {
		t.Fatal(err)
	}
	err = w.Write(LevelInfo, "second")
	if err == nil || !strings.Contains(err.Error(), "symlink") {
		t.Fatal(err)
	}
	body, err := os.ReadFile(secret)
	if err != nil || string(body) != "secret" {
		t.Fatalf("%q %v", body, err)
	}
	body, err = os.ReadFile(logPath)
	if err != nil || !strings.Contains(string(body), "first") || strings.Contains(string(body), "second") {
		t.Fatalf("%q %v", body, err)
	}
}
