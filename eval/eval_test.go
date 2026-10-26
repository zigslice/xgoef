package eval

import (
	"strings"
	"testing"
)

func TestOriginAllowed(t *testing.T) {
	ok := [][2]string{
		{"xgoef://app/index.html", "xgoef://app/index.html"},
		{"xgoef://app/other.html", "xgoef://app/index.html"},
		{"XGOEF://APP/a#frag", "xgoef://app/b?q=1"},
		{"https://example.test/a", "https://example.test/b"},
		{"https://Example.TEST/a", "https://example.test/b?x=1#y"},
		{"https://example.test/a", "https://example.test:443/b"},
		{"http://example.test:80/a", "http://example.test/b"},
		{"http://127.0.0.1:5173/a", "http://127.0.0.1:5173/"},
		{"http://[::1]/a", "http://[::1]/b"},
		{"http://[::1]:8080/a", "http://[::1]:8080/b"},
	}
	for _, pair := range ok {
		if !OriginAllowed(pair[0], pair[1]) {
			t.Fatalf("want allow %s vs %s", pair[0], pair[1])
		}
	}
	bad := [][2]string{
		{"https://evil.test/", "xgoef://app/index.html"},
		{"xgoef://app.evil/a", "xgoef://app/a"},
		{"xgoef://other/a", "xgoef://other/a"},
		{"xgoef://app:80/a", "xgoef://app/a"},
		{"http://127.0.0.1:5173/", "http://127.0.0.1:5174/"},
		{"http://example.test/", "https://example.test/"},
		{"http://example.test.evil/", "http://example.test/"},
		{"http://user:pass@example.test/", "http://example.test/"},
		{"http://example.test\\@evil.test/", "http://example.test/"},
		{"javascript:alert(1)", "javascript:alert(1)"},
		{"data:text/html,hi", "data:text/html,hi"},
		{"file:///tmp/a", "file:///tmp/a"},
		{"about:blank", "about:blank"},
		{"", "xgoef://app/"},
		{"xgoef://app/", ""},
		{"http://[::1]/", "http://127.0.0.1/"},
		{"http://[::1]:8080/", "http://[::1]/"},
		{"https://example.test/a b", "https://example.test/"},
	}
	for _, pair := range bad {
		if OriginAllowed(pair[0], pair[1]) {
			t.Fatalf("want deny %s vs %s", pair[0], pair[1])
		}
	}
}

func TestEncodeRoundTrip(t *testing.T) {
	scripts := []string{"window.ok = 1;", "a\nb\nc"}
	text, err := Encode(scripts)
	if err != nil {
		t.Fatal(err)
	}
	if strings.IndexByte(text, 0) >= 0 {
		t.Fatal("encoded NUL")
	}
	got, err := Decode(text)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0] != scripts[0] || got[1] != scripts[1] {
		t.Fatalf("%q", got)
	}
	if _, err := Decode(text + "1"); err == nil {
		t.Fatal("trailing bytes accepted")
	}
	if _, err := Decode("0\n"); err != nil {
		t.Fatal(err)
	}
}

func TestRejectsScript(t *testing.T) {
	if err := ValidateSource(""); err == nil {
		t.Fatal("empty")
	}
	if err := ValidateSource(" \n\t"); err == nil {
		t.Fatal("blank")
	}
	if err := ValidateSource("ok\x00"); err == nil {
		t.Fatal("nul")
	}
	if err := ValidateSource(strings.Repeat("a", MaxScriptBytes+1)); err == nil {
		t.Fatal("long")
	}
	many := make([]string, MaxInitScripts+1)
	for i := range many {
		many[i] = "window.ok = 1;"
	}
	if _, err := Encode(many); err == nil {
		t.Fatal("count")
	}
	if err := ValidateName("Mark"); err == nil {
		t.Fatal("upper")
	}
	if err := ValidateName("window.ok"); err == nil {
		t.Fatal("source-like name")
	}
	if err := ValidateName("a"); err != nil {
		t.Fatal(err)
	}
	if err := ValidateNamed(map[string]string{"mark": "window.ok = 1;", "bad name": "window.ok = 1;"}); err == nil {
		t.Fatal("bad name in map")
	}
}
