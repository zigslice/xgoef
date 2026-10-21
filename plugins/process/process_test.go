package process

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/zigslice/xgoef/ipc"
	"github.com/zigslice/xgoef/plugin"
)

type host struct{ r *ipc.Router }

func (h host) Register(name string, fn ipc.Handler) { h.r.Handle(name, fn) }

func TestExitCodes(t *testing.T) {
	var got []int
	p := New(Spec{Exit: func(code int) { got = append(got, code) }})
	r := ipc.NewRouter()
	if err := p.Setup(host{r: r}); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		args string
		code int
	}{
		{`{}`, 0},
		{``, 0},
		{`null`, 0},
		{`{"code":null}`, 0},
		{`{"code":0}`, 0},
		{`{"code":7}`, 7},
		{`{"code":255}`, 255},
	} {
		got = nil
		if _, err := r.Invoke("process:exit", []byte(tc.args)); err != nil {
			t.Fatalf("%s: %v", tc.args, err)
		}
		if len(got) != 1 || got[0] != tc.code {
			t.Fatalf("%s -> %v", tc.args, got)
		}
	}
	got = nil
	for _, args := range []string{`{"code":-1}`, `{"code":1.5}`, `{"code":256}`, `{"code":"1"}`, `{"code":7,"path":"/bin/sh"}`, `{"path":"/bin/sh"}`, `[]`} {
		if _, err := r.Invoke("process:exit", []byte(args)); err == nil || ipc.CodeOf(err) != "process.invalid" {
			t.Fatalf("%s: %v", args, err)
		}
	}
	if len(got) != 0 {
		t.Fatalf("invalid exit ran: %v", got)
	}
}

func TestRelaunchRejectsPageArgsAndMissingExecutable(t *testing.T) {
	released := false
	started := false
	exited := false
	p := New(Spec{
		Executable: filepath.Join(t.TempDir(), "missing"),
		Args:       []string{"app"},
		Release:    func() { released = true },
		Exit:       func(int) { exited = true },
		Start: func(string, []string) error {
			started = true
			return nil
		},
	})
	r := ipc.NewRouter()
	if err := p.Setup(host{r: r}); err != nil {
		t.Fatal(err)
	}
	if _, err := r.Invoke("process:relaunch", []byte(`{"exe":"/bin/sh"}`)); err == nil || ipc.CodeOf(err) != "process.invalid" || released || started || exited {
		t.Fatal(err)
	}
	_, err := r.Invoke("process:relaunch", []byte(`{}`))
	if err == nil || ipc.CodeOf(err) != "process.relaunch" || released || started || exited {
		t.Fatalf("missing exe %v released %v", err, released)
	}
}

func TestRelaunchOrderAndFilteredArgs(t *testing.T) {
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	var order []string
	var argv []string
	p := New(Spec{
		Executable: exe,
		Args:       []string{"old", "--url=https://example.test", "--type=renderer", "--type", "gpu-process", "--utility-sub-type=network", "keep"},
		Release:    func() { order = append(order, "release") },
		Exit:       func(code int) { order = append(order, "exit") },
		Start: func(gotExe string, gotArgv []string) error {
			if gotExe != exe && gotExe != absPath(exe) {
				t.Fatalf("exe %s", gotExe)
			}
			order = append(order, "start")
			argv = append([]string(nil), gotArgv...)
			return nil
		},
	})
	r := ipc.NewRouter()
	if err := p.Setup(host{r: r}); err != nil {
		t.Fatal(err)
	}
	if _, err := r.Invoke("process:relaunch", []byte(`{}`)); err != nil {
		t.Fatal(err)
	}
	if strings.Join(order, ",") != "release,start,exit" {
		t.Fatalf("order %v", order)
	}
	if len(argv) != 3 || argv[0] == "old" || argv[1] != "--url=https://example.test" || argv[2] != "keep" {
		t.Fatalf("argv %q", argv)
	}
	if strings.Contains(strings.Join(argv, "\n"), "--type") || strings.Contains(strings.Join(argv, "\n"), "utility") {
		t.Fatalf("cef switch kept: %q", argv)
	}
}

func TestStartFailureDoesNotExit(t *testing.T) {
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	released := false
	exited := false
	p := New(Spec{
		Executable: exe,
		Args:       []string{"app"},
		Release:    func() { released = true },
		Exit:       func(int) { exited = true },
		Start:      func(string, []string) error { return errors.New("boom") },
	})
	r := ipc.NewRouter()
	if err := p.Setup(host{r: r}); err != nil {
		t.Fatal(err)
	}
	_, err = r.Invoke("process:relaunch", []byte(`{}`))
	if err == nil || ipc.CodeOf(err) != "process.relaunch" || !strings.Contains(err.Error(), "boom") || !released || exited {
		t.Fatalf("err %v released %v exited %v", err, released, exited)
	}
}

func TestDefaultStart(t *testing.T) {
	dir := t.TempDir()
	out := filepath.Join(dir, "out")
	script := filepath.Join(dir, "run.sh")
	body := "#!/bin/sh\nprintf '%s\\n' \"$0\" \"$@\" > '" + out + "'\n"
	if err := os.WriteFile(script, []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
	p := New(Spec{
		Executable: script,
		Args:       []string{script, "--url=ok", "--type=renderer", "keep"},
		Exit:       func(int) {},
	})
	r := ipc.NewRouter()
	if err := p.Setup(host{r: r}); err != nil {
		t.Fatal(err)
	}
	if _, err := r.Invoke("process:relaunch", []byte(`{}`)); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(2 * time.Second)
	var text string
	for {
		b, err := os.ReadFile(out)
		if err == nil {
			text = string(b)
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("script output: %v", err)
		}
		time.Sleep(10 * time.Millisecond)
	}
	lines := strings.Split(strings.TrimSpace(text), "\n")
	if len(lines) != 3 || !strings.HasSuffix(lines[0], "run.sh") || lines[1] != "--url=ok" || lines[2] != "keep" {
		t.Fatalf("lines %q", lines)
	}
}

func TestRelaunchArgs(t *testing.T) {
	got := relaunchArgs("/bin/app", nil)
	if len(got) != 1 || got[0] != "/bin/app" {
		t.Fatalf("%q", got)
	}
	got = relaunchArgs("/bin/app", []string{"name", "--utility-sub-type", "network.mojom.NetworkService", "--type", "zygote", "pos"})
	if strings.Join(got, " ") != "/bin/app pos" {
		t.Fatalf("%q", got)
	}
}

func absPath(path string) string {
	abs, err := filepath.Abs(path)
	if err != nil {
		return path
	}
	return abs
}

// host must implement plugin.Host. The blank assignment keeps the import honest
// if Setup's signature changes.
var _ plugin.Host = host{}
