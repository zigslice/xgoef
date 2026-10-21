package fs

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/zigslice/xgoef/ipc"
	"github.com/zigslice/xgoef/plugin"
)

type host struct{ r *ipc.Router }

func (h host) Register(name string, fn ipc.Handler) { h.r.Handle(name, fn) }

func TestScopeAllowsDenyAndTraversal(t *testing.T) {
	base := t.TempDir()
	app := filepath.Join(base, "app")
	outside := filepath.Join(base, "outside")
	if err := os.MkdirAll(filepath.Join(app, "secret"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(outside, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(app, "ok.txt"), []byte("ok"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(app, "secret", "no.txt"), []byte("no"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(outside, "x.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(outside, "x.txt"), filepath.Join(app, "link")); err != nil {
		t.Fatal(err)
	}

	p, err := New(app, filepath.Join(app, "web"), []string{"$APP"}, []string{"$APP/secret"})
	if err != nil {
		t.Fatal(err)
	}
	router := ipc.NewRouter()
	if err := p.Setup(host{r: router}); err != nil {
		t.Fatal(err)
	}

	got, err := router.Invoke("fs:read_text", []byte(`{"path":"ok.txt"}`))
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != `"ok"` {
		t.Fatalf("read %s", got)
	}
	if _, err := router.Invoke("fs:read_text", []byte(`{"path":"secret/no.txt"}`)); err == nil || !strings.Contains(err.Error(), "outside") || ipc.CodeOf(err) != "fs.denied" {
		t.Fatalf("deny secret: %v", err)
	}
	if _, err := router.Invoke("fs:read_text", []byte(`{"path":"../outside/x.txt"}`)); err == nil || !strings.Contains(err.Error(), "outside") {
		t.Fatalf("deny traversal: %v", err)
	}
	if _, err := router.Invoke("fs:read_text", []byte(`{"path":"link"}`)); err == nil || !strings.Contains(err.Error(), "outside") {
		t.Fatalf("deny symlink: %v", err)
	}

	if _, err := router.Invoke("fs:write_text", []byte(`{"path":"note.txt","contents":"hi"}`)); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(filepath.Join(app, "note.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if string(b) != "hi" {
		t.Fatalf("wrote %q", b)
	}

	exists, err := router.Invoke("fs:exists", []byte(`{"path":"missing.txt"}`))
	if err != nil {
		t.Fatal(err)
	}
	if string(exists) != `false` {
		t.Fatalf("exists %s", exists)
	}

	list, err := router.Invoke("fs:list", []byte(`{"path":"."}`))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(list), `"ok.txt"`) {
		t.Fatalf("list %s", list)
	}
}

func TestEmptyAllowDenies(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "a.txt"), []byte("a"), 0o644); err != nil {
		t.Fatal(err)
	}
	p, err := New(dir, filepath.Join(dir, "web"), nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	router := ipc.NewRouter()
	_ = p.Setup(host{r: router})
	if _, err := router.Invoke("fs:read_text", []byte(`{"path":"a.txt"}`)); err == nil {
		t.Fatal("empty allow must deny")
	}
}

func TestPermissions(t *testing.T) {
	p, err := New(t.TempDir(), "", []string{"$APP"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if p.Name() != "fs" {
		t.Fatal(p.Name())
	}
	var _ plugin.Plugin = p
	if len(p.Permissions()) != 4 {
		t.Fatalf("%v", p.Permissions())
	}
}
