package plugins

import (
	"strings"
	"testing"

	"github.com/zigslice/xgoef/plugins/autostart"
)

func TestOpenAutostart(t *testing.T) {
	if _, err := Open("autostart", Options{AppName: "Hello"}); err == nil || !strings.Contains(err.Error(), "identifier is required") {
		t.Fatal(err)
	}
	if _, err := Open("nope", Options{}); err == nil || !strings.Contains(err.Error(), "unknown builtin") {
		t.Fatal(err)
	}
	p, err := Open("autostart", Options{
		Identifier:    "com.example.hello",
		AppName:       "Hello",
		Executable:    "/usr/bin/xgoef-hello",
		AutostartArgs: []string{"--flag"},
	})
	if err != nil {
		t.Fatal(err)
	}
	got, ok := p.(*autostart.Plugin)
	if !ok || got.Name() != "autostart" {
		t.Fatalf("%T %v", p, got)
	}
}
