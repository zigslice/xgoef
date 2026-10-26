package runtime

import (
	"strings"
	"testing"

	"github.com/zigslice/xgoef/acl"
	"github.com/zigslice/xgoef/cef"
	"github.com/zigslice/xgoef/conf"
)

func TestWebViewFromConfig(t *testing.T) {
	app := New(conf.File{
		Plugins: []string{"webview"},
		Plugin:  conf.PluginSection{WebView: conf.WebViewConfig{DevTools: true}},
		Capabilities: []acl.Capability{{
			Identifier: "main",
			Windows:    []string{"main"},
			Permissions: []string{
				"webview:print",
				"webview:open_dev_tools",
				"webview:close_dev_tools",
				"webview:is_dev_tools_open",
			},
		}},
	}, nil)
	if err := app.OpenPlugins(); err != nil {
		t.Fatal(err)
	}
	if err := app.Plugins.Setup(app); err != nil {
		t.Fatal(err)
	}
	printed := handleMessage(app.ACL, app.Router, nil, "main",
		`{"type":"invoke","id":1,"cmd":"webview:print","args":{}}`)
	if !strings.Contains(printed, `"ok":true`) {
		t.Fatalf("%s", printed)
	}
	bridge := app.Bridge.(*cef.NativeBridge)
	if label, ok := bridge.LastPrint(); !ok || label != "main" {
		t.Fatalf("print %q %v", label, ok)
	}
	opened := handleMessage(app.ACL, app.Router, nil, "main",
		`{"type":"invoke","id":2,"cmd":"webview:open_dev_tools","args":{}}`)
	if !strings.Contains(opened, `"ok":true`) {
		t.Fatalf("%s", opened)
	}
	action, label, open := bridge.LastDevTools()
	if action != "open" || label != "main" || !open {
		t.Fatalf("%s %s %v", action, label, open)
	}
	isOpen := handleMessage(app.ACL, app.Router, nil, "main",
		`{"type":"invoke","id":3,"cmd":"webview:is_dev_tools_open","args":{}}`)
	if !strings.Contains(isOpen, `"payload":true`) {
		t.Fatalf("%s", isOpen)
	}
	closed := handleMessage(app.ACL, app.Router, nil, "main",
		`{"type":"invoke","id":4,"cmd":"webview:close_dev_tools","args":{}}`)
	if !strings.Contains(closed, `"ok":true`) {
		t.Fatalf("%s", closed)
	}
	action, _, open = bridge.LastDevTools()
	if action != "close" || open {
		t.Fatalf("%s %v", action, open)
	}
	isOpen = handleMessage(app.ACL, app.Router, nil, "main",
		`{"type":"invoke","id":5,"cmd":"webview:is_dev_tools_open","args":{}}`)
	if !strings.Contains(isOpen, `"payload":false`) {
		t.Fatalf("%s", isOpen)
	}
	denied := handleMessage(app.ACL, app.Router, nil, "aux",
		`{"type":"invoke","id":6,"cmd":"webview:print","args":{}}`)
	if !strings.Contains(denied, `"code":"acl.denied"`) {
		t.Fatalf("%s", denied)
	}
}

func TestWebViewDevToolsDisabled(t *testing.T) {
	app := New(conf.File{
		Plugins: []string{"webview"},
		Capabilities: []acl.Capability{{
			Identifier: "main",
			Windows:    []string{"main"},
			Permissions: []string{
				"webview:open_dev_tools",
				"webview:is_dev_tools_open",
			},
		}},
	}, nil)
	if err := app.OpenPlugins(); err != nil {
		t.Fatal(err)
	}
	if err := app.Plugins.Setup(app); err != nil {
		t.Fatal(err)
	}
	denied := handleMessage(app.ACL, app.Router, nil, "main",
		`{"type":"invoke","id":1,"cmd":"webview:open_dev_tools","args":{}}`)
	if !strings.Contains(denied, `"code":"webview.denied"`) {
		t.Fatalf("%s", denied)
	}
	bridge := app.Bridge.(*cef.NativeBridge)
	if action, _, _ := bridge.LastDevTools(); action != "" {
		t.Fatalf("bridge action %q", action)
	}
}
