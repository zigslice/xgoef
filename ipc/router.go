package ipc

import (
	"encoding/json"
	"fmt"
	"sync"
)

// Context is passed to command handlers.
type Context struct {
	Command  string
	Window   string
	channels *Hub
}

// Handler is a Tauri-like command implementation.
// argsJSON is the raw JSON object/array from the frontend.
// Return value is JSON-encoded for the response payload.
type Handler func(ctx Context, argsJSON []byte) (any, error)

// Router dispatches invoke(command) to registered handlers.
type Router struct {
	mu       sync.RWMutex
	handlers map[string]Handler
	channels *Hub
}

// NewRouter creates an empty command router.
func NewRouter() *Router {
	return &Router{handlers: make(map[string]Handler)}
}

// SetChannels attaches the hub Context.Channel will bind against.
func (r *Router) SetChannels(h *Hub) {
	r.mu.Lock()
	r.channels = h
	r.mu.Unlock()
}

// Channels returns the hub set by SetChannels.
func (r *Router) Channels() *Hub {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.channels
}

// Handle registers or replaces a command.
func (r *Router) Handle(name string, h Handler) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.handlers[name] = h
}

// Invoke runs a command by name (used by the CEF message bridge).
func (r *Router) Invoke(name string, argsJSON []byte) ([]byte, error) {
	return r.InvokeFrom("", name, argsJSON)
}

// InvokeFrom runs a command and records the calling window label.
func (r *Router) InvokeFrom(window, name string, argsJSON []byte) ([]byte, error) {
	r.mu.RLock()
	h, ok := r.handlers[name]
	hub := r.channels
	r.mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("ipc: unknown command %q", name)
	}
	out, err := h(Context{Command: name, Window: window, channels: hub}, argsJSON)
	if err != nil {
		return nil, err
	}
	if out == nil {
		return []byte("null"), nil
	}
	return json.Marshal(out)
}

// Commands returns registered command names (for capabilities checks).
func (r *Router) Commands() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	names := make([]string, 0, len(r.handlers))
	for n := range r.handlers {
		names = append(names, n)
	}
	return names
}
