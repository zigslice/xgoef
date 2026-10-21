package dialog

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/zigslice/xgoef/ipc"
)

type host struct{ r *ipc.Router }

func (h host) Register(name string, fn ipc.Handler) { h.r.Handle(name, fn) }

func TestPickerScopeAndCancel(t *testing.T) {
	dir := t.TempDir()
	note := filepath.Join(dir, "note.txt")
	if err := os.WriteFile(note, []byte("n"), 0o644); err != nil {
		t.Fatal(err)
	}
	outside := filepath.Join(filepath.Dir(dir), "no.txt")
	if err := os.WriteFile(outside, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	next := Result{Path: note}
	p, err := New(dir, filepath.Join(dir, "web"), []string{"$APP"}, nil, func(req Request) (Result, error) {
		if req.Save && req.Title != "save" {
			t.Fatalf("req %+v", req)
		}
		return next, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	r := ipc.NewRouter()
	if err := p.Setup(host{r: r}); err != nil {
		t.Fatal(err)
	}
	out, err := r.Invoke("dialog:open", []byte(`{"title":"open"}`))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(out), "note.txt") {
		t.Fatalf("%s", out)
	}
	next = Result{Path: outside}
	if _, err := r.Invoke("dialog:open", []byte(`{}`)); err == nil || !strings.Contains(err.Error(), "outside") || ipc.CodeOf(err) != "dialog.denied" {
		t.Fatalf("outside: %v", err)
	}
	next = Result{Cancelled: true}
	out, err = r.Invoke("dialog:save", []byte(`{"title":"save"}`))
	if err != nil || string(out) != "null" {
		t.Fatalf("cancel %s %v", out, err)
	}
}
