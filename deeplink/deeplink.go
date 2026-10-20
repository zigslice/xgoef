// Package deeplink accepts custom URL schemes and registers them for the
// current user with an XDG desktop file. Platform installers are separate.
package deeplink

import (
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

const maxScheme = 64

var reserved = map[string]bool{
	"about": true, "data": true, "file": true, "http": true, "https": true,
	"javascript": true, "mailto": true, "ws": true, "wss": true, "xgoef": true,
}

// Normalize checks custom schemes. Empty input means deep link is off.
// Names are stored lowercase. http, https, file, and xgoef are reserved.
func Normalize(schemes []string) ([]string, error) {
	if len(schemes) == 0 {
		return nil, nil
	}
	seen := map[string]bool{}
	out := make([]string, 0, len(schemes))
	for _, raw := range schemes {
		s := strings.ToLower(strings.TrimSpace(raw))
		if !validScheme(s) {
			return nil, fmt.Errorf("deeplink: invalid scheme %q", raw)
		}
		if reserved[s] {
			return nil, fmt.Errorf("deeplink: scheme %q is reserved", s)
		}
		if seen[s] {
			return nil, fmt.Errorf("deeplink: duplicate scheme %q", s)
		}
		seen[s] = true
		out = append(out, s)
	}
	return out, nil
}

// FromArgs returns arguments whose scheme is in the allowed list.
func FromArgs(args, schemes []string) []string {
	allow := map[string]bool{}
	for _, s := range schemes {
		allow[strings.ToLower(s)] = true
	}
	var out []string
	for _, arg := range args {
		if arg == "" || strings.HasPrefix(arg, "-") || strings.ContainsAny(arg, " \t\r\n") {
			continue
		}
		if !strings.Contains(arg, "://") {
			continue
		}
		u, err := url.Parse(arg)
		if err != nil || u.Scheme == "" || !allow[strings.ToLower(u.Scheme)] {
			continue
		}
		out = append(out, arg)
	}
	return out
}

// FileName is the desktop entry file for an application identifier.
func FileName(identifier string) (string, error) {
	name := sanitize(identifier)
	if name == "" {
		return "", fmt.Errorf("deeplink: identifier is empty")
	}
	return "xgoef-" + name + ".desktop", nil
}

// File is the contents of a desktop entry that opens exec for schemes.
// exec must be absolute. The field uses %u so the OS passes one URL.
func File(name, exec string, schemes []string) (string, error) {
	schemes, err := Normalize(schemes)
	if err != nil {
		return "", err
	}
	if len(schemes) == 0 {
		return "", fmt.Errorf("deeplink: no schemes")
	}
	title := strings.TrimSpace(name)
	if title == "" || strings.ContainsAny(title, "\r\n") {
		return "", fmt.Errorf("deeplink: application name is empty")
	}
	if !filepath.IsAbs(exec) || strings.ContainsAny(exec, "\r\n") {
		return "", fmt.Errorf("deeplink: executable must be absolute")
	}
	quoted, err := quoteExec(exec)
	if err != nil {
		return "", err
	}
	var mime strings.Builder
	for _, s := range schemes {
		mime.WriteString("x-scheme-handler/")
		mime.WriteString(s)
		mime.WriteByte(';')
	}
	return "[Desktop Entry]\nVersion=1.0\nType=Application\nName=" + title +
		"\nExec=" + quoted + " %u\nMimeType=" + mime.String() + "\nNoDisplay=true\n", nil
}

// Install writes the desktop entry and points mimeapps.list at it.
// applicationsDir receives the .desktop file. configDir receives mimeapps.list.
func Install(applicationsDir, configDir, identifier, name, exec string, schemes []string) (string, error) {
	file, err := FileName(identifier)
	if err != nil {
		return "", err
	}
	body, err := File(name, exec, schemes)
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(applicationsDir, 0o755); err != nil {
		return "", err
	}
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		return "", err
	}
	path := filepath.Join(applicationsDir, file)
	if filepath.Dir(path) != filepath.Clean(applicationsDir) {
		return "", fmt.Errorf("deeplink: desktop path left %s", applicationsDir)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		return "", err
	}
	listPath := filepath.Join(configDir, "mimeapps.list")
	existing, err := os.ReadFile(listPath)
	if err != nil && !os.IsNotExist(err) {
		return "", err
	}
	normalized, err := Normalize(schemes)
	if err != nil {
		return "", err
	}
	merged := mergeMIME(string(existing), file, normalized)
	if err := os.WriteFile(listPath, []byte(merged), 0o644); err != nil {
		return "", err
	}
	return path, nil
}

func mergeMIME(existing, desktop string, schemes []string) string {
	want := map[string]string{}
	var keys []string
	for _, s := range schemes {
		key := "x-scheme-handler/" + s
		want[key] = desktop
		keys = append(keys, key)
	}
	lines := strings.Split(strings.ReplaceAll(existing, "\r\n", "\n"), "\n")
	var out []string
	inDefault := false
	seenDefault := false
	written := map[string]bool{}
	flush := func() {
		var missing []string
		for _, key := range keys {
			if !written[key] {
				missing = append(missing, key)
			}
		}
		sort.Strings(missing)
		for _, key := range missing {
			out = append(out, key+"="+want[key])
			written[key] = true
		}
	}
	for _, line := range lines {
		trim := strings.TrimSpace(line)
		if strings.HasPrefix(trim, "[") && strings.HasSuffix(trim, "]") {
			if inDefault {
				flush()
			}
			inDefault = trim == "[Default Applications]"
			if inDefault {
				seenDefault = true
			}
			out = append(out, line)
			continue
		}
		if inDefault {
			key, _, ok := strings.Cut(trim, "=")
			key = strings.TrimSpace(key)
			if ok {
				if dest, replace := want[key]; replace {
					if written[key] {
						continue
					}
					out = append(out, key+"="+dest)
					written[key] = true
					continue
				}
			}
		}
		out = append(out, line)
	}
	if inDefault {
		flush()
	}
	if !seenDefault {
		if len(out) > 0 && strings.TrimSpace(out[len(out)-1]) != "" {
			out = append(out, "")
		}
		out = append(out, "[Default Applications]")
		flush()
	}
	body := strings.Join(out, "\n")
	if !strings.HasSuffix(body, "\n") {
		body += "\n"
	}
	return body
}

func validScheme(s string) bool {
	if s == "" || len(s) > maxScheme {
		return false
	}
	for i, r := range s {
		switch {
		case r >= 'a' && r <= 'z':
		case i > 0 && (r >= '0' && r <= '9' || r == '+' || r == '-' || r == '.'):
		default:
			return false
		}
	}
	return true
}

func sanitize(identifier string) string {
	var b strings.Builder
	for _, r := range strings.TrimSpace(identifier) {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '.', r == '_', r == '-':
			b.WriteRune(r)
		default:
			b.WriteByte('_')
		}
	}
	name := b.String()
	if name == "" || name == "." || name == ".." {
		return ""
	}
	return name
}

func quoteExec(path string) (string, error) {
	var b strings.Builder
	b.WriteByte('"')
	for _, r := range path {
		if r == '\\' || r == '"' || r == '$' || r == '`' {
			b.WriteByte('\\')
		}
		b.WriteRune(r)
	}
	b.WriteByte('"')
	return b.String(), nil
}
