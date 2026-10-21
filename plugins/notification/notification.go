// Package notification shows a desktop notification.
// The page supplies a title, an optional body, and an optional icon path.
// The icon must sit inside the configured directory scope. Text is passed
// as argv to notify-send, never through a shell.
package notification

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"sync"

	"github.com/zigslice/xgoef/ipc"
	"github.com/zigslice/xgoef/plugin"
	"github.com/zigslice/xgoef/plugins/fs"
)

const (
	maxTitle = 256
	maxBody  = 4096
)

// Note is one notification after validation.
type Note struct {
	AppName string
	Title   string
	Body    string
	Icon    string
}

// Sender displays one note. Tests replace notify-send.
type Sender func(Note) error

// Plugin implements plugin.Plugin.
type Plugin struct {
	appName string
	base    string
	scope   *fs.Scope

	mu      sync.Mutex
	granted bool
	send    Sender
	custom  bool
	probe   func() bool
}

// New builds the notification plugin. An empty allow list rejects every icon.
// send nil uses notify-send when a command runs. appName is the notify-send
// application name; an empty or unusable name becomes "xgoef".
func New(base, resource, appName string, allow, deny []string, send Sender) (*Plugin, error) {
	if base == "" {
		base = "."
	}
	scope, err := fs.NewScope(base, resource, allow, deny)
	if err != nil {
		return nil, err
	}
	p := &Plugin{
		appName: cleanAppName(appName),
		base:    base,
		scope:   scope,
		probe: func() bool {
			_, err := exec.LookPath("notify-send")
			return err == nil
		},
	}
	if send != nil {
		p.send = send
		p.custom = true
	}
	return p, nil
}

// SetSender replaces the display function. A nil sender restores notify-send.
func (p *Plugin) SetSender(send Sender) {
	if p == nil {
		return
	}
	p.mu.Lock()
	p.send = send
	p.custom = send != nil
	p.mu.Unlock()
}

// SetProbe replaces the check for a desktop notification service.
// Tests use it to force permission denied without looking at PATH.
func (p *Plugin) SetProbe(fn func() bool) {
	if p == nil {
		return
	}
	p.mu.Lock()
	p.probe = fn
	p.mu.Unlock()
}

// Name is the builtin plugin id.
func (p *Plugin) Name() string { return "notification" }

// Permissions are the capability ids an app must grant.
func (p *Plugin) Permissions() []string {
	return []string{
		"notification:is_permission_granted",
		"notification:request_permission",
		"notification:show",
	}
}

// Setup registers the notification commands.
func (p *Plugin) Setup(app plugin.Host) error {
	app.Register("notification:is_permission_granted", p.grantedCmd)
	app.Register("notification:request_permission", p.requestCmd)
	app.Register("notification:show", p.showCmd)
	return nil
}

// Teardown is a no-op.
func (p *Plugin) Teardown(plugin.Host) error { return nil }

func (p *Plugin) grantedCmd(_ ipc.Context, argsJSON []byte) (any, error) {
	if err := noArgs(argsJSON); err != nil {
		return nil, err
	}
	return p.isGranted(), nil
}

func (p *Plugin) requestCmd(_ ipc.Context, argsJSON []byte) (any, error) {
	if err := noArgs(argsJSON); err != nil {
		return nil, err
	}
	ok := p.available()
	p.mu.Lock()
	p.granted = ok
	p.mu.Unlock()
	if ok {
		return "granted", nil
	}
	return "denied", nil
}

func (p *Plugin) showCmd(_ ipc.Context, argsJSON []byte) (any, error) {
	note, err := p.prepare(argsJSON)
	if err != nil {
		return nil, err
	}
	p.mu.Lock()
	ok := p.granted
	send := p.send
	p.mu.Unlock()
	if !ok {
		return nil, ipc.Fail("notification.denied", "notification: permission is not granted")
	}
	if send == nil {
		send = defaultSend
	}
	if err := send(note); err != nil {
		return nil, ipc.Fail("notification.show", "notification: %s", err.Error())
	}
	return nil, nil
}

func (p *Plugin) isGranted() bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.granted
}

func (p *Plugin) available() bool {
	p.mu.Lock()
	custom := p.custom
	probe := p.probe
	p.mu.Unlock()
	if custom {
		return true
	}
	if probe == nil {
		return false
	}
	return probe()
}

func (p *Plugin) prepare(argsJSON []byte) (Note, error) {
	raw, err := objectArgs(argsJSON)
	if err != nil {
		return Note{}, invalid("notification: title is required")
	}
	for key := range raw {
		switch key {
		case "title", "body", "icon":
		default:
			return Note{}, invalid("notification: title is required")
		}
	}
	titleRaw, ok := raw["title"]
	if !ok {
		return Note{}, invalid("notification: title is required")
	}
	title, ok := jsonString(titleRaw)
	if !ok {
		return Note{}, invalid("notification: title is required")
	}
	title, err = cleanTitle(title)
	if err != nil {
		return Note{}, err
	}
	body := ""
	if part, ok := raw["body"]; ok {
		body, ok = jsonString(part)
		if !ok {
			return Note{}, invalid("notification: body is not valid text")
		}
		body, err = cleanBody(body)
		if err != nil {
			return Note{}, err
		}
	}
	icon := ""
	if part, ok := raw["icon"]; ok {
		icon, ok = jsonString(part)
		if !ok {
			return Note{}, invalid("notification: icon is not a path")
		}
		icon = strings.TrimSpace(icon)
		if icon != "" {
			abs, err := p.scope.Resolve(p.base, icon)
			if err != nil {
				return Note{}, ipc.Fail("notification.icon", "notification: icon %q is outside the configured scope", icon)
			}
			icon = abs
		}
	}
	return Note{AppName: p.appName, Title: title, Body: body, Icon: icon}, nil
}

func cleanAppName(name string) string {
	name = strings.TrimSpace(name)
	if name == "" || len(name) > 128 || strings.ContainsFunc(name, controlRune) {
		return "xgoef"
	}
	return name
}

func cleanTitle(s string) (string, error) {
	s = strings.TrimSpace(s)
	if s == "" || len(s) > maxTitle || strings.ContainsFunc(s, controlRune) {
		return "", invalid("notification: title must be a single line")
	}
	return s, nil
}

func cleanBody(s string) (string, error) {
	if len(s) > maxBody || strings.ContainsFunc(s, func(r rune) bool {
		return r != '\n' && r != '\t' && controlRune(r)
	}) {
		return "", invalid("notification: body is not valid text")
	}
	return s, nil
}

func controlRune(r rune) bool {
	return r < 0x20 || r == 0x7f
}

func noArgs(argsJSON []byte) error {
	raw, err := objectArgs(argsJSON)
	if err != nil || len(raw) != 0 {
		return invalid("notification: command takes no arguments")
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
	return ipc.Fail("notification.invalid", "%s", message)
}

// notifyArgs is the notify-send argv. "--" keeps a title that starts with "-"
// from being read as an option.
func notifyArgs(note Note) []string {
	app := note.AppName
	if app == "" {
		app = "xgoef"
	}
	args := []string{"-a", app}
	if note.Icon != "" {
		args = append(args, "-i", note.Icon)
	}
	args = append(args, "--", note.Title)
	if note.Body != "" {
		args = append(args, note.Body)
	}
	return args
}

func defaultSend(note Note) error {
	if _, err := exec.LookPath("notify-send"); err != nil {
		return fmt.Errorf("notify-send not found")
	}
	return exec.Command("notify-send", notifyArgs(note)...).Run()
}
