package globalshortcut

import (
	"errors"
	"strings"
	"testing"

	"github.com/zigslice/xgoef/ipc"
	"github.com/zigslice/xgoef/plugin"
)

type host struct{ r *ipc.Router }

func (h host) Register(name string, fn ipc.Handler) { h.r.Handle(name, fn) }

func TestParseCanonical(t *testing.T) {
	got, err := parseShortcut(" ctrl + shift + x ", false)
	if err != nil || got.String() != "Control+Shift+X" {
		t.Fatalf("%v %v", got, err)
	}
	got, err = parseShortcut("CommandOrControl+Alt+KeyF", false)
	if err != nil || got.String() != "Control+Alt+F" {
		t.Fatalf("linux %v %v", got, err)
	}
	got, err = parseShortcut("CmdOrCtrl+F5", true)
	if err != nil || got.String() != "Super+F5" {
		t.Fatalf("darwin %v %v", got, err)
	}
	got, err = parseShortcut("Shift+Control+Up", false)
	if err != nil || got.String() != "Control+Shift+ArrowUp" {
		t.Fatalf("%v %v", got, err)
	}
	for _, raw := range []string{
		"",
		"X",
		"Ctrl+Ctrl+X",
		"Ctrl+A+B",
		"Ctrl++X",
		"Ctrl+Foo",
		"Control+\nShift+X",
		strings.Repeat("A", maxShortcutLen+1),
	} {
		if _, err := parseShortcut(raw, false); !errors.Is(err, errBadShortcut) {
			t.Fatalf("%q: %v", raw, err)
		}
	}
}

func TestRegisterEmitsAndUnregisters(t *testing.T) {
	var events []string
	p := newPlugin(t, []string{"ctrl+shift+x", "Alt+F4"}, func(_ string, payload any) error {
		m := payload.(map[string]string)
		events = append(events, m["shortcut"]+" "+m["state"])
		return nil
	})
	mem := NewMemory()
	p.SetSession(mem)
	r := setup(t, p)
	sc, err := Parse("Control+Shift+X")
	if err != nil {
		t.Fatal(err)
	}
	mem.Press(sc)
	if len(events) != 0 || len(mem.Grabs()) != 0 {
		t.Fatalf("early %v %v", events, mem.Grabs())
	}
	if _, err := r.Invoke("global-shortcut:register", []byte(`{"shortcut":"Control+Shift+X"}`)); err != nil {
		t.Fatal(err)
	}
	if _, err := r.Invoke("global-shortcut:register", []byte(`{"shortcut":"ctrl + shift + x"}`)); err == nil || ipc.CodeOf(err) != "shortcut.registered" {
		t.Fatal(err)
	}
	got, err := r.Invoke("global-shortcut:is_registered", []byte(`{"shortcut":"Shift+Control+X"}`))
	if err != nil || string(got) != "true" {
		t.Fatalf("%s %v", got, err)
	}
	mem.Press(sc)
	mem.Release(sc)
	mem.fire(sc, "down")
	if strings.Join(events, ",") != "Control+Shift+X Pressed,Control+Shift+X Released" {
		t.Fatalf("%v", events)
	}
	if _, err := r.Invoke("global-shortcut:unregister", []byte(`{"shortcut":"Control+Shift+X"}`)); err != nil {
		t.Fatal(err)
	}
	mem.Press(sc)
	if len(events) != 2 || len(mem.Grabs()) != 0 {
		t.Fatalf("%v %v", events, mem.Grabs())
	}
	if _, err := r.Invoke("global-shortcut:unregister", []byte(`{"shortcut":"Alt+F4"}`)); err != nil {
		t.Fatal(err)
	}
}

func TestDeniedInvalidAndGrab(t *testing.T) {
	p := newPlugin(t, []string{"Control+Shift+X"}, nil)
	mem := NewMemory()
	p.SetSession(mem)
	r := setup(t, p)
	if _, err := r.Invoke("global-shortcut:register", []byte(`{"shortcut":"Alt+F4"}`)); err == nil || ipc.CodeOf(err) != "shortcut.denied" || !strings.Contains(err.Error(), "not allowed") {
		t.Fatal(err)
	}
	if len(mem.Grabs()) != 0 {
		t.Fatal(mem.Grabs())
	}
	for _, args := range []string{
		`{}`,
		`{"shortcut":"X"}`,
		`{"shortcut":"Control+Shift+X","extra":true}`,
		`{"shortcut":1}`,
		`[]`,
	} {
		if _, err := r.Invoke("global-shortcut:register", []byte(args)); err == nil || ipc.CodeOf(err) != "shortcut.invalid" {
			t.Fatalf("%s: %v", args, err)
		}
	}
	if _, err := r.Invoke("global-shortcut:unregister_all", []byte(`{"shortcut":"Control+Shift+X"}`)); err == nil || ipc.CodeOf(err) != "shortcut.invalid" {
		t.Fatal(err)
	}
	if _, err := r.Invoke("global-shortcut:is_registered", []byte(`{"shortcut":"nope"}`)); err == nil || ipc.CodeOf(err) != "shortcut.invalid" {
		t.Fatal(err)
	}
	mem.SetError(errors.New("busy"))
	if _, err := r.Invoke("global-shortcut:register", []byte(`{"shortcut":"Control+Shift+X"}`)); err == nil || ipc.CodeOf(err) != "shortcut.grab" || !strings.Contains(err.Error(), "busy") {
		t.Fatal(err)
	}
	got, err := r.Invoke("global-shortcut:is_registered", []byte(`{"shortcut":"Control+Shift+X"}`))
	if err != nil || string(got) != "false" {
		t.Fatalf("%s %v", got, err)
	}
}

func TestUnavailableAndEmptyAllow(t *testing.T) {
	p := newPlugin(t, []string{"Control+Shift+X"}, nil)
	p.SetSession(nil)
	r := setup(t, p)
	if _, err := r.Invoke("global-shortcut:register", []byte(`{"shortcut":"Control+Shift+X"}`)); err == nil || ipc.CodeOf(err) != "shortcut.unavailable" || !strings.Contains(err.Error(), "not available") {
		t.Fatal(err)
	}
	empty, err := New(nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	mem := NewMemory()
	empty.SetSession(mem)
	r = setup(t, empty)
	if _, err := r.Invoke("global-shortcut:register", []byte(`{"shortcut":"Control+Shift+X"}`)); err == nil || ipc.CodeOf(err) != "shortcut.denied" {
		t.Fatal(err)
	}
	if _, err := New([]string{"nope"}, nil); err == nil || !strings.Contains(err.Error(), "not valid") {
		t.Fatal(err)
	}
}

func TestUnregisterAllKeepsFailedGrab(t *testing.T) {
	p := newPlugin(t, []string{"Control+Shift+X", "Alt+F4"}, nil)
	mem := NewMemory()
	p.SetSession(mem)
	r := setup(t, p)
	if _, err := r.Invoke("global-shortcut:register", []byte(`{"shortcut":"Control+Shift+X"}`)); err != nil {
		t.Fatal(err)
	}
	if _, err := r.Invoke("global-shortcut:register", []byte(`{"shortcut":"Alt+F4"}`)); err != nil {
		t.Fatal(err)
	}
	mem.SetError(errors.New("busy"))
	if _, err := r.Invoke("global-shortcut:unregister_all", []byte(`{}`)); err == nil || ipc.CodeOf(err) != "shortcut.grab" {
		t.Fatal(err)
	}
	if len(mem.Grabs()) != 2 {
		t.Fatal(mem.Grabs())
	}
	got, err := r.Invoke("global-shortcut:is_registered", []byte(`{"shortcut":"Alt+F4"}`))
	if err != nil || string(got) != "true" {
		t.Fatalf("%s %v", got, err)
	}
	mem.SetError(nil)
	if err := p.Teardown(host{}); err != nil {
		t.Fatal(err)
	}
	if len(mem.Grabs()) != 0 {
		t.Fatal(mem.Grabs())
	}
}

func newPlugin(t *testing.T, allow []string, emit func(string, any) error) *Plugin {
	t.Helper()
	p, err := New(allow, emit)
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func setup(t *testing.T, p *Plugin) *ipc.Router {
	t.Helper()
	r := ipc.NewRouter()
	if err := p.Setup(host{r: r}); err != nil {
		t.Fatal(err)
	}
	return r
}

var _ plugin.Host = host{}
