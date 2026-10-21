package binding

import (
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"
)

// typ is a TypeScript type the generator knows how to print.
type typ interface {
	String() string
}

type named string

func (n named) String() string { return string(n) }

type list struct{ elem typ }

func (l list) String() string { return l.elem.String() + "[]" }

type union struct{ parts []typ }

func (u union) String() string {
	out := make([]string, len(u.parts))
	for i, p := range u.parts {
		out[i] = p.String()
	}
	return strings.Join(out, " | ")
}

type field struct {
	name     string
	optional bool
	typ      typ
}

type object struct{ fields []field }

func (o object) String() string {
	if len(o.fields) == 0 {
		return "{}"
	}
	var b strings.Builder
	b.WriteString("{ ")
	for i, f := range o.fields {
		if i > 0 {
			b.WriteString("; ")
		}
		b.WriteString(f.name)
		if f.optional {
			b.WriteByte('?')
		}
		b.WriteString(": ")
		b.WriteString(f.typ.String())
	}
	b.WriteString(" }")
	return b.String()
}

var baseTypes = map[string]bool{
	"string":       true,
	"number":       true,
	"boolean":      true,
	"null":         true,
	"unknown":      true,
	"XgoefChannel": true,
}

// parseType checks a type expression and returns its canonical form.
// Allowed pieces are string, number, boolean, null, unknown, XgoefChannel,
// T[], A | B, and { name: T; name?: T }.
func parseType(raw string) (typ, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, fmt.Errorf("binding: empty type")
	}
	parts, err := splitTop(raw, '|')
	if err != nil {
		return nil, err
	}
	if len(parts) == 0 || strings.TrimSpace(parts[0]) == "" {
		return nil, fmt.Errorf("binding: empty type")
	}
	parsed := make([]typ, 0, len(parts))
	for _, p := range parts {
		one, err := parsePrimary(p)
		if err != nil {
			return nil, err
		}
		parsed = append(parsed, one)
	}
	if len(parsed) == 1 {
		return parsed[0], nil
	}
	return union{parts: parsed}, nil
}

func parsePrimary(raw string) (typ, error) {
	s := strings.TrimSpace(raw)
	if s == "" {
		return nil, fmt.Errorf("binding: empty type")
	}
	dims := 0
	for strings.HasSuffix(strings.TrimSpace(s), "[]") {
		s = strings.TrimSpace(s)
		s = strings.TrimSpace(s[:len(s)-2])
		dims++
		if s == "" {
			return nil, fmt.Errorf("binding: empty type")
		}
	}
	base, err := parseBase(s)
	if err != nil {
		return nil, err
	}
	for i := 0; i < dims; i++ {
		base = list{elem: base}
	}
	return base, nil
}

func parseBase(raw string) (typ, error) {
	s := strings.TrimSpace(raw)
	if s == "" {
		return nil, fmt.Errorf("binding: empty type")
	}
	if strings.HasPrefix(s, "{") {
		return parseObject(s)
	}
	if !baseTypes[s] {
		return nil, fmt.Errorf("binding: unsupported type %q", s)
	}
	return named(s), nil
}

func parseObject(raw string) (typ, error) {
	s := strings.TrimSpace(raw)
	if !strings.HasPrefix(s, "{") || !strings.HasSuffix(s, "}") {
		return nil, fmt.Errorf("binding: unsupported type %q", raw)
	}
	body := strings.TrimSpace(s[1 : len(s)-1])
	if body == "" {
		return nil, fmt.Errorf("binding: empty object type")
	}
	chunks, err := splitFields(body)
	if err != nil {
		return nil, err
	}
	var fields []field
	seen := map[string]bool{}
	for _, chunk := range chunks {
		chunk = strings.TrimSpace(chunk)
		if chunk == "" {
			continue
		}
		name, optional, rest, err := splitField(chunk)
		if err != nil {
			return nil, err
		}
		if seen[name] {
			return nil, fmt.Errorf("binding: duplicate field %q", name)
		}
		seen[name] = true
		inner, err := parseType(rest)
		if err != nil {
			return nil, err
		}
		fields = append(fields, field{name: name, optional: optional, typ: inner})
	}
	if len(fields) == 0 {
		return nil, fmt.Errorf("binding: empty object type")
	}
	return object{fields: fields}, nil
}

func splitField(chunk string) (name string, optional bool, rest string, err error) {
	colon := strings.IndexByte(chunk, ':')
	if colon <= 0 {
		return "", false, "", fmt.Errorf("binding: field %q needs a type", chunk)
	}
	left := strings.TrimSpace(chunk[:colon])
	rest = strings.TrimSpace(chunk[colon+1:])
	if strings.HasSuffix(left, "?") {
		optional = true
		left = strings.TrimSpace(left[:len(left)-1])
	}
	if !ident(left) {
		return "", false, "", fmt.Errorf("binding: field name %q", left)
	}
	if rest == "" {
		return "", false, "", fmt.Errorf("binding: field %q needs a type", left)
	}
	return left, optional, rest, nil
}

func splitFields(s string) ([]string, error) {
	return splitTopAny(s, func(b byte) bool { return b == ';' || b == ',' })
}

// splitTop splits on sep when it is outside braces.
func splitTop(s string, sep byte) ([]string, error) {
	return splitTopAny(s, func(b byte) bool { return b == sep })
}

func splitTopAny(s string, sep func(byte) bool) ([]string, error) {
	var parts []string
	depth := 0
	start := 0
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case '{':
			depth++
		case '}':
			depth--
			if depth < 0 {
				return nil, fmt.Errorf("binding: unsupported type %q", s)
			}
		default:
			if sep(s[i]) && depth == 0 {
				parts = append(parts, s[start:i])
				start = i + 1
			}
		}
	}
	if depth != 0 {
		return nil, fmt.Errorf("binding: unsupported type %q", s)
	}
	parts = append(parts, s[start:])
	return parts, nil
}

func ident(s string) bool {
	if s == "" {
		return false
	}
	for i, r := range s {
		if r == '_' || unicode.IsLetter(r) {
			continue
		}
		if i > 0 && unicode.IsDigit(r) {
			continue
		}
		return false
	}
	r, _ := utf8.DecodeRuneInString(s)
	return r != utf8.RuneError
}
