// Package clipboard is a text clipboard. The default store stays inside the
// process. Mode "system" uses wl-copy/wl-paste or xclip when those tools exist.
package clipboard

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"sync"

	"github.com/zigslice/xgoef/ipc"
	"github.com/zigslice/xgoef/plugin"
)

// MaxBytes is the text cap, matching the fs plugin.
const MaxBytes = 1 << 20

// Store reads and writes clipboard text.
type Store interface {
	Read() (string, error)
	Write(text string) error
}

// Memory is a process-local clipboard.
type Memory struct {
	mu   sync.Mutex
	text string
}

// Read returns the stored text.
func (m *Memory) Read() (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.text, nil
}

// Write replaces the stored text.
func (m *Memory) Write(text string) error {
	if len(text) > MaxBytes {
		return fmt.Errorf("clipboard: text exceeds %d bytes", MaxBytes)
	}
	m.mu.Lock()
	m.text = text
	m.mu.Unlock()
	return nil
}

// Plugin implements plugin.Plugin.
type Plugin struct {
	store Store
}

// New uses store, or a memory clipboard when store is nil.
func New(store Store) *Plugin {
	if store == nil {
		store = &Memory{}
	}
	return &Plugin{store: store}
}

// NewSystem returns a clipboard backed by wl-clipboard or xclip.
func NewSystem() (*Plugin, error) {
	if _, err := exec.LookPath("wl-copy"); err == nil {
		if _, err := exec.LookPath("wl-paste"); err == nil {
			return New(commandStore{copy: []string{"wl-copy"}, paste: []string{"wl-paste", "-n"}}), nil
		}
	}
	if _, err := exec.LookPath("xclip"); err == nil {
		return New(commandStore{
			copy:  []string{"xclip", "-selection", "clipboard"},
			paste: []string{"xclip", "-selection", "clipboard", "-o"},
		}), nil
	}
	return nil, fmt.Errorf("clipboard: system clipboard needs wl-copy/wl-paste or xclip")
}

// Name is the builtin plugin id.
func (p *Plugin) Name() string { return "clipboard" }

// Permissions are the capability ids an app must grant.
func (p *Plugin) Permissions() []string {
	return []string{"clipboard:read_text", "clipboard:write_text"}
}

// Setup registers the clipboard commands.
func (p *Plugin) Setup(app plugin.Host) error {
	app.Register("clipboard:read_text", p.read)
	app.Register("clipboard:write_text", p.write)
	return nil
}

// Teardown is a no-op.
func (p *Plugin) Teardown(plugin.Host) error { return nil }

func (p *Plugin) read(ipc.Context, []byte) (any, error) {
	text, err := p.store.Read()
	if err != nil {
		return nil, fmt.Errorf("clipboard: %w", err)
	}
	return text, nil
}

func (p *Plugin) write(_ ipc.Context, argsJSON []byte) (any, error) {
	var args struct {
		Text string `json:"text"`
	}
	if err := json.Unmarshal(argsJSON, &args); err != nil {
		return nil, fmt.Errorf("clipboard: %w", err)
	}
	if err := p.store.Write(args.Text); err != nil {
		return nil, err
	}
	return nil, nil
}

type commandStore struct {
	copy  []string
	paste []string
}

func (c commandStore) Write(text string) error {
	if len(text) > MaxBytes {
		return fmt.Errorf("clipboard: text exceeds %d bytes", MaxBytes)
	}
	cmd := exec.Command(c.copy[0], c.copy[1:]...)
	cmd.Stdin = strings.NewReader(text)
	return cmd.Run()
}

func (c commandStore) Read() (string, error) {
	cmd := exec.Command(c.paste[0], c.paste[1:]...)
	var buf bytes.Buffer
	cmd.Stdout = &buf
	if err := cmd.Run(); err != nil {
		return "", err
	}
	return buf.String(), nil
}
