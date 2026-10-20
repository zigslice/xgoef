// Package asset maps xgoef://app/... onto one frontend directory.
// Requests that would leave that directory are rejected, including symlinks.
package asset

import (
	"fmt"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"
)

// Scheme is the custom origin used for packaged frontend files.
const Scheme = "xgoef"

// Host is the only host the scheme serves. xgoef://app/<path> is a file
// under the frontend root.
const Host = "app"

// Root is one frontend directory (build.frontendDist).
type Root struct {
	dir string // symlink-resolved absolute path
}

// Open checks that dir exists and resolves symlinks in the root itself.
func Open(dir string) (*Root, error) {
	if dir == "" {
		return nil, fmt.Errorf("asset: frontend root is empty")
	}
	abs, err := filepath.Abs(dir)
	if err != nil {
		return nil, err
	}
	st, err := os.Stat(abs)
	if err != nil {
		return nil, fmt.Errorf("asset: frontend root %s: %w", abs, err)
	}
	if !st.IsDir() {
		return nil, fmt.Errorf("asset: frontend root %s is not a directory", abs)
	}
	real, err := filepath.EvalSymlinks(abs)
	if err != nil {
		return nil, err
	}
	return &Root{dir: real}, nil
}

// Dir is the symlink-resolved frontend root.
func (r *Root) Dir() string {
	if r == nil {
		return ""
	}
	return r.dir
}

// Resolve maps an xgoef://app URL to a file inside the root and its MIME type.
// Query and fragment are ignored. A directory URL serves index.html.
func (r *Root) Resolve(rawURL string) (string, string, error) {
	if r == nil {
		return "", "", fmt.Errorf("asset: frontend root is not configured")
	}
	rel, err := Rel(rawURL)
	if err != nil {
		return "", "", err
	}
	return r.resolveRel(rel)
}

// Rel returns the cleaned slash-separated path inside the root for an
// xgoef://app URL. An empty result means the root document (index.html).
func Rel(rawURL string) (string, error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return "", fmt.Errorf("asset: %w", err)
	}
	if !strings.EqualFold(u.Scheme, Scheme) || !strings.EqualFold(u.Host, Host) {
		return "", fmt.Errorf("asset: unsupported url %q", rawURL)
	}
	if strings.Contains(u.Path, "\\") || strings.ContainsRune(u.Path, 0) {
		return "", fmt.Errorf("asset: path %q is outside the frontend root", rawURL)
	}
	cleaned := path.Clean("/" + u.Path)
	if cleaned == "/" || cleaned == "." {
		return "", nil
	}
	rel := strings.TrimPrefix(cleaned, "/")
	if rel == "" || rel == ".." || strings.HasPrefix(rel, "../") {
		return "", fmt.Errorf("asset: path %q is outside the frontend root", rawURL)
	}
	return rel, nil
}

// FileURL maps an existing file under the root to an xgoef://app URL.
func (r *Root) FileURL(filePath string) (string, error) {
	if r == nil {
		return "", fmt.Errorf("asset: frontend root is not configured")
	}
	abs, err := filepath.Abs(filePath)
	if err != nil {
		return "", err
	}
	real, err := filepath.EvalSymlinks(abs)
	if err != nil {
		return "", err
	}
	if !within(real, r.dir) {
		return "", fmt.Errorf("asset: path %q is outside the frontend root", filePath)
	}
	rel, err := filepath.Rel(r.dir, real)
	if err != nil {
		return "", err
	}
	return Scheme + "://" + Host + "/" + filepath.ToSlash(rel), nil
}

func (r *Root) resolveRel(rel string) (string, string, error) {
	if rel == "" {
		rel = "index.html"
	}
	joined := filepath.Join(r.dir, filepath.FromSlash(rel))
	if st, err := os.Stat(joined); err == nil && st.IsDir() {
		joined = filepath.Join(joined, "index.html")
	}
	real, err := realize(joined)
	if err != nil {
		return "", "", fmt.Errorf("asset: %w", err)
	}
	if st, err := os.Stat(real); err == nil && st.IsDir() {
		real, err = realize(filepath.Join(real, "index.html"))
		if err != nil {
			return "", "", fmt.Errorf("asset: %w", err)
		}
	}
	if !within(real, r.dir) {
		return "", "", fmt.Errorf("asset: path %q is outside the frontend root", rel)
	}
	return real, Mime(real), nil
}

func realize(abs string) (string, error) {
	if _, err := os.Lstat(abs); err != nil {
		return "", err
	}
	return filepath.EvalSymlinks(abs)
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

// Mime returns a content type for a frontend file name.
func Mime(filePath string) string {
	switch strings.ToLower(filepath.Ext(filePath)) {
	case ".html", ".htm":
		return "text/html"
	case ".css":
		return "text/css"
	case ".js", ".mjs":
		return "text/javascript"
	case ".json", ".map":
		return "application/json"
	case ".svg":
		return "image/svg+xml"
	case ".png":
		return "image/png"
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".webp":
		return "image/webp"
	case ".ico":
		return "image/x-icon"
	case ".wasm":
		return "application/wasm"
	case ".woff2":
		return "font/woff2"
	case ".txt":
		return "text/plain"
	default:
		return "application/octet-stream"
	}
}
