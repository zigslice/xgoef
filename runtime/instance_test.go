package runtime

import (
	"fmt"
	"strings"
	"testing"

	"github.com/zigslice/xgoef/cef"
	"github.com/zigslice/xgoef/conf"
)

func TestSecondInstanceFocusesTheFirst(t *testing.T) {
	dir := t.TempDir()
	cfg := conf.File{Identifier: "com.example.hello"}
	first := New(cfg, nil)
	first.SetInstanceDir(dir)
	var focused string
	var args string
	first.Events.Listen("single-instance", func(payload any) {
		args = fmt.Sprint(payload)
	})
	orig := first.Bridge
	first.Bridge = focusBridge{Bridge: orig, focused: &focused}
	if err := first.acquireInstance(); err != nil {
		t.Fatal(err)
	}
	defer first.closeInstance()

	second := New(cfg, nil)
	second.SetInstanceDir(dir)
	err := second.acquireInstance()
	ee, ok := err.(*ExitError)
	if !ok || ee.Code != 0 {
		t.Fatalf("%v", err)
	}
	if focused != "main" || !strings.Contains(args, "args") {
		t.Fatalf("focus %q args %q", focused, args)
	}
}

func TestSingleInstanceCanBeDisabled(t *testing.T) {
	off := false
	dir := t.TempDir()
	cfg := conf.File{
		Identifier: "com.example.hello",
		App:        conf.AppSection{SingleInstance: &off},
	}
	a := New(cfg, nil)
	a.SetInstanceDir(dir)
	b := New(cfg, nil)
	b.SetInstanceDir(dir)
	if err := a.acquireInstance(); err != nil {
		t.Fatal(err)
	}
	if err := b.acquireInstance(); err != nil {
		t.Fatal(err)
	}
	if a.inst != nil || b.inst != nil {
		t.Fatal("disabled app took the socket")
	}
}

func TestSingleInstanceRequiresIdentifier(t *testing.T) {
	on := true
	app := New(conf.File{App: conf.AppSection{SingleInstance: &on}}, nil)
	if err := app.acquireInstance(); err == nil || !strings.Contains(err.Error(), "identifier") {
		t.Fatal(err)
	}
}

type focusBridge struct {
	cef.Bridge
	focused *string
}

func (b focusBridge) FocusWindow(label string) error {
	*b.focused = label
	return b.Bridge.FocusWindow(label)
}
