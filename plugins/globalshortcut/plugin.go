// Package globalshortcut registers a small set of global accelerators.
// The page names a shortcut the config allow list permits. Linux installs
// the grab through X11. Presses and releases are delivered as the
// global-shortcut event.
package globalshortcut

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"

	"github.com/zigslice/xgoef/ipc"
	"github.com/zigslice/xgoef/plugin"
)

const (
	// Event is the host event emitted for a press or a release.
	Event = "global-shortcut"
	// Pressed is Shortcut state when the accelerator goes down.
	Pressed = "Pressed"
	// Released is Shortcut state when the accelerator goes up.
	Released = "Released"
)

// Plugin implements plugin.Plugin.
type Plugin struct {
	mu      sync.Mutex
	allow   map[string]Shortcut
	reg     map[string]Shortcut
	emit    func(name string, payload any) error
	session Session
	custom  bool
}

// New builds the plugin. allow is the only set of accelerators the page
// may register; an empty list rejects every register. emit nil drops the
// events. The grabber connects on the first register.
func New(allow []string, emit func(name string, payload any) error) (*Plugin, error) {
	set := make(map[string]Shortcut, len(allow))
	for _, raw := range allow {
		sc, err := Parse(raw)
		if err != nil {
			return nil, fmt.Errorf("global-shortcut: allow %q: %w", raw, err)
		}
		set[sc.String()] = sc
	}
	return &Plugin{
		allow: set,
		reg:   map[string]Shortcut{},
		emit:  emit,
	}, nil
}

// SetSession replaces the grabber. Tests use it. nil makes the next
// register fail with shortcut.unavailable instead of opening a display.
func (p *Plugin) SetSession(s Session) {
	if p == nil {
		return
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.session != nil && p.session != s {
		_ = p.session.Close()
	}
	p.session = s
	p.custom = true
	if b, ok := s.(hookBinder); ok {
		b.Bind(p.onEvent)
	}
}

// Name implements plugin.Plugin.
func (p *Plugin) Name() string { return "global-shortcut" }

// Permissions implements plugin.Plugin.
func (p *Plugin) Permissions() []string {
	return []string{
		"global-shortcut:register",
		"global-shortcut:unregister",
		"global-shortcut:unregister_all",
		"global-shortcut:is_registered",
	}
}

// Setup registers the shortcut commands.
func (p *Plugin) Setup(app plugin.Host) error {
	app.Register("global-shortcut:register", p.registerCmd)
	app.Register("global-shortcut:unregister", p.unregisterCmd)
	app.Register("global-shortcut:unregister_all", p.unregisterAllCmd)
	app.Register("global-shortcut:is_registered", p.isRegisteredCmd)
	return nil
}

// Teardown releases every grab and closes the grabber.
func (p *Plugin) Teardown(plugin.Host) error {
	err := p.unregisterAll()
	p.mu.Lock()
	s := p.session
	p.session = nil
	p.mu.Unlock()
	if s != nil {
		if closeErr := s.Close(); err == nil {
			err = closeErr
		}
	}
	return err
}

func (p *Plugin) registerCmd(_ ipc.Context, argsJSON []byte) (any, error) {
	sc, err := shortcutArg(argsJSON)
	if err != nil {
		return nil, err
	}
	if _, ok := p.allow[sc.String()]; !ok {
		return nil, ipc.Fail("shortcut.denied", "shortcut: %s is not allowed", sc.String())
	}
	session, err := p.ensureSession()
	if err != nil {
		return nil, grabErr(err)
	}
	p.mu.Lock()
	_, exists := p.reg[sc.String()]
	p.mu.Unlock()
	if exists {
		return nil, ipc.Fail("shortcut.registered", "shortcut: %s is already registered", sc.String())
	}
	if err := session.Grab(sc); err != nil {
		return nil, grabErr(err)
	}
	p.mu.Lock()
	if _, exists := p.reg[sc.String()]; exists {
		p.mu.Unlock()
		_ = session.Ungrab(sc)
		return nil, ipc.Fail("shortcut.registered", "shortcut: %s is already registered", sc.String())
	}
	p.reg[sc.String()] = sc
	p.mu.Unlock()
	return nil, nil
}

func (p *Plugin) unregisterCmd(_ ipc.Context, argsJSON []byte) (any, error) {
	sc, err := shortcutArg(argsJSON)
	if err != nil {
		return nil, err
	}
	p.mu.Lock()
	_, exists := p.reg[sc.String()]
	session := p.session
	if exists {
		delete(p.reg, sc.String())
	}
	p.mu.Unlock()
	if !exists || session == nil {
		return nil, nil
	}
	if err := session.Ungrab(sc); err != nil {
		p.mu.Lock()
		p.reg[sc.String()] = sc
		p.mu.Unlock()
		return nil, grabErr(err)
	}
	return nil, nil
}

func (p *Plugin) unregisterAllCmd(_ ipc.Context, argsJSON []byte) (any, error) {
	if err := noArgs(argsJSON); err != nil {
		return nil, err
	}
	if err := p.unregisterAll(); err != nil {
		return nil, grabErr(err)
	}
	return nil, nil
}

func (p *Plugin) isRegisteredCmd(_ ipc.Context, argsJSON []byte) (any, error) {
	sc, err := shortcutArg(argsJSON)
	if err != nil {
		return nil, err
	}
	p.mu.Lock()
	_, ok := p.reg[sc.String()]
	p.mu.Unlock()
	return ok, nil
}

func (p *Plugin) unregisterAll() error {
	p.mu.Lock()
	list := make([]Shortcut, 0, len(p.reg))
	for _, sc := range p.reg {
		list = append(list, sc)
	}
	session := p.session
	p.mu.Unlock()
	failed := map[string]Shortcut{}
	var first error
	for _, sc := range list {
		if session == nil {
			continue
		}
		if err := session.Ungrab(sc); err != nil {
			if first == nil {
				first = err
			}
			failed[sc.String()] = sc
		}
	}
	p.mu.Lock()
	p.reg = failed
	p.mu.Unlock()
	return first
}

func (p *Plugin) ensureSession() (Session, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.session != nil {
		return p.session, nil
	}
	if p.custom {
		return nil, errUnavailable
	}
	s, err := openSession(p.onEvent)
	if err != nil {
		return nil, err
	}
	p.session = s
	return s, nil
}

func (p *Plugin) onEvent(sc Shortcut, state string) {
	if state != Pressed && state != Released {
		return
	}
	p.mu.Lock()
	_, ok := p.reg[sc.String()]
	emit := p.emit
	p.mu.Unlock()
	if !ok || emit == nil {
		return
	}
	_ = emit(Event, map[string]string{
		"shortcut": sc.String(),
		"state":    state,
	})
}

type hookBinder interface {
	Bind(func(Shortcut, string))
}

func grabErr(err error) error {
	if errors.Is(err, errUnavailable) {
		return ipc.Fail("shortcut.unavailable", "shortcut: global shortcut is not available")
	}
	return ipc.Fail("shortcut.grab", "shortcut: %s", err.Error())
}

func shortcutArg(argsJSON []byte) (Shortcut, error) {
	raw, err := objectArgs(argsJSON)
	if err != nil {
		return Shortcut{}, invalid("shortcut: shortcut is required")
	}
	for key := range raw {
		if key != "shortcut" {
			return Shortcut{}, invalid("shortcut: shortcut is required")
		}
	}
	part, ok := raw["shortcut"]
	if !ok {
		return Shortcut{}, invalid("shortcut: shortcut is required")
	}
	text, ok := jsonString(part)
	if !ok || strings.TrimSpace(text) == "" {
		return Shortcut{}, invalid("shortcut: shortcut is required")
	}
	sc, err := Parse(text)
	if err != nil {
		return Shortcut{}, invalid("shortcut: shortcut is not valid")
	}
	return sc, nil
}

func noArgs(argsJSON []byte) error {
	raw, err := objectArgs(argsJSON)
	if err != nil || len(raw) != 0 {
		return invalid("shortcut: command takes no arguments")
	}
	return nil
}

func objectArgs(argsJSON []byte) (map[string]json.RawMessage, error) {
	trimmed := bytes.TrimSpace(argsJSON)
	if len(trimmed) == 0 || string(trimmed) == "null" {
		return map[string]json.RawMessage{}, nil
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(trimmed, &raw); err != nil || raw == nil {
		return nil, err
	}
	return raw, nil
}

func jsonString(raw json.RawMessage) (string, bool) {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 || string(trimmed) == "null" {
		return "", true
	}
	var s string
	if err := json.Unmarshal(trimmed, &s); err != nil {
		return "", false
	}
	return s, true
}

func invalid(message string) error {
	return ipc.Fail("shortcut.invalid", "%s", message)
}
