package binding

import (
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"
)

// Arg is one field of a command argument object.
type Arg struct {
	Name     string `json:"name"`
	Type     string `json:"type"`
	Optional bool   `json:"optional,omitempty"`
}

// Command is the shape of one invoke. An empty Result means null.
type Command struct {
	Name   string `json:"name"`
	Doc    string `json:"doc,omitempty"`
	Args   []Arg  `json:"args,omitempty"`
	Result string `json:"result,omitempty"`
}

// Normalize checks a command and rewrites types into their canonical form.
func Normalize(c Command) (Command, error) {
	name := strings.TrimSpace(c.Name)
	if !commandName(name) {
		return Command{}, fmt.Errorf("binding: command name %q", c.Name)
	}
	doc := strings.TrimSpace(c.Doc)
	if strings.Contains(doc, "\n") || strings.Contains(doc, "*/") {
		return Command{}, fmt.Errorf("binding: command %s: doc must be one line", name)
	}
	seen := map[string]bool{}
	args := make([]Arg, len(c.Args))
	for i, arg := range c.Args {
		argName := strings.TrimSpace(arg.Name)
		if !ident(argName) {
			return Command{}, fmt.Errorf("binding: command %s: arg name %q", name, arg.Name)
		}
		if seen[argName] {
			return Command{}, fmt.Errorf("binding: command %s: duplicate arg %q", name, argName)
		}
		seen[argName] = true
		parsed, err := parseType(arg.Type)
		if err != nil {
			return Command{}, fmt.Errorf("binding: command %s: arg %s: %w", name, argName, err)
		}
		args[i] = Arg{Name: argName, Type: parsed.String(), Optional: arg.Optional}
	}
	result := strings.TrimSpace(c.Result)
	if result == "" {
		result = "null"
	}
	parsed, err := parseType(result)
	if err != nil {
		return Command{}, fmt.Errorf("binding: command %s: result: %w", name, err)
	}
	return Command{Name: name, Doc: doc, Args: args, Result: parsed.String()}, nil
}

func commandName(name string) bool {
	parts := strings.Split(name, ":")
	if len(parts) != 1 && len(parts) != 2 {
		return false
	}
	for _, p := range parts {
		if !commandPart(p) {
			return false
		}
	}
	return true
}

// commandPart is an identifier that may contain a hyphen, so plugin ids
// such as global-shortcut can be command prefixes. A hyphen cannot sit
// at either end or next to another hyphen.
func commandPart(s string) bool {
	if s == "" || strings.HasPrefix(s, "-") || strings.HasSuffix(s, "-") || strings.Contains(s, "--") {
		return false
	}
	for i, r := range s {
		if r == '_' || r == '-' || unicode.IsLetter(r) {
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

// Assemble merges app commands with the builtins selected by plugins.
// menu:popup, tray:popup, and tray:click are always included.
// deeplink:get_current is included when deepLink is set.
func Assemble(app []Command, plugins []string, deepLink bool) ([]Command, error) {
	out := make([]Command, 0, len(app)+8)
	seen := map[string]bool{}
	add := func(c Command) error {
		n, err := Normalize(c)
		if err != nil {
			return err
		}
		if seen[n.Name] {
			return fmt.Errorf("binding: duplicate command %q", n.Name)
		}
		seen[n.Name] = true
		out = append(out, n)
		return nil
	}
	for _, c := range app {
		if err := add(c); err != nil {
			return nil, err
		}
	}
	used := map[string]bool{}
	for _, name := range plugins {
		name = strings.TrimSpace(name)
		if name == "" {
			return nil, fmt.Errorf("binding: empty plugin name")
		}
		if used[name] {
			return nil, fmt.Errorf("binding: plugin %q repeated", name)
		}
		used[name] = true
		cmds, ok := builtinPlugins[name]
		if !ok {
			return nil, fmt.Errorf("binding: unknown plugin %q", name)
		}
		for _, c := range cmds {
			if err := add(c); err != nil {
				return nil, err
			}
		}
	}
	for _, c := range chromeCommands {
		if err := add(c); err != nil {
			return nil, err
		}
	}
	if deepLink {
		if err := add(deepLinkCommand); err != nil {
			return nil, err
		}
	}
	return out, nil
}
