package globalshortcut

import "errors"

// Session installs and removes operating-system grabs.
type Session interface {
	Grab(Shortcut) error
	Ungrab(Shortcut) error
	Close() error
}

// errUnavailable means this process cannot install a global grab.
var errUnavailable = errors.New("global shortcut is not available")

// openSession connects the platform grabber. Tests replace it.
var openSession = defaultOpen
