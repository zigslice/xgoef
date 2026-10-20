package shell

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/zigslice/xgoef/ipc"
)

type host struct{ r *ipc.Router }

func (h host) Register(name string, fn ipc.Handler) { h.r.Handle(name, fn) }

func TestOpenURLAndReject(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "note.txt"), []byte("n"), 0o644); err != nil {
		t.Fatal(err)
	}
	p, err := New(dir, filepath.Join(dir, "web"), nil, []string{"$APP"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	p.SetOpener(func(target string) error {
		got = append(got, target)
		return nil
	})
	r := ipc.NewRouter()
	if err := p.Setup(host{r: r}); err != nil {
		t.Fatal(err)
	}
	out, err := r.Invoke("shell:open", []byte(`{"url":"https://example.com/a"}`))
	if err != nil {
		t.Fatal(err)
	}
	if string(out) != `"https://example.com/a"` {
		t.Fatalf("%s", out)
	}
	if _, err := r.Invoke("shell:open", []byte(`{"url":"file:///etc/passwd"}`)); err == nil {
		t.Fatal("file url must be rejected")
	}
	if _, err := r.Invoke("shell:open", []byte(`{"url":"javascript:alert(1)"}`)); err == nil {
		t.Fatal("javascript url must be rejected")
	}
	if _, err := r.Invoke("shell:open", []byte(`{"url":"https://user:pw@example.com"}`)); err == nil {
		t.Fatal("userinfo must be rejected")
	}
	out, err = r.Invoke("shell:open", []byte(`{"path":"note.txt"}`))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(out), "note.txt") {
		t.Fatalf("%s", out)
	}
	if _, err := r.Invoke("shell:open", []byte(`{"path":"../outside"}`)); err == nil || !strings.Contains(err.Error(), "outside") {
		t.Fatalf("path escape: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("opened %v", got)
	}
}

func TestPathDeniedWithoutScope(t *testing.T) {
	p, err := New(t.TempDir(), "", nil, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	p.SetOpener(func(string) error { return nil })
	r := ipc.NewRouter()
	_ = p.Setup(host{r: r})
	if _, err := r.Invoke("shell:open", []byte(`{"path":"note.txt"}`)); err == nil {
		t.Fatal("expected deny")
	}
}

func TestFileSchemeRejectedAtNew(t *testing.T) {
	if _, err := New(t.TempDir(), "", []string{"file"}, nil, nil); err == nil {
		t.Fatal("file scheme should be rejected")
	}
}
