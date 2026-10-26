package cli

import (
	"crypto/sha256"
	"fmt"
	"net"
	"net/url"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/zigslice/xgoef/conf"
)

// devPoll is how often dev checks the frontend tree.
var devPoll = 200 * time.Millisecond

// devReadyTimeout is how long dev waits for devUrl to accept connections.
var devReadyTimeout = 30 * time.Second

func serveDev(appDir, outDir, bin string) error {
	cfg, err := conf.Load(filepath.Join(appDir, "xgoef.conf.json"))
	if err != nil {
		return err
	}
	front, err := startBeforeDev(appDir, cfg)
	if err != nil {
		return err
	}
	defer func() {
		if front != nil {
			front.stop()
			_ = front.wait()
		}
	}()
	src, err := frontendDir(appDir)
	if err != nil {
		return err
	}
	sock := devSocket(outDir)
	if err := os.MkdirAll(filepath.Dir(sock), 0o700); err != nil {
		return err
	}
	_ = os.Remove(sock)
	ln, err := net.Listen("unix", sock)
	if err != nil {
		return err
	}
	defer ln.Close()
	hub := &devHub{}
	go hub.serve(ln)

	snap, err := dirSnapshot(src)
	if err != nil {
		return err
	}
	proc, err := startDev(bin, outDir, sock)
	if err != nil {
		return err
	}
	fmt.Printf("running %s\n", bin)
	tick := time.NewTicker(devPoll)
	defer tick.Stop()
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(sig)
	for {
		select {
		case err := <-proc.done:
			return err
		case <-sig:
			proc.stop()
			_ = proc.wait()
			return nil
		case <-tick.C:
			next, err := dirSnapshot(src)
			if err != nil {
				proc.stop()
				_ = proc.wait()
				return err
			}
			if sameSnapshot(snap, next) {
				continue
			}
			snap = next
			if err := refreshFrontend(appDir, outDir); err != nil {
				proc.stop()
				_ = proc.wait()
				return err
			}
			hub.notify()
			fmt.Printf("reloaded %s\n", src)
		}
	}
}

func frontendDir(appDir string) (string, error) {
	cfg, err := conf.Load(filepath.Join(appDir, "xgoef.conf.json"))
	if err != nil {
		return "", err
	}
	dist := cfg.Build.FrontendDist
	if dist == "" {
		dist = "web"
	}
	if !filepath.IsAbs(dist) {
		dist = filepath.Join(appDir, dist)
	}
	st, err := os.Stat(dist)
	if err != nil || !st.IsDir() {
		return "", fmt.Errorf("frontend dist %s: %w", dist, err)
	}
	return dist, nil
}

func refreshFrontend(appDir, outDir string) error {
	src, err := frontendDir(appDir)
	if err != nil {
		return err
	}
	dst := filepath.Join(outDir, "web")
	if err := os.RemoveAll(dst); err != nil {
		return err
	}
	if err := copyTree(src, dst); err != nil {
		return err
	}
	js, err := findJS(appDir)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Join(outDir, "api", "js"), 0o755); err != nil {
		return err
	}
	if err := copyFile(js, filepath.Join(outDir, "api", "js", "xgoef.js")); err != nil {
		return err
	}
	if err := copyFile(js, filepath.Join(dst, "xgoef.js")); err != nil {
		return err
	}
	if err := rewriteBridgeSrc(filepath.Join(dst, "index.html")); err != nil && !os.IsNotExist(err) {
		return err
	}
	cfg, err := conf.Load(filepath.Join(appDir, "xgoef.conf.json"))
	if err != nil {
		return err
	}
	return writeBindings(cfg, outDir)
}

func devSocket(outDir string) string {
	sock := filepath.Join(outDir, ".xgoef", "dev.sock")
	if len(sock) < 100 {
		return sock
	}
	return filepath.Join(os.TempDir(), fmt.Sprintf("xgoef-dev-%d.sock", os.Getpid()))
}

func dirSnapshot(root string) (map[string][32]byte, error) {
	out := map[string][32]byte{}
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !d.Type().IsRegular() {
			return nil
		}
		body, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		out[rel] = sha256.Sum256(body)
		return nil
	})
	return out, err
}

func sameSnapshot(a, b map[string][32]byte) bool {
	if len(a) != len(b) {
		return false
	}
	for k, v := range a {
		if b[k] != v {
			return false
		}
	}
	return true
}

type devProc struct {
	cmd  *exec.Cmd
	done chan error
}

func startDev(bin, dir, sock string) (*devProc, error) {
	cmd := exec.Command(bin)
	cmd.Dir = dir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = append(os.Environ(), "XGOEF_DEV_SOCK="+sock)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	p := &devProc{cmd: cmd, done: make(chan error, 1)}
	go func() {
		p.done <- cmd.Wait()
	}()
	return p, nil
}

func (p *devProc) stop() {
	if p == nil || p.cmd == nil || p.cmd.Process == nil {
		return
	}
	_ = syscall.Kill(-p.cmd.Process.Pid, syscall.SIGTERM)
}

func (p *devProc) wait() error {
	if p == nil {
		return nil
	}
	return <-p.done
}

func runBeforeBuild(appDir string) error {
	cfg, err := conf.Load(filepath.Join(appDir, "xgoef.conf.json"))
	if err != nil {
		return err
	}
	return runShell(appDir, cfg.Build.BeforeBuildCommand)
}

func runShell(dir, command string) error {
	command = strings.TrimSpace(command)
	if command == "" {
		return nil
	}
	cmd := exec.Command("sh", "-c", command)
	cmd.Dir = dir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("build: %s: %w", command, err)
	}
	return nil
}

type shellProc struct {
	cmd  *exec.Cmd
	done chan struct{}
	err  error
}

func startShell(dir, command string) (*shellProc, error) {
	cmd := exec.Command("sh", "-c", command)
	cmd.Dir = dir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	p := &shellProc{cmd: cmd, done: make(chan struct{})}
	go func() {
		p.err = cmd.Wait()
		close(p.done)
	}()
	return p, nil
}

func (p *shellProc) stop() {
	if p == nil || p.cmd == nil || p.cmd.Process == nil {
		return
	}
	_ = syscall.Kill(-p.cmd.Process.Pid, syscall.SIGTERM)
}

func (p *shellProc) wait() error {
	if p == nil {
		return nil
	}
	<-p.done
	return p.err
}

func (p *shellProc) exited() (bool, error) {
	if p == nil {
		return true, nil
	}
	select {
	case <-p.done:
		return true, p.err
	default:
		return false, nil
	}
}

func startBeforeDev(appDir string, cfg conf.File) (*shellProc, error) {
	command := strings.TrimSpace(cfg.Build.BeforeDevCommand)
	if command == "" {
		return nil, nil
	}
	proc, err := startShell(appDir, command)
	if err != nil {
		return nil, fmt.Errorf("dev: %s: %w", command, err)
	}
	if err := waitDevURL(cfg.Build.DevURL, proc); err != nil {
		proc.stop()
		_ = proc.wait()
		return nil, err
	}
	return proc, nil
}

func waitDevURL(raw string, proc *shellProc) error {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	addr, err := devAddr(raw)
	if err != nil {
		return err
	}
	deadline := time.Now().Add(devReadyTimeout)
	ticker := time.NewTicker(50 * time.Millisecond)
	defer ticker.Stop()
	for {
		conn, err := net.DialTimeout("tcp", addr, 200*time.Millisecond)
		if err == nil {
			_ = conn.Close()
			return nil
		}
		if done, exitErr := proc.exited(); done {
			if exitErr == nil {
				return fmt.Errorf("dev: frontend exited before %s", raw)
			}
			return fmt.Errorf("dev: frontend exited before %s: %w", raw, exitErr)
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("dev: frontend did not listen on %s", addr)
		}
		<-ticker.C
	}
}

func devAddr(raw string) (string, error) {
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" || (u.Scheme != "http" && u.Scheme != "https") {
		return "", fmt.Errorf("dev: devUrl %q is not an http(s) URL", raw)
	}
	if u.Port() != "" {
		return u.Host, nil
	}
	port := "80"
	if u.Scheme == "https" {
		port = "443"
	}
	return net.JoinHostPort(u.Hostname(), port), nil
}

type devHub struct {
	mu    sync.Mutex
	conns []net.Conn
}

func (h *devHub) serve(ln net.Listener) {
	for {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		h.mu.Lock()
		h.conns = append(h.conns, conn)
		h.mu.Unlock()
	}
}

func (h *devHub) notify() {
	h.mu.Lock()
	defer h.mu.Unlock()
	live := h.conns[:0]
	for _, conn := range h.conns {
		if _, err := conn.Write([]byte("reload\n")); err != nil {
			_ = conn.Close()
			continue
		}
		live = append(live, conn)
	}
	h.conns = live
}
