// Package fs is a scoped filesystem plugin (fs:read_text, fs:write_text,
// fs:exists, fs:list). Paths outside the allow list are rejected.
package fs

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"

	"github.com/zigslice/xgoef/ipc"
	"github.com/zigslice/xgoef/plugin"
)

// MaxBytes is the read/write cap for text commands.
const MaxBytes = 1 << 20

// Plugin implements plugin.Plugin.
type Plugin struct {
	base  string
	scope *Scope
}

// New builds an fs plugin. base is the app directory used for relative paths
// and for $APP. resource is the frontend dist directory ($RESOURCE).
// An empty allow list denies every path.
func New(base, resource string, allow, deny []string) (*Plugin, error) {
	if base == "" {
		base = "."
	}
	absBase, err := filepath.Abs(base)
	if err != nil {
		return nil, err
	}
	if resource == "" {
		resource = filepath.Join(absBase, "web")
	}
	if !filepath.IsAbs(resource) {
		resource = filepath.Join(absBase, resource)
	}
	scope, err := NewScope(absBase, resource, allow, deny)
	if err != nil {
		return nil, err
	}
	return &Plugin{base: absBase, scope: scope}, nil
}

// Name is the builtin plugin id.
func (p *Plugin) Name() string { return "fs" }

// Permissions are the capability ids an app must grant.
func (p *Plugin) Permissions() []string {
	return []string{"fs:read_text", "fs:write_text", "fs:exists", "fs:list"}
}

// Setup registers the fs commands.
func (p *Plugin) Setup(app plugin.Host) error {
	app.Register("fs:read_text", p.readText)
	app.Register("fs:write_text", p.writeText)
	app.Register("fs:exists", p.exists)
	app.Register("fs:list", p.list)
	return nil
}

// Teardown is a no-op; the plugin holds no OS resources.
func (p *Plugin) Teardown(plugin.Host) error { return nil }

type pathArgs struct {
	Path string `json:"path"`
}

type writeArgs struct {
	Path     string `json:"path"`
	Contents string `json:"contents"`
}

// Entry is one directory listing row.
type Entry struct {
	Name  string `json:"name"`
	IsDir bool   `json:"isDir"`
}

func (p *Plugin) readText(_ ipc.Context, argsJSON []byte) (any, error) {
	var args pathArgs
	if err := json.Unmarshal(argsJSON, &args); err != nil {
		return nil, fmt.Errorf("fs: %w", err)
	}
	path, err := p.scope.Resolve(p.base, args.Path)
	if err != nil {
		return nil, err
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("fs: %w", err)
	}
	defer f.Close()
	st, err := f.Stat()
	if err != nil {
		return nil, fmt.Errorf("fs: %w", err)
	}
	if st.IsDir() {
		return nil, fmt.Errorf("fs: %q is a directory", args.Path)
	}
	buf, err := io.ReadAll(io.LimitReader(f, MaxBytes+1))
	if err != nil {
		return nil, fmt.Errorf("fs: %w", err)
	}
	if len(buf) > MaxBytes {
		return nil, fmt.Errorf("fs: %q exceeds %d bytes", args.Path, MaxBytes)
	}
	return string(buf), nil
}

func (p *Plugin) writeText(_ ipc.Context, argsJSON []byte) (any, error) {
	var args writeArgs
	if err := json.Unmarshal(argsJSON, &args); err != nil {
		return nil, fmt.Errorf("fs: %w", err)
	}
	if len(args.Contents) > MaxBytes {
		return nil, fmt.Errorf("fs: contents exceed %d bytes", MaxBytes)
	}
	path, err := p.scope.Resolve(p.base, args.Path)
	if err != nil {
		return nil, err
	}
	if err := os.WriteFile(path, []byte(args.Contents), 0o644); err != nil {
		return nil, fmt.Errorf("fs: %w", err)
	}
	return nil, nil
}

func (p *Plugin) exists(_ ipc.Context, argsJSON []byte) (any, error) {
	var args pathArgs
	if err := json.Unmarshal(argsJSON, &args); err != nil {
		return nil, fmt.Errorf("fs: %w", err)
	}
	path, err := p.scope.Resolve(p.base, args.Path)
	if err != nil {
		return nil, err
	}
	_, err = os.Lstat(path)
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return nil, fmt.Errorf("fs: %w", err)
}

func (p *Plugin) list(_ ipc.Context, argsJSON []byte) (any, error) {
	var args pathArgs
	if err := json.Unmarshal(argsJSON, &args); err != nil {
		return nil, fmt.Errorf("fs: %w", err)
	}
	path, err := p.scope.Resolve(p.base, args.Path)
	if err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(path)
	if err != nil {
		return nil, fmt.Errorf("fs: %w", err)
	}
	out := make([]Entry, 0, len(entries))
	for _, e := range entries {
		out = append(out, Entry{Name: e.Name(), IsDir: e.IsDir()})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}
