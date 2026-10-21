package bundle

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestSquashFSRoundTrip(t *testing.T) {
	img, err := writeSquashFS(
		map[string][]byte{
			"usr/bin/hello":        []byte("#!/bin/sh\necho hi\n"),
			"usr/share/doc/readme": []byte("docs"),
		},
		map[string]uint16{"usr/bin/hello": 0o755},
		map[string]string{"link": "usr/bin/hello"},
	)
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	src := filepath.Join(dir, "image.sqsh")
	if err := os.WriteFile(src, img, 0o644); err != nil {
		t.Fatal(err)
	}
	dest := filepath.Join(dir, "out")
	out, err := exec.Command("unsquashfs", "-d", dest, src).CombinedOutput()
	if err != nil {
		t.Fatalf("%v\n%s", err, out)
	}
	got, err := os.ReadFile(filepath.Join(dest, "usr", "share", "doc", "readme"))
	if err != nil {
		t.Fatalf("%v\n%s", err, out)
	}
	if string(got) != "docs" {
		t.Fatalf("%q", got)
	}
	info, err := os.Lstat(filepath.Join(dest, "link"))
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode()&os.ModeSymlink == 0 {
		t.Fatalf("link mode %s", info.Mode())
	}
	target, err := os.Readlink(filepath.Join(dest, "link"))
	if err != nil {
		t.Fatal(err)
	}
	if target != "usr/bin/hello" {
		t.Fatalf("%s", target)
	}
	bin, err := os.Stat(filepath.Join(dest, "usr", "bin", "hello"))
	if err != nil {
		t.Fatal(err)
	}
	if bin.Mode().Perm() != 0o755 {
		t.Fatalf("%o", bin.Mode().Perm())
	}
}
