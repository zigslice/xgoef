package instance

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSecondProcessHandsOff(t *testing.T) {
	dir := t.TempDir()
	got := make(chan []string, 1)
	first, err := Acquire(dir, "com.example.hello", nil, func(args []string) {
		got <- args
	})
	if err != nil {
		t.Fatal(err)
	}
	defer first.Close()
	if !strings.HasPrefix(first.Path(), dir+string(os.PathSeparator)) {
		t.Fatalf("path %s", first.Path())
	}
	_, err = Acquire(dir, "com.example.hello", []string{"bin", "again"}, nil)
	if !errors.Is(err, ErrRunning) {
		t.Fatal(err)
	}
	args := <-got
	if len(args) != 2 || args[1] != "again" {
		t.Fatalf("%q", args)
	}
	other, err := Acquire(dir, "com.example.other", nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	other.Close()
}

func TestStaleSocketIsReplaced(t *testing.T) {
	dir := t.TempDir()
	id := "../not-a-dir"
	path, err := socketPath(dir, id)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("stale"), 0o600); err != nil {
		t.Fatal(err)
	}
	first, err := Acquire(dir, id, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer first.Close()
	if filepath.Dir(first.Path()) != dir {
		t.Fatalf("escaped to %s", first.Path())
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatal(err)
	}
}
