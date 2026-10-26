package eval

import (
	"strings"
	"testing"

	gate "github.com/zigslice/xgoef/eval"
	"github.com/zigslice/xgoef/ipc"
)

type host struct{ r *ipc.Router }

func (h host) Register(name string, fn ipc.Handler) { h.r.Handle(name, fn) }

func TestRunUsesRegisteredSource(t *testing.T) {
	const source = `window.__xgoefMark = "marked";`
	var gotWindow, gotSource string
	p := New(map[string]string{"mark": source}, func(window, script string) error {
		gotWindow = window
		gotSource = script
		return nil
	})
	if p.Name() != "eval" || len(p.Permissions()) != 1 || p.Permissions()[0] != "eval:run" {
		t.Fatalf("%s %v", p.Name(), p.Permissions())
	}
	r := ipc.NewRouter()
	if err := p.Setup(host{r}); err != nil {
		t.Fatal(err)
	}
	out, err := r.InvokeFrom("aux", "eval:run", []byte(`{"name":"mark"}`))
	if err != nil || string(out) != "null" || gotWindow != "aux" || gotSource != source {
		t.Fatalf("%s %v window %q source %q", out, err, gotWindow, gotSource)
	}
	gotWindow = ""
	if _, err := r.Invoke("eval:run", []byte(`{"name":"mark"}`)); err != nil || gotWindow != "main" {
		t.Fatalf("empty label %v %q", err, gotWindow)
	}
}

func TestRejectsPageSourceAndTarget(t *testing.T) {
	called := false
	p := New(map[string]string{"mark": "window.ok = 1;"}, func(string, string) error {
		called = true
		return nil
	})
	r := ipc.NewRouter()
	if err := p.Setup(host{r}); err != nil {
		t.Fatal(err)
	}
	cases := []string{
		`{"name":"mark","script":"alert(1)"}`,
		`{"script":"alert(1)"}`,
		`{"name":"mark","window":"other"}`,
		`{"code":"alert(1)"}`,
		`{"name":"mark","source":"alert(1)"}`,
	}
	for _, args := range cases {
		_, err := r.InvokeFrom("main", "eval:run", []byte(args))
		if err == nil || ipc.CodeOf(err) != "eval.denied" || !strings.Contains(err.Error(), "only a script name") {
			t.Fatalf("%s %v", args, err)
		}
	}
	if called {
		t.Fatal("runner was called")
	}
	_, err := r.Invoke("eval:run", []byte(`{"name":"missing"}`))
	if err == nil || ipc.CodeOf(err) != "eval.denied" || !strings.Contains(err.Error(), "not registered") {
		t.Fatalf("missing %v", err)
	}
	if called {
		t.Fatal("runner was called for a missing name")
	}
}

func TestInvalidNameAndRunnerErrors(t *testing.T) {
	p := New(map[string]string{"mark": "window.ok = 1;"}, func(string, string) error {
		return gate.ErrOrigin
	})
	r := ipc.NewRouter()
	if err := p.Setup(host{r}); err != nil {
		t.Fatal(err)
	}
	for _, args := range []string{`{}`, `{"name":"Mark"}`, `{"name":"window.ok = 1;"}`, `not-json`, `[]`} {
		_, err := r.Invoke("eval:run", []byte(args))
		if err == nil || ipc.CodeOf(err) != "eval.invalid" {
			t.Fatalf("%s %v", args, err)
		}
	}
	_, err := r.Invoke("eval:run", []byte(`{"name":"mark"}`))
	if err == nil || ipc.CodeOf(err) != "eval.denied" || strings.Contains(err.Error(), "window.ok") {
		t.Fatalf("origin %v", err)
	}
	p = New(map[string]string{"mark": "window.ok = 1;"}, func(string, string) error {
		return gate.ErrUnpinned
	})
	r = ipc.NewRouter()
	if err := p.Setup(host{r}); err != nil {
		t.Fatal(err)
	}
	_, err = r.Invoke("eval:run", []byte(`{"name":"mark"}`))
	if err == nil || ipc.CodeOf(err) != "eval.denied" {
		t.Fatalf("unpinned %v", err)
	}
	p = New(map[string]string{"mark": "window.ok = 1;"}, nil)
	r = ipc.NewRouter()
	if err := p.Setup(host{r}); err != nil {
		t.Fatal(err)
	}
	_, err = r.Invoke("eval:run", []byte(`{"name":"mark"}`))
	if err == nil || ipc.CodeOf(err) != "eval.unavailable" {
		t.Fatalf("nil runner %v", err)
	}
	p = New(map[string]string{"mark": "window.ok = 1;"}, func(string, string) error {
		return gate.ErrRejected
	})
	r = ipc.NewRouter()
	if err := p.Setup(host{r}); err != nil {
		t.Fatal(err)
	}
	_, err = r.Invoke("eval:run", []byte(`{"name":"mark"}`))
	if err == nil || ipc.CodeOf(err) != "eval.invalid" || strings.Contains(err.Error(), "window.ok") {
		t.Fatalf("rejected %v", err)
	}
}
