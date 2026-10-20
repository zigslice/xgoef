package fs

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Scope is an allow/deny list of directories. Empty allow denies every path.
// Roots may be absolute, relative to base, or one of $APP, $RESOURCE, $HOME
// (optionally followed by a slash and a subpath).
type Scope struct {
	allow []string
	deny  []string
}

// NewScope expands roots against base (the app directory) and resource
// (the frontend dist directory).
func NewScope(base, resource string, allow, deny []string) (*Scope, error) {
	a, err := expandAll(base, resource, allow)
	if err != nil {
		return nil, err
	}
	d, err := expandAll(base, resource, deny)
	if err != nil {
		return nil, err
	}
	return &Scope{allow: a, deny: d}, nil
}

// Resolve returns the cleaned absolute path if it is inside an allow root
// and outside every deny root. Symlinks are evaluated so a link inside the
// scope cannot point outside it.
func (s *Scope) Resolve(base, userPath string) (string, error) {
	if s == nil {
		return "", fmt.Errorf("fs: scope is not configured")
	}
	if strings.TrimSpace(userPath) == "" {
		return "", fmt.Errorf("fs: path is empty")
	}
	lexical := userPath
	if !filepath.IsAbs(lexical) {
		lexical = filepath.Join(base, lexical)
	}
	lexical, err := filepath.Abs(lexical)
	if err != nil {
		return "", err
	}
	lexical = filepath.Clean(lexical)
	if !s.permits(lexical) {
		return "", deny(userPath)
	}
	real, err := realize(lexical)
	if err != nil {
		return "", err
	}
	if !s.permits(real) {
		return "", deny(userPath)
	}
	return real, nil
}

func deny(userPath string) error {
	return fmt.Errorf("fs: path %q is outside the configured scope", userPath)
}

func (s *Scope) permits(path string) bool {
	if s == nil {
		return false
	}
	for _, root := range s.deny {
		if within(path, root) {
			return false
		}
	}
	for _, root := range s.allow {
		if within(path, root) {
			return true
		}
	}
	return false
}

func within(path, root string) bool {
	path = filepath.Clean(path)
	root = filepath.Clean(root)
	if path == root {
		return true
	}
	sep := string(filepath.Separator)
	if !strings.HasSuffix(root, sep) {
		root += sep
	}
	return strings.HasPrefix(path, root)
}

func realize(abs string) (string, error) {
	if _, err := os.Lstat(abs); err == nil {
		return filepath.EvalSymlinks(abs)
	} else if !os.IsNotExist(err) {
		return "", err
	}
	parent := filepath.Dir(abs)
	realParent, err := filepath.EvalSymlinks(parent)
	if err != nil {
		return "", err
	}
	return filepath.Join(realParent, filepath.Base(abs)), nil
}

func expandAll(base, resource string, roots []string) ([]string, error) {
	out := make([]string, 0, len(roots))
	for _, root := range roots {
		exp, err := expandRoot(base, resource, root)
		if err != nil {
			return nil, err
		}
		out = append(out, exp)
	}
	return out, nil
}

func expandRoot(base, resource, root string) (string, error) {
	root = strings.TrimSpace(filepath.ToSlash(root))
	if root == "" {
		return "", fmt.Errorf("fs: empty scope root")
	}
	switch {
	case root == "$APP":
		return filepath.Abs(base)
	case root == "$RESOURCE":
		return filepath.Abs(resource)
	case root == "$HOME":
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		return filepath.Abs(home)
	case strings.HasPrefix(root, "$APP/"):
		return filepath.Abs(filepath.Join(base, root[len("$APP/"):]))
	case strings.HasPrefix(root, "$RESOURCE/"):
		return filepath.Abs(filepath.Join(resource, root[len("$RESOURCE/"):]))
	case strings.HasPrefix(root, "$HOME/"):
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		return filepath.Abs(filepath.Join(home, root[len("$HOME/"):]))
	default:
		if filepath.IsAbs(root) {
			return filepath.Abs(root)
		}
		return filepath.Abs(filepath.Join(base, filepath.FromSlash(root)))
	}
}
