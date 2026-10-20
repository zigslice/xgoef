// Package bundle writes deb, msi, and dmg packages for a staged xgoef tree.
// The packages are not OS-signed.
package bundle

import (
	"fmt"
	"runtime"
	"strings"
)

// Spec describes the application being packed.
type Spec struct {
	Product    string
	Version    string
	Identifier string
	Arch       string
	Schemes    []string
}

// DebianArch maps a GOARCH name to a Debian architecture.
func DebianArch(goarch string) string {
	switch goarch {
	case "386":
		return "i386"
	case "amd64":
		return "amd64"
	case "arm":
		return "armhf"
	case "arm64":
		return "arm64"
	default:
		if goarch == "" {
			return DebianArch(runtime.GOARCH)
		}
		return goarch
	}
}

func (s Spec) prepare() (Spec, error) {
	s.Product = strings.TrimSpace(s.Product)
	s.Version = strings.TrimSpace(s.Version)
	s.Identifier = strings.TrimSpace(s.Identifier)
	if s.Product == "" {
		return s, fmt.Errorf("bundle: product name is empty")
	}
	if s.Version == "" {
		return s, fmt.Errorf("bundle: version is empty")
	}
	if s.Arch == "" {
		s.Arch = DebianArch(runtime.GOARCH)
	}
	if _, err := packageName(s.Product); err != nil {
		return s, err
	}
	if _, err := debianVersion(s.Version); err != nil {
		return s, err
	}
	if _, err := msiVersion(s.Version); err != nil {
		return s, err
	}
	var schemes []string
	for _, scheme := range s.Schemes {
		scheme = strings.ToLower(strings.TrimSpace(scheme))
		if scheme == "" {
			continue
		}
		schemes = append(schemes, scheme)
	}
	s.Schemes = schemes
	return s, nil
}

func packageName(product string) (string, error) {
	var b strings.Builder
	for _, r := range strings.ToLower(strings.TrimSpace(product)) {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9', r == '+', r == '-', r == '.':
			b.WriteRune(r)
		default:
			b.WriteByte('-')
		}
	}
	name := strings.Trim(b.String(), "-.")
	if name == "" {
		return "", fmt.Errorf("bundle: package name is empty")
	}
	first := name[0]
	if (first < 'a' || first > 'z') && (first < '0' || first > '9') {
		name = "x" + name
	}
	return name, nil
}

func debianVersion(version string) (string, error) {
	if version == "" || version[0] < '0' || version[0] > '9' {
		return "", fmt.Errorf("bundle: version %q is not a Debian version", version)
	}
	for _, r := range version {
		switch {
		case r >= '0' && r <= '9', r >= 'A' && r <= 'Z', r >= 'a' && r <= 'z', r == '.', r == '+', r == '~', r == '-':
		default:
			return "", fmt.Errorf("bundle: version %q is not a Debian version", version)
		}
	}
	return version, nil
}

func msiVersion(version string) (string, error) {
	parts := strings.Split(version, ".")
	if len(parts) > 3 {
		parts = parts[:3]
	}
	var nums []string
	for _, part := range parts {
		if part == "" {
			return "", fmt.Errorf("bundle: version %q is not an MSI version", version)
		}
		for _, r := range part {
			if r < '0' || r > '9' {
				return "", fmt.Errorf("bundle: version %q is not an MSI version", version)
			}
		}
		nums = append(nums, part)
	}
	for len(nums) < 3 {
		nums = append(nums, "0")
	}
	return strings.Join(nums, "."), nil
}

func binName(product string) string {
	var b strings.Builder
	for _, r := range product {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '-', r == '_', r == '.':
			b.WriteRune(r)
		}
	}
	if b.Len() == 0 {
		return "xgoef"
	}
	return b.String()
}

func segment(name string) string {
	var b strings.Builder
	for _, r := range strings.TrimSpace(name) {
		switch {
		case r == '/' || r == '\\' || r < 0x20 || r == '|' || r == ':' || r == '*' || r == '?' || r == '"' || r == '<' || r == '>':
			b.WriteByte('_')
		default:
			b.WriteRune(r)
		}
	}
	out := strings.Trim(b.String(), " .")
	if out == "" || out == "." || out == ".." {
		return "app"
	}
	return out
}
