package window

import (
	"errors"
	"strings"
	"testing"

	"github.com/zigslice/xgoef/cef"
	"github.com/zigslice/xgoef/ipc"
)

type host struct{ r *ipc.Router }

func (h host) Register(name string, fn ipc.Handler) { h.r.Handle(name, fn) }

type call struct {
	window   string
	op, a, b int
	text     string
	query    int
	sawQuery bool
	fail     error
	answer   bool
}

func setup(t *testing.T, c *call) *ipc.Router {
	t.Helper()
	p := New(func(window string, op, a, b int, text string) error {
		c.window = window
		c.op, c.a, c.b, c.text = op, a, b, text
		return c.fail
	}, func(window string, query int) (bool, error) {
		c.window = window
		c.query = query
		c.sawQuery = true
		return c.answer, c.fail
	})
	r := ipc.NewRouter()
	if err := p.Setup(host{r}); err != nil {
		t.Fatal(err)
	}
	return r
}

func TestVerbsUseCallingWindow(t *testing.T) {
	c := &call{}
	r := setup(t, c)
	for _, cmd := range []string{
		"window:minimize",
		"window:maximize",
		"window:unminimize",
		"window:unmaximize",
		"window:toggle_maximize",
		"window:show",
		"window:hide",
		"window:center",
	} {
		got, err := r.InvokeFrom("", cmd, []byte(`{}`))
		if err != nil || string(got) != "null" || c.window != "main" {
			t.Fatalf("%s %s %v window %q", cmd, got, err, c.window)
		}
	}
	if c.op != cef.WindowOpCenter {
		t.Fatalf("op %d", c.op)
	}
	got, err := r.InvokeFrom("aux", "window:minimize", nil)
	if err != nil || c.window != "aux" || c.op != cef.WindowOpMinimize {
		t.Fatalf("%s %v %q %d", got, err, c.window, c.op)
	}
}

func TestTitleTopFullAndSize(t *testing.T) {
	c := &call{}
	r := setup(t, c)
	if _, err := r.InvokeFrom("main", "window:set_title", []byte(`{"title":""}`)); err != nil || c.text != "" || c.op != cef.WindowOpSetTitle {
		t.Fatal(err)
	}
	if _, err := r.InvokeFrom("main", "window:set_title", []byte(`{"title":"窗口"}`)); err != nil || c.text != "窗口" {
		t.Fatal(err)
	}
	if _, err := r.InvokeFrom("main", "window:set_always_on_top", []byte(`{"alwaysOnTop":true}`)); err != nil || c.a != 1 || c.op != cef.WindowOpSetAlwaysOnTop {
		t.Fatalf("%d %d %v", c.op, c.a, err)
	}
	if _, err := r.InvokeFrom("main", "window:set_always_on_top", []byte(`{"alwaysOnTop":false}`)); err != nil || c.a != 0 {
		t.Fatal(err)
	}
	if _, err := r.InvokeFrom("main", "window:set_fullscreen", []byte(`{"fullscreen":true}`)); err != nil || c.a != 1 || c.op != cef.WindowOpSetFullscreen {
		t.Fatal(err)
	}
	if _, err := r.InvokeFrom("main", "window:set_size", []byte(`{"width":800,"height":600}`)); err != nil || c.a != 800 || c.b != 600 || c.op != cef.WindowOpSetSize {
		t.Fatalf("%d %d %d %v", c.op, c.a, c.b, err)
	}
}

func TestQueries(t *testing.T) {
	c := &call{answer: true}
	r := setup(t, c)
	got, err := r.InvokeFrom("aux", "window:is_maximized", []byte(`{}`))
	if err != nil || string(got) != "true" || c.query != cef.WindowQueryMaximized || c.window != "aux" {
		t.Fatalf("%s %v %d %q", got, err, c.query, c.window)
	}
	c.answer = false
	got, err = r.InvokeFrom("", "window:is_visible", nil)
	if err != nil || string(got) != "false" || c.window != "main" || c.query != cef.WindowQueryVisible {
		t.Fatalf("%s %v %q %d", got, err, c.window, c.query)
	}
}

func TestInvalid(t *testing.T) {
	c := &call{}
	r := setup(t, c)
	cases := []struct {
		cmd  string
		args string
	}{
		{"window:minimize", `{"label":"other"}`},
		{"window:minimize", `null`},
		{"window:minimize", `{"minimize":true}`},
		{"window:set_title", `{}`},
		{"window:set_title", `{"title":"a\nb"}`},
		{"window:set_title", `{"title":"` + strings.Repeat("a", 257) + `"}`},
		{"window:set_title", `{"title":"ok","extra":1}`},
		{"window:set_always_on_top", `{}`},
		{"window:set_always_on_top", `{"alwaysOnTop":1}`},
		{"window:set_fullscreen", `{"fullscreen":"yes"}`},
		{"window:set_size", `{"width":800}`},
		{"window:set_size", `{"width":0,"height":1}`},
		{"window:set_size", `{"width":1.5,"height":1}`},
		{"window:set_size", `{"width":16385,"height":1}`},
		{"window:is_minimized", `{"window":"aux"}`},
	}
	for _, tc := range cases {
		_, err := r.Invoke(tc.cmd, []byte(tc.args))
		if ipc.CodeOf(err) != "window.invalid" {
			t.Fatalf("%s %s %v", tc.cmd, tc.args, err)
		}
	}
	if c.op != 0 || c.sawQuery {
		t.Fatalf("applied op %d query %v", c.op, c.sawQuery)
	}
}

func TestUnavailableAndRejected(t *testing.T) {
	p := New(nil, nil)
	r := ipc.NewRouter()
	if err := p.Setup(host{r}); err != nil {
		t.Fatal(err)
	}
	if _, err := r.Invoke("window:show", []byte(`{}`)); ipc.CodeOf(err) != "window.unavailable" {
		t.Fatal(err)
	}
	if _, err := r.Invoke("window:is_minimized", []byte(`{}`)); ipc.CodeOf(err) != "window.unavailable" {
		t.Fatal(err)
	}
	c := &call{fail: cef.ErrWindowRejected}
	r = setup(t, c)
	if _, err := r.Invoke("window:center", []byte(`{}`)); ipc.CodeOf(err) != "window.invalid" {
		t.Fatal(err)
	}
	if _, err := r.Invoke("window:is_fullscreen", []byte(`{}`)); ipc.CodeOf(err) != "window.invalid" {
		t.Fatal(err)
	}
	c.fail = errors.New("cef: no window")
	if _, err := r.Invoke("window:hide", []byte(`{}`)); ipc.CodeOf(err) != "window.unavailable" || !strings.Contains(err.Error(), "no window") {
		t.Fatal(err)
	}
}
