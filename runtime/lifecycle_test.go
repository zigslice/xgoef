package runtime

import (
	"fmt"
	"strings"
	"testing"

	"github.com/zigslice/xgoef/cef"
	"github.com/zigslice/xgoef/conf"
)

func TestLifecycleReadyAndExitBracketTheLoop(t *testing.T) {
	dir := t.TempDir()
	var order []string
	host := &orderHost{note: func(step string) { order = append(order, step) }}
	app := New(conf.File{
		ProductName: "Hello",
		App:         conf.AppSection{Windows: []conf.Window{{Label: "main", URL: "about:blank"}}},
	}, host)
	app.SetBase(dir)
	app.OnLifecycle(nil)
	app.OnLifecycle(func(ev Lifecycle) {
		order = append(order, ev.Event)
		if ev.Event != LifecyclePageLoad && (ev.Label != "" || ev.URL != "") {
			t.Fatalf("%+v", ev)
		}
	})
	var second []string
	app.OnLifecycle(func(ev Lifecycle) { second = append(second, ev.Event) })
	if err := app.Run(); err != nil {
		t.Fatal(err)
	}
	if strings.Join(order, ",") != "browser,ready,loop,exit,shutdown" {
		t.Fatalf("%v", order)
	}
	if strings.Join(second, ",") != "ready,exit" {
		t.Fatalf("%v", second)
	}

	bridge := app.Bridge.(*cef.NativeBridge)
	bridge.NotePageLoad("main", "xgoef://app/index.html")
	bridge.NotePageLoad("aux", "http://127.0.0.1:5173/")
	bridge.NotePageLoad("", "about:blank")
	bridge.NotePageLoad("main", "http://example.test/a\nb")
	bridge.NotePageLoad(strings.Repeat("a", 65), "about:blank")
	bridge.NotePageLoad("main", strings.Repeat("b", 4097))
	if strings.Join(order, ",") != "browser,ready,loop,exit,shutdown,page-load,page-load" {
		t.Fatalf("%v", order)
	}
	if strings.Join(second, ",") != "ready,exit,page-load,page-load" {
		t.Fatalf("%v", second)
	}
}

func TestLifecycleSkipsReadyWhenTheBrowserFails(t *testing.T) {
	var order []string
	app := New(conf.File{ProductName: "Hello"}, &orderHost{
		failBrowser: true,
		note:        func(step string) { order = append(order, step) },
	})
	app.SetBase(t.TempDir())
	app.OnLifecycle(func(ev Lifecycle) { order = append(order, ev.Event) })
	err := app.Run()
	if err == nil || !strings.Contains(err.Error(), "browser down") {
		t.Fatal(err)
	}
	if strings.Join(order, ",") != "shutdown" {
		t.Fatalf("%v", order)
	}
}

func TestAcceptPageLoad(t *testing.T) {
	if !acceptPageLoad("main", "about:blank") {
		t.Fatal("about:blank")
	}
	if acceptPageLoad("main", "") || acceptPageLoad("", "about:blank") || acceptPageLoad("a\x00b", "about:blank") {
		t.Fatal("accepted an empty or broken load")
	}
}

type orderHost struct {
	failBrowser bool
	note        func(string)
}

func (h *orderHost) ExecuteProcess() (int, bool) { return -1, false }

func (h *orderHost) InitAPIHash() error { return nil }

func (h *orderHost) Initialize(cef.Settings) error { return nil }

func (h *orderHost) CreateBrowser(string, cef.WindowBounds) error {
	if h.failBrowser {
		return fmt.Errorf("browser down")
	}
	h.note("browser")
	return nil
}

func (h *orderHost) RunLoop() { h.note("loop") }

func (h *orderHost) Shutdown() { h.note("shutdown") }
