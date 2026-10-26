package log

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/zigslice/xgoef/ipc"
)

type host struct{ r *ipc.Router }

func (h host) Register(name string, fn ipc.Handler) { h.r.Handle(name, fn) }

func TestWriteUsesLevelAndRejectsExtraFields(t *testing.T) {
	dir := t.TempDir()
	p, err := New("com.example.hello", "warn")
	if err != nil {
		t.Fatal(err)
	}
	if p.Name() != "log" || len(p.Permissions()) != 1 || p.Permissions()[0] != "log:write" {
		t.Fatalf("%s %v", p.Name(), p.Permissions())
	}
	p.SetDir(dir)
	p.SetStderr(nil)
	r := ipc.NewRouter()
	if err := p.Setup(host{r}); err != nil {
		t.Fatal(err)
	}
	for _, args := range []string{
		`{"level":"info","message":"nope","path":"/tmp/x"}`,
		`{"level":"error","message":"nope","crash":true}`,
		`{"message":"nope","file":"x"}`,
	} {
		_, err := r.Invoke("log:write", []byte(args))
		if err == nil || ipc.CodeOf(err) != "log.denied" {
			t.Fatalf("%s %v", args, err)
		}
	}
	entries, err := os.ReadDir(dir)
	if err != nil || len(entries) != 0 {
		t.Fatalf("%v %v", entries, err)
	}
	out, err := r.Invoke("log:write", []byte(`{"level":"info","message":"hidden"}`))
	if err != nil || string(out) != "null" {
		t.Fatalf("%s %v", out, err)
	}
	if entries, err = os.ReadDir(dir); err != nil || len(entries) != 0 {
		t.Fatalf("filtered write created %v %v", entries, err)
	}
	for _, args := range []string{`{}`, `{"level":"error"}`, `{"message":" "}`, `{"level":"fatal","message":"x"}`, `[]`} {
		_, err := r.Invoke("log:write", []byte(args))
		if err == nil || ipc.CodeOf(err) != "log.invalid" {
			t.Fatalf("%s %v", args, err)
		}
	}
	out, err = r.Invoke("log:write", []byte(`{"message":"also-hidden"}`))
	if err != nil || string(out) != "null" {
		t.Fatalf("%s %v", out, err)
	}
	if entries, err = os.ReadDir(dir); err != nil || len(entries) != 0 {
		t.Fatalf("omitted level created %v %v", entries, err)
	}
	out, err = r.Invoke("log:write", []byte(`{"level":"warn","message":"shown"}`))
	if err != nil || string(out) != "null" {
		t.Fatalf("%s %v", out, err)
	}
	body, err := os.ReadFile(filepath.Join(dir, "xgoef-com.example.hello.log"))
	if err != nil || !strings.Contains(string(body), " WARN shown\n") || strings.Contains(string(body), "hidden") {
		t.Fatalf("%q %v", body, err)
	}
	if _, err := os.Stat(filepath.Join(dir, "xgoef-com.example.hello.crash")); !os.IsNotExist(err) {
		t.Fatal(err)
	}
}

func TestCrashIsHostOnly(t *testing.T) {
	dir := t.TempDir()
	p, err := New("app", "")
	if err != nil {
		t.Fatal(err)
	}
	p.SetDir(dir)
	p.SetStderr(nil)
	if err := p.Crash("boom"); err != nil {
		t.Fatal(err)
	}
	crash, err := os.ReadFile(filepath.Join(dir, "xgoef-app.crash"))
	if err != nil || !strings.Contains(string(crash), " CRASH boom\n") {
		t.Fatalf("%q %v", crash, err)
	}
}

func TestUnavailableWithoutTargets(t *testing.T) {
	p, err := New("app", "info")
	if err != nil {
		t.Fatal(err)
	}
	p.SetDir("")
	p.SetStderr(nil)
	r := ipc.NewRouter()
	if err := p.Setup(host{r}); err != nil {
		t.Fatal(err)
	}
	_, err = r.Invoke("log:write", []byte(`{"message":"x"}`))
	if err == nil || ipc.CodeOf(err) != "log.unavailable" {
		t.Fatal(err)
	}
}

func TestOpenRejectsIdentifierAndLevel(t *testing.T) {
	if _, err := New("", "info"); err == nil || !strings.Contains(err.Error(), "identifier is required") {
		t.Fatal(err)
	}
	if _, err := New("app", "verbose"); err == nil || !strings.Contains(err.Error(), "level is invalid") {
		t.Fatal(err)
	}
}
