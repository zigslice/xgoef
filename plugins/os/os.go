// Package osinfo exposes host platform facts as IPC commands os:platform and os:arch.
package osinfo

import (
	"runtime"

	"github.com/zigslice/xgoef/ipc"
	"github.com/zigslice/xgoef/plugin"
)

// Plugin implements plugin.Plugin.
type Plugin struct{}

// New returns the os builtin.
func New() *Plugin { return &Plugin{} }

// Name is the builtin plugin id.
func (p *Plugin) Name() string { return "os" }

// Permissions are the capability ids an app must grant.
func (p *Plugin) Permissions() []string {
	return []string{"os:platform", "os:arch"}
}

// Setup registers the os commands.
func (p *Plugin) Setup(app plugin.Host) error {
	app.Register("os:platform", func(ipc.Context, []byte) (any, error) {
		return runtime.GOOS, nil
	})
	app.Register("os:arch", func(ipc.Context, []byte) (any, error) {
		return runtime.GOARCH, nil
	})
	return nil
}

// Teardown is a no-op.
func (p *Plugin) Teardown(plugin.Host) error { return nil }
