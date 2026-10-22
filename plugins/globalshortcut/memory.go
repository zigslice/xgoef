package globalshortcut

import "sync"

// Memory is an in-process grabber. Tests use it in place of the display.
type Memory struct {
	mu    sync.Mutex
	grabs []string
	hook  func(Shortcut, string)
	err   error
}

// NewMemory returns a grabber that records grabs and can fire events.
func NewMemory() *Memory {
	return &Memory{}
}

// SetError makes later Grab and Ungrab calls fail.
func (m *Memory) SetError(err error) {
	if m == nil {
		return
	}
	m.mu.Lock()
	m.err = err
	m.mu.Unlock()
}

// Bind receives the plugin hook. SetSession calls it.
func (m *Memory) Bind(hook func(Shortcut, string)) {
	if m == nil {
		return
	}
	m.mu.Lock()
	m.hook = hook
	m.mu.Unlock()
}

// Grab records one accelerator.
func (m *Memory) Grab(s Shortcut) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.err != nil {
		return m.err
	}
	name := s.String()
	for _, got := range m.grabs {
		if got == name {
			return nil
		}
	}
	m.grabs = append(m.grabs, name)
	return nil
}

// Ungrab forgets one accelerator.
func (m *Memory) Ungrab(s Shortcut) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.err != nil {
		return m.err
	}
	name := s.String()
	next := make([]string, 0, len(m.grabs))
	for _, got := range m.grabs {
		if got != name {
			next = append(next, got)
		}
	}
	m.grabs = next
	return nil
}

// Close implements Session. The recorded grabs stay until Ungrab.
func (m *Memory) Close() error { return nil }

// Grabs returns the accelerators currently held, in grab order.
func (m *Memory) Grabs() []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]string, len(m.grabs))
	copy(out, m.grabs)
	return out
}

// Press delivers Pressed through the plugin hook.
func (m *Memory) Press(s Shortcut) { m.fire(s, Pressed) }

// Release delivers Released through the plugin hook.
func (m *Memory) Release(s Shortcut) { m.fire(s, Released) }

func (m *Memory) fire(s Shortcut, state string) {
	m.mu.Lock()
	hook := m.hook
	m.mu.Unlock()
	if hook != nil {
		hook(s, state)
	}
}
