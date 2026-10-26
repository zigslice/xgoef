package taskbar

import (
	"strings"
	"testing"

	"github.com/zigslice/xgoef/ipc"
)

type host struct{ r *ipc.Router }

func (h host) Register(name string, fn ipc.Handler) { h.r.Handle(name, fn) }

func TestSkipAndProgress(t *testing.T) {
	var gotWindow string
	var gotSkip bool
	var gotProgress int
	var sawProgress bool
	p := New(func(window string, skip bool) error {
		gotWindow = window
		gotSkip = skip
		return nil
	}, func(window string, progress int) error {
		gotWindow = window
		gotProgress = progress
		sawProgress = true
		return nil
	})
	r := ipc.NewRouter()
	if err := p.Setup(host{r}); err != nil {
		t.Fatal(err)
	}
	if _, err := r.InvokeFrom("aux", "taskbar:skip", []byte(`{"skip":true}`)); err != nil {
		t.Fatal(err)
	}
	if gotWindow != "aux" || !gotSkip {
		t.Fatalf("skip %s %v", gotWindow, gotSkip)
	}
	if _, err := r.InvokeFrom("", "taskbar:skip", []byte(`{"skip":false}`)); err != nil {
		t.Fatal(err)
	}
	if gotWindow != "main" || gotSkip {
		t.Fatalf("show %s %v", gotWindow, gotSkip)
	}
	if _, err := r.InvokeFrom("aux", "taskbar:progress", []byte(`{"progress":40}`)); err != nil {
		t.Fatal(err)
	}
	if !sawProgress || gotProgress != 40 || gotWindow != "aux" {
		t.Fatalf("progress %s %d", gotWindow, gotProgress)
	}
	if _, err := r.InvokeFrom("aux", "taskbar:progress", []byte(`{"progress":null}`)); err != nil {
		t.Fatal(err)
	}
	if gotProgress != -1 {
		t.Fatalf("clear %d", gotProgress)
	}
	if _, err := r.InvokeFrom("aux", "taskbar:progress", []byte(`{"progress":0}`)); err != nil {
		t.Fatal(err)
	}
	if gotProgress != 0 {
		t.Fatalf("zero %d", gotProgress)
	}
}

func TestRejectsBadArgs(t *testing.T) {
	called := false
	p := New(func(string, bool) error {
		called = true
		return nil
	}, func(string, int) error {
		called = true
		return nil
	})
	r := ipc.NewRouter()
	if err := p.Setup(host{r}); err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		cmd  string
		args string
	}{
		{"taskbar:skip", `{}`},
		{"taskbar:skip", `{"skip":1}`},
		{"taskbar:skip", `{"skip":true,"window":"other"}`},
		{"taskbar:progress", `{}`},
		{"taskbar:progress", `{"progress":40.5}`},
		{"taskbar:progress", `{"progress":101}`},
		{"taskbar:progress", `{"progress":-1}`},
		{"taskbar:progress", `{"progress":"40"}`},
		{"taskbar:progress", `{"progress":null,"label":"aux"}`},
	}
	for _, tc := range cases {
		if _, err := r.InvokeFrom("main", tc.cmd, []byte(tc.args)); err == nil || ipc.CodeOf(err) != "taskbar.invalid" {
			t.Fatalf("%s %s %v", tc.cmd, tc.args, err)
		}
	}
	if called {
		t.Fatal("bridge called")
	}
}

func TestUnavailable(t *testing.T) {
	p := New(nil, nil)
	r := ipc.NewRouter()
	if err := p.Setup(host{r}); err != nil {
		t.Fatal(err)
	}
	if _, err := r.Invoke("taskbar:skip", []byte(`{"skip":true}`)); err == nil || ipc.CodeOf(err) != "taskbar.unavailable" {
		t.Fatal(err)
	}
	if _, err := r.Invoke("taskbar:progress", []byte(`{"progress":1}`)); err == nil || ipc.CodeOf(err) != "taskbar.unavailable" {
		t.Fatal(err)
	}
	p = New(func(string, bool) error {
		return ipc.Fail("cef.failed", "no window")
	}, func(string, int) error {
		return ipc.Fail("cef.failed", "no window")
	})
	r = ipc.NewRouter()
	if err := p.Setup(host{r}); err != nil {
		t.Fatal(err)
	}
	if _, err := r.Invoke("taskbar:skip", []byte(`{"skip":true}`)); err == nil || ipc.CodeOf(err) != "taskbar.unavailable" || !strings.Contains(err.Error(), "no window") {
		t.Fatal(err)
	}
}
