// Package windowstate remembers each window's screen place between runs.
// The file is JSON. The shim receives the same data as tab-separated lines.
package windowstate

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
)

const (
	minSize = 1
	maxSize = 16384
	minPos  = -32768
	maxPos  = 32768
)

// Place is one window's last geometry. Width and height are the restored
// size. Maximized and Minimized are remembered separately so those frames
// do not replace that size. Minimized keeps the previous Maximized flag.
type Place struct {
	X         int  `json:"x"`
	Y         int  `json:"y"`
	Width     int  `json:"width"`
	Height    int  `json:"height"`
	Maximized bool `json:"maximized"`
	Minimized bool `json:"minimized"`
}

// Positioned reports whether x, y, width, and height should be applied.
func (p Place) Positioned() bool {
	return p.Width >= minSize && p.Height >= minSize
}

type fileJSON struct {
	Windows map[string]Place `json:"windows"`
}

// Store is the on-disk map keyed by window label.
type Store struct {
	mu      sync.Mutex
	path    string
	windows map[string]Place
}

// New is an empty store that writes to path.
func New(path string) *Store {
	return &Store{path: path, windows: map[string]Place{}}
}

// Load reads path. A missing file is an empty store.
// Broken JSON returns an error. Entries with a bad label or size are dropped.
func Load(path string) (*Store, error) {
	st := &Store{path: path, windows: map[string]Place{}}
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return st, nil
	}
	if err != nil {
		return nil, err
	}
	var raw fileJSON
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("windowstate: %w", err)
	}
	for label, p := range raw.Windows {
		if !validLabel(label) || !validPlace(p) {
			continue
		}
		st.windows[label] = p
	}
	return st, nil
}

// Get returns the saved place for label.
func (s *Store) Get(label string) (Place, bool) {
	if s == nil {
		return Place{}, false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	p, ok := s.windows[label]
	return p, ok
}

// Note records a live geometry change and writes the file.
// minimized keeps the previous restored size and the previous maximized flag.
// maximized with a non-positive size keeps the previous restored size and
// clears minimized. A normal note replaces the bounds and clears both flags.
func (s *Store) Note(label string, x, y, width, height int, maximized, minimized bool) error {
	if s == nil {
		return fmt.Errorf("windowstate: not configured")
	}
	label = strings.TrimSpace(label)
	if !validLabel(label) {
		return fmt.Errorf("windowstate: bad label %q", label)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	cur := s.windows[label]
	if minimized {
		cur.Minimized = true
		s.windows[label] = cur
		return s.writeLocked()
	}
	if maximized {
		cur.Maximized = true
		cur.Minimized = false
		s.windows[label] = cur
		return s.writeLocked()
	}
	next := Place{X: x, Y: y, Width: width, Height: height}
	if !validPlace(next) {
		return fmt.Errorf("windowstate: %s bounds out of range", label)
	}
	s.windows[label] = next
	return s.writeLocked()
}

// Encode writes one line per label: label, x, y, width, height, flags.
// flags contains p when the place has a size, m when maximized, and n when
// minimized. Labels are sorted so the text is stable.
func (s *Store) Encode() string {
	if s == nil {
		return ""
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	labels := make([]string, 0, len(s.windows))
	for label := range s.windows {
		labels = append(labels, label)
	}
	sort.Strings(labels)
	var b strings.Builder
	for _, label := range labels {
		p := s.windows[label]
		flags := ""
		if p.Positioned() {
			flags += "p"
		}
		if p.Maximized {
			flags += "m"
		}
		if p.Minimized {
			flags += "n"
		}
		if flags == "" {
			continue
		}
		fmt.Fprintf(&b, "%s\t%d\t%d\t%d\t%d\t%s\n", label, p.X, p.Y, p.Width, p.Height, flags)
	}
	return b.String()
}

func (s *Store) writeLocked() error {
	if s.path == "" {
		return fmt.Errorf("windowstate: empty path")
	}
	body, err := json.MarshalIndent(fileJSON{Windows: s.windows}, "", "  ")
	if err != nil {
		return err
	}
	body = append(body, '\n')
	dir := filepath.Dir(s.path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, body, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, s.path)
}

func validLabel(label string) bool {
	if label == "" || strings.ContainsAny(label, "\t\r\n") {
		return false
	}
	return true
}

func validPlace(p Place) bool {
	if p.Width == 0 && p.Height == 0 && p.X == 0 && p.Y == 0 {
		return p.Maximized || p.Minimized
	}
	if p.Width < minSize || p.Height < minSize || p.Width > maxSize || p.Height > maxSize {
		return false
	}
	if p.X < minPos || p.Y < minPos || p.X > maxPos || p.Y > maxPos {
		return false
	}
	return true
}
