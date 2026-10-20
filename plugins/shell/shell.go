// Package shell opens http(s) URLs or files inside a directory scope.
// It never invokes a shell; the opener receives one argument.
package shell

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os/exec"
	"strings"

	"github.com/zigslice/xgoef/ipc"
	"github.com/zigslice/xgoef/plugin"
	"github.com/zigslice/xgoef/plugins/fs"
)

// Opener launches one target (a URL or an absolute path).
type Opener func(target string) error

// Plugin implements plugin.Plugin.
type Plugin struct {
	base    string
	scope   *fs.Scope
	schemes map[string]bool
	open    Opener
}

// New builds the shell plugin. schemes empty means http and https.
// An empty allow list rejects every local path; URLs are still checked
// against schemes.
func New(base, resource string, schemes, allow, deny []string) (*Plugin, error) {
	if base == "" {
		base = "."
	}
	set := map[string]bool{}
	if len(schemes) == 0 {
		schemes = []string{"https", "http"}
	}
	for _, s := range schemes {
		s = strings.ToLower(strings.TrimSpace(s))
		if s == "" {
			continue
		}
		if s == "file" {
			return nil, fmt.Errorf("shell: use path scope instead of the file scheme")
		}
		set[s] = true
	}
	if len(set) == 0 {
		return nil, fmt.Errorf("shell: no URL schemes allowed")
	}
	var scope *fs.Scope
	if len(allow) > 0 {
		sc, err := fs.NewScope(base, resource, allow, deny)
		if err != nil {
			return nil, err
		}
		scope = sc
	}
	return &Plugin{base: base, scope: scope, schemes: set, open: xdgOpen}, nil
}

// SetOpener replaces the process launcher. Tests use this.
func (p *Plugin) SetOpener(open Opener) {
	if open == nil {
		p.open = xdgOpen
		return
	}
	p.open = open
}

// Name is the builtin plugin id.
func (p *Plugin) Name() string { return "shell" }

// Permissions are the capability ids an app must grant.
func (p *Plugin) Permissions() []string { return []string{"shell:open"} }

// Setup registers shell:open.
func (p *Plugin) Setup(app plugin.Host) error {
	app.Register("shell:open", p.openCmd)
	return nil
}

// Teardown is a no-op.
func (p *Plugin) Teardown(plugin.Host) error { return nil }

type openArgs struct {
	URL  string `json:"url"`
	Path string `json:"path"`
}

func (p *Plugin) openCmd(_ ipc.Context, argsJSON []byte) (any, error) {
	var args openArgs
	if err := json.Unmarshal(argsJSON, &args); err != nil {
		return nil, fmt.Errorf("shell: %w", err)
	}
	if strings.TrimSpace(args.URL) != "" && strings.TrimSpace(args.Path) != "" {
		return nil, fmt.Errorf("shell: pass url or path, not both")
	}
	var target string
	var err error
	switch {
	case strings.TrimSpace(args.URL) != "":
		target, err = p.checkURL(args.URL)
	case strings.TrimSpace(args.Path) != "":
		target, err = p.checkPath(args.Path)
	default:
		err = fmt.Errorf("shell: url or path is required")
	}
	if err != nil {
		return nil, err
	}
	if err := p.open(target); err != nil {
		return nil, fmt.Errorf("shell: %w", err)
	}
	return target, nil
}

func (p *Plugin) checkURL(raw string) (string, error) {
	if strings.ContainsAny(raw, "\r\n") {
		return "", fmt.Errorf("shell: url %q is not allowed", raw)
	}
	u, err := url.Parse(raw)
	if err != nil {
		return "", fmt.Errorf("shell: %w", err)
	}
	scheme := strings.ToLower(u.Scheme)
	if !u.IsAbs() || scheme == "" || !p.schemes[scheme] {
		return "", fmt.Errorf("shell: url %q is not allowed", raw)
	}
	if scheme == "http" || scheme == "https" {
		if u.Host == "" || u.User != nil {
			return "", fmt.Errorf("shell: url %q is not allowed", raw)
		}
	}
	return u.String(), nil
}

func (p *Plugin) checkPath(userPath string) (string, error) {
	if p.scope == nil {
		return "", fmt.Errorf("shell: path %q is outside the configured scope", userPath)
	}
	abs, err := p.scope.Resolve(p.base, userPath)
	if err != nil {
		if strings.Contains(err.Error(), "outside") || strings.Contains(err.Error(), "not configured") {
			return "", fmt.Errorf("shell: path %q is outside the configured scope", userPath)
		}
		return "", fmt.Errorf("shell: %w", err)
	}
	return abs, nil
}

func xdgOpen(target string) error {
	return exec.Command("xdg-open", target).Run()
}
