// Package remote is the http(s) origin allowlist for window navigation.
// The page cannot choose these origins. Paths are not a scope.
package remote

import (
	"fmt"
	"strconv"
	"strings"
)

const (
	// MaxURLs is the most origins app.remote.urls may list.
	MaxURLs = 16
	// MaxURLBytes is the longest single config entry.
	MaxURLBytes = 256
)

// Normalize checks config entries and returns canonical origins.
// An entry is an origin: scheme, host, and port. "/" is accepted and dropped.
// A longer path, a query, a fragment, or userinfo is rejected.
func Normalize(urls []string) ([]string, error) {
	if len(urls) > MaxURLs {
		return nil, fmt.Errorf("remote: too many urls")
	}
	if len(urls) == 0 {
		return nil, nil
	}
	out := make([]string, 0, len(urls))
	seen := map[string]bool{}
	for _, raw := range urls {
		origin, err := Canonical(raw)
		if err != nil {
			return nil, err
		}
		if seen[origin] {
			return nil, fmt.Errorf("remote: duplicate url")
		}
		seen[origin] = true
		out = append(out, origin)
	}
	return out, nil
}

// Canonical returns the origin form of one config entry.
func Canonical(raw string) (string, error) {
	if !entryTextOK(raw) || strings.ContainsAny(raw, "@?#") {
		return "", fmt.Errorf("remote: url is invalid")
	}
	mark := strings.Index(raw, "://")
	if mark < 0 {
		return "", fmt.Errorf("remote: url is invalid")
	}
	rest := raw[mark+3:]
	if i := strings.IndexByte(rest, '/'); i >= 0 && rest[i:] != "/" {
		return "", fmt.Errorf("remote: url is invalid")
	}
	origin, ok := Origin(raw)
	if !ok || !hostOK(origin) {
		return "", fmt.Errorf("remote: url is invalid")
	}
	return origin, nil
}

// IsHTTP reports whether raw uses the http or https scheme.
func IsHTTP(raw string) bool {
	i := strings.IndexByte(raw, ':')
	if i <= 0 || !strings.HasPrefix(raw[i:], "://") {
		return false
	}
	scheme, ok := lowerASCII(raw[:i])
	return ok && (scheme == "http" || scheme == "https")
}

// Origin is the canonical http(s) origin of a navigation URL.
// The path, query, and fragment are ignored. Userinfo is ignored.
// Default ports are omitted. A non-http(s) URL is not an origin.
func Origin(raw string) (string, bool) {
	if raw == "" || len(raw) > 2048 {
		return "", false
	}
	for i := 0; i < len(raw); i++ {
		if raw[i] < 0x20 || raw[i] >= 128 {
			return "", false
		}
	}
	mark := strings.Index(raw, "://")
	if mark <= 0 || !schemeChars(raw[:mark]) {
		return "", false
	}
	scheme, ok := lowerASCII(raw[:mark])
	if !ok || (scheme != "http" && scheme != "https") {
		return "", false
	}
	rest := raw[mark+3:]
	if cut := strings.IndexAny(rest, "/?#"); cut >= 0 {
		rest = rest[:cut]
	}
	if at := strings.LastIndexByte(rest, '@'); at >= 0 {
		rest = rest[at+1:]
	}
	if rest == "" {
		return "", false
	}
	host, port, hasPort, ok := splitAuthority(rest)
	if !ok {
		return "", false
	}
	host, ok = lowerASCII(host)
	if !ok || host == "" || (host[0] == '[' && host[len(host)-1] != ']') {
		return "", false
	}
	if hasPort && port == "" {
		return "", false
	}
	if port != "" {
		if len(port) > 5 {
			return "", false
		}
		value := 0
		for i := 0; i < len(port); i++ {
			c := port[i]
			if c < '0' || c > '9' {
				return "", false
			}
			value = value*10 + int(c-'0')
		}
		if value < 1 || value > 65535 {
			return "", false
		}
		def := 80
		if scheme == "https" {
			def = 443
		}
		if value == def {
			port = ""
		} else {
			port = strconv.Itoa(value)
		}
	}
	if port == "" {
		return scheme + "://" + host, true
	}
	return scheme + "://" + host + ":" + port, true
}

// Allowed reports whether raw's origin is one of origins.
func Allowed(raw string, origins []string) bool {
	origin, ok := Origin(raw)
	if !ok {
		return false
	}
	for _, item := range origins {
		if item == origin {
			return true
		}
	}
	return false
}

func entryTextOK(raw string) bool {
	if raw == "" || len(raw) > MaxURLBytes {
		return false
	}
	for i := 0; i < len(raw); i++ {
		switch raw[i] {
		case ' ', '\\', '<', '>', '"', '\'':
			return false
		}
		if raw[i] < 0x20 {
			return false
		}
	}
	return true
}

func schemeChars(s string) bool {
	if s == "" {
		return false
	}
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case c >= 'a' && c <= 'z':
		case c >= 'A' && c <= 'Z':
		case c >= '0' && c <= '9':
		case c == '+' || c == '-' || c == '.':
		default:
			return false
		}
	}
	return true
}

func splitAuthority(rest string) (host, port string, hasPort, ok bool) {
	if rest[0] == '[' {
		br := strings.IndexByte(rest, ']')
		if br < 0 {
			return "", "", false, false
		}
		host = rest[:br+1]
		if br+1 >= len(rest) {
			return host, "", false, true
		}
		if rest[br+1] != ':' {
			return "", "", false, false
		}
		return host, rest[br+2:], true, true
	}
	colon := strings.LastIndexByte(rest, ':')
	if colon >= 0 {
		return rest[:colon], rest[colon+1:], true, true
	}
	return rest, "", false, true
}

func hostOK(origin string) bool {
	mark := strings.Index(origin, "://")
	if mark < 0 {
		return false
	}
	host := origin[mark+3:]
	if strings.HasPrefix(host, "[") {
		end := strings.IndexByte(host, ']')
		if end < 2 {
			return false
		}
		if !ipv6OK(host[1:end]) {
			return false
		}
		rest := host[end+1:]
		if rest == "" {
			return true
		}
		return len(rest) > 1 && rest[0] == ':' && portDigits(rest[1:])
	}
	if colon := strings.LastIndexByte(host, ':'); colon >= 0 {
		if !portDigits(host[colon+1:]) {
			return false
		}
		host = host[:colon]
	}
	return dnsHostOK(host)
}

func ipv6OK(inner string) bool {
	if inner == "" || !strings.Contains(inner, ":") {
		return false
	}
	for i := 0; i < len(inner); i++ {
		c := inner[i]
		switch {
		case c >= '0' && c <= '9':
		case c >= 'a' && c <= 'f':
		case c == ':':
		default:
			return false
		}
	}
	return true
}

func portDigits(port string) bool {
	if port == "" || len(port) > 5 {
		return false
	}
	for i := 0; i < len(port); i++ {
		if port[i] < '0' || port[i] > '9' {
			return false
		}
	}
	return true
}

func dnsHostOK(host string) bool {
	if host == "" || host[0] == '.' || host[len(host)-1] == '.' {
		return false
	}
	labels := strings.Split(host, ".")
	for _, label := range labels {
		if label == "" || len(label) > 63 {
			return false
		}
		if label[0] == '-' || label[len(label)-1] == '-' {
			return false
		}
		for i := 0; i < len(label); i++ {
			c := label[i]
			switch {
			case c >= 'a' && c <= 'z':
			case c >= '0' && c <= '9':
			case c == '-':
			default:
				return false
			}
		}
	}
	return true
}

func lowerASCII(s string) (string, bool) {
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
