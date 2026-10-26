package autostart

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestFileNameAndArgs(t *testing.T) {
	got, err := FileName("com.example.hello")
	if err != nil || got != "xgoef-com.example.hello.desktop" {
		t.Fatalf("%s %v", got, err)
	}
	got, err = FileName("a b")
	if err != nil || got != "xgoef-a_b.desktop" {
		t.Fatalf("%s %v", got, err)
	}
	for _, bad := range []string{"", " ", ".", "..", "  .  "} {
		if _, err := FileName(bad); err == nil {
			t.Fatalf("accepted %q", bad)
		}
	}
	if err := ValidateArgs([]string{"--ok", "a b"}); err != nil {
		t.Fatal(err)
	}
	long := strings.Repeat("a", MaxArgLen)
	if err := ValidateArgs([]string{long}); err != nil {
		t.Fatal(err)
	}
	many := make([]string, MaxArgs+1)
	for i := range many {
		many[i] = "a"
	}
	for _, bad := range [][]string{
		many,
		{""},
		{"   "},
		{long + "a"},
		{"line\n"},
		{"tab\t"},
		{"nul\x00"},
	} {
		err := ValidateArgs(bad)
		if err == nil || codeOf(err) != "invalid" {
			t.Fatalf("%q %v", bad, err)
		}
	}
}

func TestEntryQuotesTokens(t *testing.T) {
	body, err := Entry("Hello", "/opt/xgoef/hello", []string{"--flag=$HOME", "a b"})
	if err != nil {
		t.Fatal(err)
	}
	want := "[Desktop Entry]\nVersion=1.0\nType=Application\nName=Hello\nExec=\"/opt/xgoef/hello\" \"--flag=\\$HOME\" \"a b\"\nX-GNOME-Autostart-enabled=true\n"
	if body != want {
		t.Fatalf("%q", body)
	}
	special, err := Entry("Hello", `/opt/my app/bin`, []string{"a\"b\\c`d"})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(special, "Exec=\"/opt/my app/bin\" \"a\\\"b\\\\c\\`d\"\n") {
		t.Fatalf("%q", special)
	}
	if _, err := Entry(" ", "/opt/xgoef/hello", nil); err == nil || codeOf(err) != "invalid" {
		t.Fatal(err)
	}
	if _, err := Entry("Bad\nName", "/opt/xgoef/hello", nil); err == nil || codeOf(err) != "invalid" {
		t.Fatal(err)
	}
	if _, err := Entry("Hello", "relative", nil); err == nil || codeOf(err) != "invalid" {
		t.Fatal(err)
	}
	if _, err := Entry("Hello", "/opt/xgoef/hello\n", nil); err == nil || codeOf(err) != "invalid" {
		t.Fatal(err)
	}
}

func TestEnableDisableAndEnabled(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "autostart")
	exec := filepath.Join(t.TempDir(), "xgoef-hello")
	path, err := Enable(dir, "com.example.hello", "Hello", exec, []string{"--flag"})
	if err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil || info.Mode().Perm() != 0o644 {
		t.Fatalf("%v %v", info, err)
	}
	again, err := Enable(dir, "com.example.hello", "Hello", exec, []string{"--flag"})
	if err != nil || again != path {
		t.Fatalf("%s %v", again, err)
	}
	entries, err := os.ReadDir(dir)
	if err != nil || len(entries) != 1 || entries[0].Name() != "xgoef-com.example.hello.desktop" {
		t.Fatalf("%v %v", entries, err)
	}
	on, err := Enabled(dir, "com.example.hello")
	if err != nil || !on {
		t.Fatalf("%v %v", on, err)
	}
	body, err := os.ReadFile(path)
	if err != nil || !bytes.Contains(body, []byte(`Exec="`+exec+`" "--flag"`)) {
		t.Fatalf("%s %v", body, err)
	}
	if err := Disable(dir, "com.example.hello"); err != nil {
		t.Fatal(err)
	}
	on, err = Enabled(dir, "com.example.hello")
	if err != nil || on {
		t.Fatalf("after disable %v %v", on, err)
	}
	if err := Disable(dir, "com.example.hello"); err != nil {
		t.Fatal(err)
	}
	if _, err := Enabled(dir, "missing"); err != nil {
		t.Fatal(err)
	}
}

func TestSymlinkAndDirectory(t *testing.T) {
	dir := t.TempDir()
	exec := filepath.Join(t.TempDir(), "xgoef-hello")
	file, err := FileName("com.example.hello")
	if err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(t.TempDir(), "secret")
	if err := os.WriteFile(target, []byte("SECRET"), 0o644); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(dir, file)
	if err := os.Symlink(target, link); err != nil {
		t.Fatal(err)
	}
	if _, err := Enable(dir, "com.example.hello", "Hello", exec, nil); err == nil || codeOf(err) != "denied" {
		t.Fatal(err)
	}
	got, err := os.ReadFile(target)
	if err != nil || string(got) != "SECRET" {
		t.Fatalf("%s %v", got, err)
	}
	if _, err := os.Lstat(link); err != nil {
		t.Fatal(err)
	}
	on, err := Enabled(dir, "com.example.hello")
	if on || err == nil || codeOf(err) != "denied" || strings.Contains(err.Error(), "SECRET") {
		t.Fatalf("%v %v", on, err)
	}
	if err := Disable(dir, "com.example.hello"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Lstat(link); !os.IsNotExist(err) {
		t.Fatal(err)
	}
	got, err = os.ReadFile(target)
	if err != nil || string(got) != "SECRET" {
		t.Fatalf("target %s %v", got, err)
	}

	if err := os.Mkdir(link, 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := Enable(dir, "com.example.hello", "Hello", exec, nil); err == nil || codeOf(err) != "denied" {
		t.Fatal(err)
	}
	if err := Disable(dir, "com.example.hello"); err == nil || codeOf(err) != "denied" {
		t.Fatal(err)
	}
	if info, err := os.Stat(link); err != nil || !info.IsDir() {
		t.Fatalf("%v %v", info, err)
	}
}

func TestEnabledReadsFirstDesktopGroup(t *testing.T) {
	dir := t.TempDir()
	file, err := FileName("com.example.hello")
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, file)
	cases := []struct {
		body string
		on   bool
	}{
		{"[Desktop Entry]\nHidden=true\n", false},
		{"[Desktop Entry]\nX-GNOME-Autostart-enabled=false\n", false},
		{"[Desktop Entry]\nHidden=True\nX-GNOME-Autostart-enabled=true\n", true},
		{"Hidden=true\n[Desktop Entry]\nName=App\n[Desktop Entry]\nHidden=true\n", true},
		{"[Other]\nHidden=true\n\n[Desktop Entry]\nName=App\n", true},
		{"[Desktop Entry]\n# Hidden=true\nName=App\n", true},
		{"", true},
	}
	for _, tc := range cases {
		if err := os.WriteFile(path, []byte(tc.body), 0o644); err != nil {
			t.Fatal(err)
		}
		on, err := Enabled(dir, "com.example.hello")
		if err != nil || on != tc.on {
			t.Fatalf("%q on %v %v", tc.body, on, err)
		}
	}
	if err := os.WriteFile(path, bytes.Repeat([]byte("a"), 64*1024), 0o644); err != nil {
		t.Fatal(err)
	}
	on, err := Enabled(dir, "com.example.hello")
	if err != nil || !on {
		t.Fatalf("limit %v %v", on, err)
	}
	if err := os.WriteFile(path, bytes.Repeat([]byte("a"), 64*1024+1), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Enabled(dir, "com.example.hello"); err == nil || codeOf(err) != "invalid" {
		t.Fatal(err)
	}
}

func TestUserDir(t *testing.T) {
	got, err := userDir("linux", "/tmp/cfg", "/home/me")
	if err != nil || got != "/tmp/cfg/autostart" {
		t.Fatalf("%s %v", got, err)
	}
	got, err = userDir("linux", "", "/home/me")
	if err != nil || got != "/home/me/.config/autostart" {
		t.Fatalf("%s %v", got, err)
	}
	for _, tc := range []struct{ goos, xdg, home string }{
		{"darwin", "/tmp/cfg", "/home/me"},
		{"windows", "", `C:\Users\me`},
		{"linux", "relative/config", "/home/me"},
		{"linux", "", "relative"},
		{"linux", "", ""},
	} {
		if _, err := userDir(tc.goos, tc.xdg, tc.home); err == nil || codeOf(err) != "unavailable" {
			t.Fatalf("%+v %v", tc, err)
		}
	}
	if _, err := Enable("relative", "com.example.hello", "Hello", "/opt/xgoef/hello", nil); err == nil || codeOf(err) != "unavailable" {
		t.Fatal(err)
	}
}

func codeOf(err error) string {
	var ae *Error
	if errors.As(err, &ae) {
		return ae.Code()
	}
	return ""
}
