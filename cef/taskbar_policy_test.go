package cef

import (
	"os/exec"
	"path/filepath"
	"testing"
)

func TestTaskbarPolicy(t *testing.T) {
	gxx, err := exec.LookPath("g++")
	if err != nil {
		t.Fatal("g++ is required to test taskbar policy")
	}
	bin := filepath.Join(t.TempDir(), "taskbar")
	cmd := exec.Command(gxx, "-std=c++17", "-Wall", "-Wextra", "-Werror", "-o", bin,
		"shim/xgoef_taskbar.cc", "shim/xgoef_taskbar_x11.cc", "shim/xgoef_taskbar_test.cc", "-lX11")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("compile: %v\n%s", err, out)
	}
	out, err = exec.Command(bin).CombinedOutput()
	if err != nil {
		t.Fatalf("run: %v\n%s", err, out)
	}
}
