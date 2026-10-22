//go:build !linux

package globalshortcut

func defaultOpen(func(Shortcut, string)) (Session, error) {
	return nil, errUnavailable
}
