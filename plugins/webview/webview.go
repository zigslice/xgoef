// Package webview prints the calling window and opens its developer tools.
// Print shows the system print dialog. Developer tools also require
// plugin.webview.devtools; the zero value is off.
package webview

import (
	"github.com/zigslice/xgoef/ipc"
	"github.com/zigslice/xgoef/plugin"
)

// Printer opens the system print dialog for a window label.
type Printer func(window string) error

// DevToolsOp opens or closes developer tools for a window label.
type DevToolsOp func(window string) error

// DevToolsQuery reports whether developer tools are open.
type DevToolsQuery func(window string) (bool, error)

// Plugin implements plugin.Plugin.
type Plugin struct {
	devtools bool
	print    Printer
	open     DevToolsOp
	close    DevToolsOp
	query    DevToolsQuery
}

// New returns the webview builtin. devtools is plugin.webview.devtools.
// A nil print, open, close, or query function fails that command with
// webview.unavailable. When devtools is false, open, close, and is return
// webview.denied and do not call the functions.
func New(devtools bool, print Printer, open, close DevToolsOp, query DevToolsQuery) *Plugin {
	return &Plugin{
		devtools: devtools,
		print:    print,
		open:     open,
		close:    close,
		query:    query,
	}
}

// Name is the builtin plugin id.
func (p *Plugin) Name() string { return "webview" }

// Permissions are the capability ids an app must grant.
func (p *Plugin) Permissions() []string {
	return []string{
		"webview:print",
		"webview:open_dev_tools",
		"webview:close_dev_tools",
		"webview:is_dev_tools_open",
	}
}

// Setup registers the webview commands.
func (p *Plugin) Setup(app plugin.Host) error {
	app.Register("webview:print", p.doPrint)
	app.Register("webview:open_dev_tools", p.doOpen)
	app.Register("webview:close_dev_tools", p.doClose)
	app.Register("webview:is_dev_tools_open", p.doIsOpen)
	return nil
}

// Teardown is a no-op.
func (p *Plugin) Teardown(plugin.Host) error { return nil }

func (p *Plugin) doPrint(ctx ipc.Context, _ []byte) (any, error) {
	if p.print == nil {
		return nil, ipc.Fail("webview.unavailable", "webview: cannot print")
	}
	if err := p.print(windowOf(ctx.Window)); err != nil {
		return nil, ipc.Fail("webview.unavailable", "%s", err.Error())
	}
	return nil, nil
}

func (p *Plugin) doOpen(ctx ipc.Context, _ []byte) (any, error) {
	if !p.devtools {
		return nil, ipc.Fail("webview.denied", "webview: devtools are disabled")
	}
	if p.open == nil {
		return nil, ipc.Fail("webview.unavailable", "webview: cannot open devtools")
	}
	if err := p.open(windowOf(ctx.Window)); err != nil {
		return nil, ipc.Fail("webview.unavailable", "%s", err.Error())
	}
	return nil, nil
}

func (p *Plugin) doClose(ctx ipc.Context, _ []byte) (any, error) {
	if !p.devtools {
		return nil, ipc.Fail("webview.denied", "webview: devtools are disabled")
	}
	if p.close == nil {
		return nil, ipc.Fail("webview.unavailable", "webview: cannot close devtools")
	}
	if err := p.close(windowOf(ctx.Window)); err != nil {
		return nil, ipc.Fail("webview.unavailable", "%s", err.Error())
	}
	return nil, nil
}

func (p *Plugin) doIsOpen(ctx ipc.Context, _ []byte) (any, error) {
	if !p.devtools {
		return nil, ipc.Fail("webview.denied", "webview: devtools are disabled")
	}
	if p.query == nil {
		return nil, ipc.Fail("webview.unavailable", "webview: cannot query devtools")
	}
	open, err := p.query(windowOf(ctx.Window))
	if err != nil {
		return nil, ipc.Fail("webview.unavailable", "%s", err.Error())
	}
	return open, nil
}

func windowOf(label string) string {
	if label == "" {
		return "main"
	}
	return label
}
