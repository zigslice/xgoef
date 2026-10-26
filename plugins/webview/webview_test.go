package webview

import (
	"errors"
	"strings"
	"testing"

	"github.com/zigslice/xgoef/ipc"
)

type host struct{ r *ipc.Router }

func (h host) Register(name string, fn ipc.Handler) { h.r.Handle(name, fn) }

func TestPrintUsesCallingWindow(t *testing.T) {
	var got string
	p := New(false, func(window string) error {
		got = window
		return nil
	}, nil, nil, nil)
	if p.Name() != "webview" {
		t.Fatalf("name %q", p.Name())
	}
	r := ipc.NewRouter()
	if err := p.Setup(host{r}); err != nil {
		t.Fatal(err)
	}
	out, err := r.InvokeFrom("aux", "webview:print", []byte(`{"extra":true}`))
	if err != nil || string(out) != "null" || got != "aux" {
		t.Fatalf("%s %v window %q", out, err, got)
	}
	out, err = r.Invoke("webview:print", []byte(`not-json`))
	if err != nil || string(out) != "null" || got != "main" {
		t.Fatalf("empty label %s %v window %q", out, err, got)
	}
}

func TestDevToolsRoundTrip(t *testing.T) {
	open := false
	var opened, closed string
	p := New(true,
		nil,
		func(window string) error {
			opened = window
			open = true
			return nil
		},
		func(window string) error {
			closed = window
			open = false
			return nil
		},
		func(window string) (bool, error) {
			if window != "main" {
				t.Fatalf("window %q", window)
			}
			return open, nil
		},
	)
	r := ipc.NewRouter()
	if err := p.Setup(host{r}); err != nil {
		t.Fatal(err)
	}
	got, err := r.InvokeFrom("", "webview:is_dev_tools_open", []byte(`{}`))
	if err != nil || string(got) != "false" {
		t.Fatalf("closed %s %v", got, err)
	}
	if _, err := r.Invoke("webview:open_dev_tools", []byte(`{}`)); err != nil || opened != "main" {
		t.Fatalf("open %v %q", err, opened)
	}
	got, err = r.Invoke("webview:is_dev_tools_open", []byte(`{}`))
	if err != nil || string(got) != "true" {
		t.Fatalf("open query %s %v", got, err)
	}
	if _, err := r.Invoke("webview:close_dev_tools", []byte(`{}`)); err != nil || closed != "main" {
		t.Fatalf("close %v %q", err, closed)
	}
	got, err = r.Invoke("webview:is_dev_tools_open", []byte(`{}`))
	if err != nil || string(got) != "false" {
		t.Fatalf("after close %s %v", got, err)
	}
}

func TestDevToolsDeniedDoesNotCallBridge(t *testing.T) {
	called := false
	p := New(false, nil,
		func(string) error { called = true; return nil },
		func(string) error { called = true; return nil },
		func(string) (bool, error) { called = true; return true, nil },
	)
	r := ipc.NewRouter()
	if err := p.Setup(host{r}); err != nil {
		t.Fatal(err)
	}
	for _, cmd := range []string{"webview:open_dev_tools", "webview:close_dev_tools", "webview:is_dev_tools_open"} {
		_, err := r.Invoke(cmd, []byte(`{}`))
		if err == nil || ipc.CodeOf(err) != "webview.denied" || !strings.Contains(err.Error(), "disabled") {
			t.Fatalf("%s %v", cmd, err)
		}
	}
	if called {
		t.Fatal("bridge was called")
	}
	if _, err := r.Invoke("webview:print", []byte(`{}`)); err == nil || ipc.CodeOf(err) != "webview.unavailable" {
		t.Fatalf("print %v", err)
	}
}

func TestUnavailable(t *testing.T) {
	p := New(true, nil, nil, nil, nil)
	r := ipc.NewRouter()
	if err := p.Setup(host{r}); err != nil {
		t.Fatal(err)
	}
	for _, cmd := range []string{"webview:open_dev_tools", "webview:close_dev_tools", "webview:is_dev_tools_open"} {
		_, err := r.Invoke(cmd, []byte(`{}`))
		if err == nil || ipc.CodeOf(err) != "webview.unavailable" {
			t.Fatalf("%s %v", cmd, err)
		}
	}
	boom := errors.New("cef: no browser")
	p = New(true,
		func(string) error { return boom },
		func(string) error { return boom },
		func(string) error { return nil },
		func(string) (bool, error) { return false, boom },
	)
	r = ipc.NewRouter()
	if err := p.Setup(host{r}); err != nil {
		t.Fatal(err)
	}
	_, err := r.Invoke("webview:print", []byte(`{}`))
	if err == nil || ipc.CodeOf(err) != "webview.unavailable" || !strings.Contains(err.Error(), "no browser") {
		t.Fatalf("print %v", err)
	}
	_, err = r.Invoke("webview:open_dev_tools", []byte(`{}`))
	if err == nil || ipc.CodeOf(err) != "webview.unavailable" {
		t.Fatalf("open %v", err)
	}
	if _, err := r.Invoke("webview:close_dev_tools", []byte(`{}`)); err != nil {
		t.Fatal(err)
	}
}
