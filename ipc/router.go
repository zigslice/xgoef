package ipc

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
)

// ErrCanceled is returned when the page cancels an invoke, or the frame drops it.
var ErrCanceled = errors.New("ipc: canceled")

// ErrTimeout is returned when an invoke exceeds its deadline.
var ErrTimeout = errors.New("ipc: timeout")

// ErrInvalidTimeout is returned when the page asks for a timeout this host rejects.
var ErrInvalidTimeout = errors.New("ipc: invalid timeout")

// Context is passed to command handlers.
// The embedded context is canceled when the page cancels the invoke or the
// deadline passes. Invoke and menu activation use a context that does not end.
type Context struct {
	context.Context
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
	return r.InvokeContext(context.Background(), window, name, argsJSON)
}

// InvokeContext runs a command with a context the handler can observe.
// A canceled or expired context is reported as ErrCanceled or ErrTimeout
// when the handler returns that context error.
func (r *Router) InvokeContext(ctx context.Context, window, name string, argsJSON []byte) ([]byte, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	r.mu.RLock()
	h, ok := r.handlers[name]
	hub := r.channels
	r.mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("ipc: unknown command %q", name)
	}
	out, err := h(Context{Context: ctx, Command: name, Window: window, channels: hub}, argsJSON)
	if err != nil {
		return nil, normalizeErr(err)
	}
	if out == nil {
		return []byte("null"), nil
	}
	return json.Marshal(out)
}

func normalizeErr(err error) error {
	if errors.Is(err, context.DeadlineExceeded) {
		return ErrTimeout
	}
	if errors.Is(err, context.Canceled) {
		return ErrCanceled
	}
	return err
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
