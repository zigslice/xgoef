package menu

import (
	"strings"
	"testing"
)

func sample() []Item {
	return []Item{{
		ID: "file", Text: "File",
		Items: []Item{
			{ID: "greet", Text: "Greet", Command: "greet"},
			{Separator: true},
			{ID: "quit", Text: "Quit", Command: "app:quit", Disabled: true},
		},
	}}
}

func TestParseActivateAndEncode(t *testing.T) {
	m, err := Parse(sample())
	if err != nil {
		t.Fatal(err)
	}
	cmd, err := m.Activate("greet")
	if err != nil || cmd != "greet" {
		t.Fatalf("%q %v", cmd, err)
	}
	if _, err := m.Activate("file"); err == nil || !strings.Contains(err.Error(), "no command") {
		t.Fatalf("submenu: %v", err)
	}
	if _, err := m.Activate("quit"); err == nil || !strings.Contains(err.Error(), "disabled") {
		t.Fatalf("disabled: %v", err)
	}
	if _, err := m.Activate("missing"); err == nil {
		t.Fatal("missing id accepted")
	}
	enc := m.Encode()
	const want = "file\tFile\t\t\t\n" +
		"greet\tGreet\tgreet\t\tfile\n" +
		"sep-file-1\t\t\tsd\tfile\n" +
		"quit\tQuit\tapp:quit\td\tfile\n"
	if enc != want {
		t.Fatalf("encode:\n%q\nwant:\n%q", enc, want)
	}
}

func TestParseRejectsDuplicate(t *testing.T) {
	_, err := Parse([]Item{
		{ID: "a", Text: "A"},
		{ID: "a", Text: "B"},
	})
	if err == nil || !strings.Contains(err.Error(), "duplicate") {
		t.Fatal(err)
	}
}
