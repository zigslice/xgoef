package autostart

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/zigslice/xgoef/ipc"
)

type host struct{ r *ipc.Router }

func (h host) Register(name string, fn ipc.Handler) { h.r.Handle(name, fn) }

func TestEnableUsesConfigArgs(t *testing.T) {
	dir := t.TempDir()
	exe := filepath.Join(t.TempDir(), "xgoef-hello")
	p, err := New("com.example.hello", "Hello", exe, []string{"--from-config"})
	if err != nil {
		t.Fatal(err)
	}
	if p.Name() != "autostart" || len(p.Permissions()) != 3 {
		t.Fatalf("%s %v", p.Name(), p.Permissions())
	}
	p.SetDir(dir)
	r := ipc.NewRouter()
	if err := p.Setup(host{r}); err != nil {
		t.Fatal(err)
	}
	for _, args := range []string{`{"args":["--evil"]}`, `{"exe":"/bin/sh"}`, `[]`, `"no"`} {
		_, err := r.Invoke("autostart:enable", []byte(args))
		if err == nil || ipc.CodeOf(err) != "autostart.invalid" {
			t.Fatalf("%s %v", args, err)
		}
	}
	entries, err := os.ReadDir(dir)
	if err != nil || len(entries) != 0 {
		t.Fatalf("wrote %v %v", entries, err)
	}
	for _, args := range []string{`{}`, `null`, ``} {
		out, err := r.Invoke("autostart:is_enabled", []byte(args))
		if err != nil || string(out) != "false" {
			t.Fatalf("%q %s %v", args, out, err)
		}
	}
	out, err := r.Invoke("autostart:enable", []byte(`{}`))
	if err != nil || string(out) != "null" {
		t.Fatalf("%s %v", out, err)
	}
	body, err := os.ReadFile(filepath.Join(dir, "xgoef-com.example.hello.desktop"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(body)
	if !strings.Contains(text, `Exec="`+exe+`" "--from-config"`) || strings.Contains(text, "--evil") {
		t.Fatalf("%s", text)
	}
	out, err = r.Invoke("autostart:is_enabled", nil)
	if err != nil || string(out) != "true" {
		t.Fatalf("%s %v", out, err)
	}
	if _, err := r.Invoke("autostart:disable", []byte(`{}`)); err != nil {
		t.Fatal(err)
	}
	out, err = r.Invoke("autostart:is_enabled", []byte(`{}`))
	if err != nil || string(out) != "false" {
		t.Fatalf("%s %v", out, err)
	}
	if _, err := r.Invoke("autostart:disable", nil); err != nil {
		t.Fatal(err)
	}
}

func TestUnavailableDoesNotTouchDir(t *testing.T) {
	dir := t.TempDir()
	exe := filepath.Join(t.TempDir(), "xgoef-hello")
	p, err := New("com.example.hello", "Hello", exe, nil)
	if err != nil {
		t.Fatal(err)
	}
	p.SetDir(dir)
	p.system = "darwin"
	r := ipc.NewRouter()
	if err := p.Setup(host{r}); err != nil {
		t.Fatal(err)
	}
	for _, cmd := range []string{"autostart:enable", "autostart:disable", "autostart:is_enabled"} {
		_, err := r.Invoke(cmd, []byte(`{}`))
		if err == nil || ipc.CodeOf(err) != "autostart.unavailable" {
			t.Fatalf("%s %v", cmd, err)
		}
	}
	entries, err := os.ReadDir(dir)
	if err != nil || len(entries) != 0 {
		t.Fatalf("%v %v", entries, err)
	}
	if _, err := New("", "Hello", exe, nil); err == nil || !strings.Contains(err.Error(), "identifier is required") {
		t.Fatal(err)
	}
	if _, err := New("..", "Hello", exe, nil); err == nil || !strings.Contains(err.Error(), "identifier is empty") {
		t.Fatal(err)
	}
	if _, err := New("com.example.hello", " ", exe, nil); err == nil {
		t.Fatal("empty name")
	}
}

func TestSymlinkIsDenied(t *testing.T) {
	dir := t.TempDir()
	exe := filepath.Join(t.TempDir(), "xgoef-hello")
	target := filepath.Join(t.TempDir(), "secret")
	if err := os.WriteFile(target, []byte("SECRET"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, filepath.Join(dir, "xgoef-com.example.hello.desktop")); err != nil {
		t.Fatal(err)
	}
	p, err := New("com.example.hello", "Hello", exe, nil)
	if err != nil {
		t.Fatal(err)
	}
	p.SetDir(dir)
	r := ipc.NewRouter()
	if err := p.Setup(host{r}); err != nil {
		t.Fatal(err)
	}
	_, err = r.Invoke("autostart:enable", []byte(`{}`))
	if err == nil || ipc.CodeOf(err) != "autostart.denied" || strings.Contains(err.Error(), "SECRET") {
		t.Fatal(err)
	}
	got, err := os.ReadFile(target)
	if err != nil || string(got) != "SECRET" {
		t.Fatalf("%s %v", got, err)
	}
}
