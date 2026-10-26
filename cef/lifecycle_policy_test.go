package cef

import (
	"os/exec"
	"path/filepath"
	"testing"
)

func TestPageLoadPolicy(t *testing.T) {
	gxx, err := exec.LookPath("g++")
	if err != nil {
		t.Fatal("g++ is required to test page-load policy")
	}
	bin := filepath.Join(t.TempDir(), "lifecycle")
	cmd := exec.Command(gxx, "-std=c++17", "-Wall", "-Wextra", "-Werror", "-o", bin,
		"shim/xgoef_lifecycle.cc", "shim/xgoef_lifecycle_test.cc")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("compile: %v\n%s", err, out)
	}
	out, err = exec.Command(bin).CombinedOutput()
	if err != nil {
		t.Fatalf("run: %v\n%s", err, out)
	}
}
