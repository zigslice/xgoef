package remote

import (
	"strings"
	"testing"
)

func TestOrigin(t *testing.T) {
	cases := []struct {
		raw  string
		want string
		ok   bool
	}{
		{"https://Example.COM:443/a", "https://example.com", true},
		{"http://example.com:80/a", "http://example.com", true},
		{"http://example.com:0080/a", "http://example.com", true},
		{"http://example.com:8080/a", "http://example.com:8080", true},
		{"https://example.com:8443", "https://example.com:8443", true},
		{"http://user:pass@example.com/a", "http://example.com", true},
		{"http://[::1]:80/a", "http://[::1]", true},
		{"http://[::A]:8080/a", "http://[::a]:8080", true},
		{"http://127.0.0.1:5173/index.html?x=1#y", "http://127.0.0.1:5173", true},
		{"http://example.com:65536/", "", false},
		{"http://example.com:/", "", false},
		{"http://[::1]:/", "", false},
		{"", "", false},
		{"xgoef://app/index.html", "", false},
		{"about:blank", "", false},
		{"https://example.com/a b", "https://example.com", true},
	}
	for _, tc := range cases {
		got, ok := Origin(tc.raw)
		if ok != tc.ok || got != tc.want {
			t.Fatalf("%q → %q %v, want %q %v", tc.raw, got, ok, tc.want, tc.ok)
		}
	}
}

func TestCanonical(t *testing.T) {
	got, err := Canonical("https://Example.COM:443/")
	if err != nil || got != "https://example.com" {
		t.Fatalf("%q %v", got, err)
	}
	got, err = Canonical("http://[::1]:8080")
	if err != nil || got != "http://[::1]:8080" {
		t.Fatalf("ipv6 %q %v", got, err)
	}
	for _, raw := range []string{
		"",
		"https://example.com/app",
		"https://example.com?x=1",
		"https://example.com#x",
		"http://user@example.com",
		"javascript:alert(1)",
		"http://example.com\n",
		"http://127.0.0.1:5173 ",
		"https://" + strings.Repeat("a", 250) + ".com",
		"http://example.com:0/",
		"http://.example.com",
		"http://[gggg]",
	} {
		if _, err := Canonical(raw); err == nil || !strings.Contains(err.Error(), "url is invalid") {
			t.Fatalf("%q %v", raw, err)
		}
	}
}

func TestNormalize(t *testing.T) {
	got, err := Normalize([]string{"https://example.com/", "http://127.0.0.1:5173"})
	if err != nil || len(got) != 2 || got[0] != "https://example.com" || got[1] != "http://127.0.0.1:5173" {
		t.Fatalf("%q %v", got, err)
	}
	if _, err := Normalize([]string{"https://example.com", "https://example.com:443/"}); err == nil || !strings.Contains(err.Error(), "duplicate") {
		t.Fatal(err)
	}
	many := make([]string, MaxURLs+1)
	for i := range many {
		many[i] = "https://e" + strconvI(i) + ".example"
	}
	if _, err := Normalize(many); err == nil || !strings.Contains(err.Error(), "too many") {
		t.Fatal(err)
	}
	if !Allowed("https://example.com/path", got) || Allowed("http://example.com/", got) || Allowed("http://127.0.0.1/", got) {
		t.Fatal("allow mismatch")
	}
}

func strconvI(n int) string {
	if n == 0 {
		return "0"
	}
	var b [4]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	return string(b[i:])
}
