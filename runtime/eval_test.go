package runtime

import (
	"errors"
	"strings"
	"testing"

	"github.com/zigslice/xgoef/acl"
	"github.com/zigslice/xgoef/cef"
	"github.com/zigslice/xgoef/conf"
	"github.com/zigslice/xgoef/eval"
)

func TestEvalFromConfig(t *testing.T) {
	const source = `window.__xgoefMark = "marked";`
	const init = `window.__xgoefInjected = "hello";`
	app := New(conf.File{
		App: conf.AppSection{Windows: []conf.Window{{
			Label:       "main",
			Title:       "t",
			Width:       100,
			Height:      100,
			URL:         "https://example.test/index.html",
			InitScripts: []string{init},
		}}},
		Plugins: []string{"eval"},
		Plugin: conf.PluginSection{Eval: conf.EvalConfig{Scripts: map[string]string{
			"mark": source,
		}}},
		Capabilities: []acl.Capability{{
			Identifier:  "main",
			Windows:     []string{"main"},
			Permissions: []string{"eval:run"},
		}},
	}, nil)
	if err := app.OpenPlugins(); err != nil {
		t.Fatal(err)
	}
	if err := app.Plugins.Setup(app); err != nil {
		t.Fatal(err)
	}
	bridge := app.Bridge.(*cef.NativeBridge)
	label, allowed, encoded, ok := bridge.LastInit()
	if !ok || label != "main" || allowed != "https://example.test/index.html" || !strings.Contains(encoded, init) {
		t.Fatalf("init %s %s %s %v", label, allowed, encoded, ok)
	}
	ran := handleMessage(app.ACL, app.Router, nil, "main",
		`{"type":"invoke","id":1,"cmd":"eval:run","args":{"name":"mark"}}`)
	if !strings.Contains(ran, `"ok":true`) {
		t.Fatalf("%s", ran)
	}
	el, ea, es, eok := bridge.LastEval()
	if !eok || el != "main" || ea != allowed || es != source {
		t.Fatalf("eval %s %s %s %v", el, ea, es, eok)
	}
	denied := handleMessage(app.ACL, app.Router, nil, "main",
		`{"type":"invoke","id":2,"cmd":"eval:run","args":{"name":"mark","script":"alert(1)"}}`)
	if !strings.Contains(denied, `"code":"eval.denied"`) {
		t.Fatalf("%s", denied)
	}
	if _, _, next, _ := bridge.LastEval(); next != source {
		t.Fatalf("page source ran: %s", next)
	}
	aux := handleMessage(app.ACL, app.Router, nil, "aux",
		`{"type":"invoke","id":3,"cmd":"eval:run","args":{"name":"mark"}}`)
	if !strings.Contains(aux, `"code":"acl.denied"`) {
		t.Fatalf("%s", aux)
	}
	if err := app.Eval("other", "window.ok = 1;"); !errors.Is(err, eval.ErrUnpinned) {
		t.Fatalf("unpinned %v", err)
	}
	if err := app.Eval("main", "ok\x00"); !errors.Is(err, eval.ErrRejected) {
		t.Fatalf("nul %v", err)
	}
	if _, _, next, _ := bridge.LastEval(); next != source {
		t.Fatal("rejected source was sent")
	}
	if err := app.CreateWindow(cef.WindowSpec{
		Label:       "side",
		URL:         "https://side.test/a",
		Width:       10,
		Height:      10,
		InitScripts: []string{`window.side = 1;`},
	}); err != nil {
		t.Fatal(err)
	}
	label, allowed, encoded, ok = bridge.LastInit()
	if !ok || label != "side" || allowed != "https://side.test/a" || !strings.Contains(encoded, "window.side = 1;") {
		t.Fatalf("side %s %s %s %v", label, allowed, encoded, ok)
	}
	if err := app.Eval("side", "window.side = 2;"); err != nil {
		t.Fatal(err)
	}
	el, ea, es, eok = bridge.LastEval()
	if !eok || el != "side" || ea != "https://side.test/a" || es != "window.side = 2;" {
		t.Fatalf("side eval %s %s %s %v", el, ea, es, eok)
	}
	err := app.CreateWindow(cef.WindowSpec{
		Label:       "blank",
		URL:         "about:blank",
		Width:       10,
		Height:      10,
		InitScripts: []string{"window.blank = 1;"},
	})
	if !errors.Is(err, eval.ErrOrigin) {
		t.Fatalf("about %v", err)
	}
	if label, _, _, _ := bridge.LastInit(); label != "side" {
		t.Fatalf("about stored init on %s", label)
	}
}
