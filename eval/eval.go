// Package eval checks which JavaScript the host may run.
// The page never supplies the source. A script runs only in the main frame
// of a window whose current URL is still the origin pinned for that window.
package eval

import (
	"errors"
	"fmt"
	"strings"
)

const (
	// MaxScriptBytes is the largest accepted script, in bytes.
	MaxScriptBytes = 16 * 1024
	// MaxInitScripts is how many init scripts one window may have.
	MaxInitScripts = 8
	// MaxNamedScripts is how many named scripts the config may register.
	MaxNamedScripts = 32
	// MaxNameBytes is the longest script name.
	MaxNameBytes = 32
)

// ErrRejected means the script text itself is not acceptable.
var ErrRejected = errors.New("eval: script rejected")

// ErrOrigin means the frame is not on the pinned origin, so nothing ran.
var ErrOrigin = errors.New("eval: origin denied")

// ErrUnpinned means this label has no pinned URL.
var ErrUnpinned = errors.New("eval: window is not pinned")

// ErrUnavailable means the browser did not run the script.
var ErrUnavailable = errors.New("eval: unavailable")

// ValidateSource reports whether source may be executed.
// The text must be non-empty after trimming ASCII space and at most 16 KiB.
// A NUL byte is rejected so the C ABI cannot truncate it.
func ValidateSource(source string) error {
	if len(source) == 0 || len(source) > MaxScriptBytes {
		return fmt.Errorf("%w: length", ErrRejected)
	}
	if strings.IndexByte(source, 0) >= 0 {
		return fmt.Errorf("%w: nul", ErrRejected)
	}
	if strings.Trim(source, " \t\r\n\v\f") == "" {
		return fmt.Errorf("%w: empty", ErrRejected)
	}
	return nil
}

// ValidateSources checks a list of init scripts.
func ValidateSources(scripts []string, limit int) error {
	if len(scripts) > limit {
		return fmt.Errorf("%w: count", ErrRejected)
	}
	for _, source := range scripts {
		if err := ValidateSource(source); err != nil {
			return err
		}
	}
	return nil
}

// ValidateName reports whether name may be used from the page.
// Names are a lowercase token, so a caller cannot smuggle source text.
func ValidateName(name string) error {
	if len(name) == 0 || len(name) > MaxNameBytes {
		return fmt.Errorf("eval: name is invalid")
	}
	for i := 0; i < len(name); i++ {
		c := name[i]
		switch {
		case c >= 'a' && c <= 'z':
		case i > 0 && c >= '0' && c <= '9':
		case i > 0 && (c == '_' || c == '-'):
		default:
			return fmt.Errorf("eval: name is invalid")
		}
	}
	return nil
}

// ValidateNamed checks the config map. The page can only ask for these names.
func ValidateNamed(scripts map[string]string) error {
	if len(scripts) > MaxNamedScripts {
		return fmt.Errorf("%w: count", ErrRejected)
	}
	for name, source := range scripts {
		if err := ValidateName(name); err != nil {
			return err
		}
		if err := ValidateSource(source); err != nil {
			return fmt.Errorf("eval: script %q: %w", name, err)
		}
	}
	return nil
}

// Encode writes a length-prefixed script table. The result contains no NUL
// when every source passed ValidateSource.
func Encode(scripts []string) (string, error) {
	if err := ValidateSources(scripts, MaxInitScripts); err != nil {
		return "", err
	}
	var b strings.Builder
	fmt.Fprintf(&b, "%d\n", len(scripts))
	for _, source := range scripts {
		fmt.Fprintf(&b, "%d\n", len(source))
		b.WriteString(source)
	}
	return b.String(), nil
}

// Decode reads a table from Encode. Trailing bytes are rejected.
func Decode(text string) ([]string, error) {
	if strings.IndexByte(text, 0) >= 0 {
		return nil, fmt.Errorf("%w: nul", ErrRejected)
	}
	line, rest, ok := takeLine(text)
	if !ok {
		return nil, fmt.Errorf("%w: table", ErrRejected)
	}
	count, ok := parseDecimal(line, MaxInitScripts)
	if !ok {
		return nil, fmt.Errorf("%w: count", ErrRejected)
	}
	out := make([]string, 0, count)
	for i := 0; i < count; i++ {
		line, rest, ok = takeLine(rest)
		if !ok {
			return nil, fmt.Errorf("%w: table", ErrRejected)
		}
		n, ok := parseDecimal(line, MaxScriptBytes)
		if !ok || n < 1 {
			return nil, fmt.Errorf("%w: length", ErrRejected)
		}
		if len(rest) < n {
			return nil, fmt.Errorf("%w: table", ErrRejected)
		}
		source := rest[:n]
		rest = rest[n:]
		if err := ValidateSource(source); err != nil {
			return nil, err
		}
		out = append(out, source)
	}
	if rest != "" {
		return nil, fmt.Errorf("%w: table", ErrRejected)
	}
	return out, nil
}

func takeLine(s string) (string, string, bool) {
	i := strings.IndexByte(s, '\n')
	if i < 0 {
		return "", "", false
	}
	return s[:i], s[i+1:], true
}

func parseDecimal(s string, max int) (int, bool) {
	if s == "" || (len(s) > 1 && s[0] == '0') || len(s) > 6 {
		return 0, false
	}
	n := 0
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c < '0' || c > '9' {
			return 0, false
		}
		n = n*10 + int(c-'0')
		if n > max {
			return 0, false
		}
	}
	return n, true
}

// OriginAllowed reports whether frame is still on the origin of allowed.
// allowed must itself be a pinned origin: xgoef://app, or an http(s) URL.
// Paths, queries, and fragments are ignored. Any other scheme is denied,
// including javascript, data, file, and about. Userinfo and backslashes
// are denied. Hosts are ASCII.
func OriginAllowed(frame, allowed string) bool {
	f, ok := parseOrigin(frame)
	if !ok {
		return false
	}
	a, ok := parseOrigin(allowed)
	if !ok {
		return false
	}
	if f.scheme != a.scheme || f.host != a.host || f.port != a.port {
		return false
	}
	switch a.scheme {
	case "xgoef":
		return a.host == "app"
	case "http", "https":
		return true
	default:
		return false
	}
}

type origin struct {
	scheme string
	host   string
	port   int
}

func parseOrigin(raw string) (origin, bool) {
	if raw == "" || len(raw) > 2048 {
		return origin{}, false
	}
	for i := 0; i < len(raw); i++ {
		switch raw[i] {
		case 0, ' ', '\t', '\r', '\n', '\\', '<', '>', '"', '\'':
			return origin{}, false
		}
	}
	scheme, rest, ok := strings.Cut(raw, "://")
	if !ok || scheme == "" || rest == "" {
		return origin{}, false
	}
	scheme, ok = asciiLower(scheme)
	if !ok || !schemeOK(scheme) {
		return origin{}, false
	}
	if i := strings.IndexByte(rest, '#'); i >= 0 {
		rest = rest[:i]
	}
	if i := strings.IndexByte(rest, '?'); i >= 0 {
		rest = rest[:i]
	}
	if i := strings.IndexByte(rest, '/'); i >= 0 {
		rest = rest[:i]
	}
	if rest == "" || strings.Contains(rest, "@") {
		return origin{}, false
	}
	host, portText, ok := splitHost(rest)
	if !ok {
		return origin{}, false
	}
	host, ok = asciiLower(host)
	if !ok || host == "" {
		return origin{}, false
	}
	port, ok := normalizePort(scheme, portText)
	if !ok {
		return origin{}, false
	}
	return origin{scheme: scheme, host: host, port: port}, true
}

func schemeOK(scheme string) bool {
	if scheme[0] < 'a' || scheme[0] > 'z' {
		return false
	}
	for i := 1; i < len(scheme); i++ {
		c := scheme[i]
		switch {
		case c >= 'a' && c <= 'z':
		case c >= '0' && c <= '9':
		case c == '+' || c == '-' || c == '.':
		default:
			return false
		}
	}
	return true
}

func splitHost(authority string) (string, string, bool) {
	if authority[0] == '[' {
		end := strings.IndexByte(authority, ']')
		if end < 2 {
			return "", "", false
		}
		host := authority[1:end]
		if host == "" || strings.IndexByte(host, '[') >= 0 {
			return "", "", false
		}
		rest := authority[end+1:]
		if rest == "" {
			return host, "", true
		}
		if rest[0] != ':' || len(rest) == 1 {
			return "", "", false
		}
		return host, rest[1:], true
	}
	if strings.Count(authority, ":") > 1 {
		return "", "", false
	}
	host, port, ok := strings.Cut(authority, ":")
	if !ok {
		return host, "", true
	}
	if host == "" || port == "" {
		return "", "", false
	}
	return host, port, true
}

func normalizePort(scheme, portText string) (int, bool) {
	if portText == "" {
		switch scheme {
		case "http":
			return 80, true
		case "https":
			return 443, true
		case "xgoef":
			return -1, true
		default:
			return 0, false
		}
	}
	n, ok := parseDecimal(portText, 65535)
	if !ok || n < 1 {
		return 0, false
	}
	return n, true
}

func asciiLower(s string) (string, bool) {
	b := []byte(s)
	for i, c := range b {
		if c >= 'A' && c <= 'Z' {
			b[i] = c - 'A' + 'a'
			continue
		}
		if c >= 128 {
			return "", false
		}
	}
	return string(b), true
}
