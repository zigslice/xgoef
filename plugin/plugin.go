// Package plugin is the Tauri-like plugin contract: lifecycle plus commands.
package plugin

import (
	"fmt"

	"github.com/zigslice/xgoef/ipc"
)

// Host is the runtime surface a plugin uses to register commands.
// *runtime.App implements Host.
type Host interface {
	Register(name string, h ipc.Handler)
}

// Plugin is one native plugin. Setup registers commands; Teardown releases them.
// Permissions lists the capability ids the application must grant (usually the
// command names, such as "fs:read_text").
type Plugin interface {
	Name() string
	Permissions() []string
	Setup(app Host) error
	Teardown(app Host) error
}

// Registry holds plugins in registration order.
type Registry struct {
	list []Plugin
}

// NewRegistry creates an empty plugin registry.
func NewRegistry() *Registry {
	return &Registry{}
}

// Has reports whether a plugin with this name is already registered.
func (r *Registry) Has(name string) bool {
	if r == nil {
		return false
	}
	for _, p := range r.list {
		if p.Name() == name {
			return true
		}
	}
	return false
}

// Add appends a plugin. A nil plugin or a duplicate name is an error.
func (r *Registry) Add(p Plugin) error {
	if p == nil {
		return fmt.Errorf("plugin: nil")
	}
	if r.Has(p.Name()) {
		return fmt.Errorf("plugin: %s already registered", p.Name())
	}
	r.list = append(r.list, p)
	return nil
}

// Plugins returns the registered plugins in order.
func (r *Registry) Plugins() []Plugin {
	if r == nil {
		return nil
	}
	out := make([]Plugin, len(r.list))
	copy(out, r.list)
	return out
}

// Setup runs all plugins.
func (r *Registry) Setup(app Host) error {
	for _, p := range r.list {
		if err := p.Setup(app); err != nil {
			return fmt.Errorf("plugin %s: %w", p.Name(), err)
		}
	}
	return nil
}

// Teardown runs plugins in reverse order.
func (r *Registry) Teardown(app Host) {
	for i := len(r.list) - 1; i >= 0; i-- {
		_ = r.list[i].Teardown(app)
	}
}
