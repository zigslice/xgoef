// Package instance keeps one process per application identifier.
// The lock is a unix socket. A later process sends its args to the first
// and leaves that process to focus its window.
package instance

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// ErrRunning means another process holds the identifier and accepted our args.
var ErrRunning = errors.New("instance: already running")

const maxMessage = 64 << 10

// Listener is the primary process. Close it on shutdown.
type Listener struct {
	ln   net.Listener
	path string
	on   func([]string)
}

// Acquire binds the socket for id, or hands args to the process that already holds it.
// dir is the directory for the socket file. The returned listener is nil when ErrRunning is set.
func Acquire(dir, id string, args []string, onArgs func([]string)) (*Listener, error) {
	if strings.TrimSpace(id) == "" {
		return nil, fmt.Errorf("instance: identifier is empty")
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, err
	}
	path, err := socketPath(dir, id)
	if err != nil {
		return nil, err
	}
	return acquire(path, args, onArgs)
}

// Close stops accepting handoffs and removes the socket file.
func (l *Listener) Close() error {
	if l == nil || l.ln == nil {
		return nil
	}
	err := l.ln.Close()
	if rm := os.Remove(l.path); err == nil {
		err = rm
	}
	return err
}

// Path is the socket file. Tests use it to check the file stays inside dir.
func (l *Listener) Path() string {
	if l == nil {
		return ""
	}
	return l.path
}

func acquire(path string, args []string, onArgs func([]string)) (*Listener, error) {
	ln, err := net.Listen("unix", path)
	if err != nil {
		if handoff(path, args) == nil {
			return nil, ErrRunning
		}
		_ = os.Remove(path)
		ln, err = net.Listen("unix", path)
		if err != nil {
			if handoff(path, args) == nil {
				return nil, ErrRunning
			}
			return nil, err
		}
	}
	_ = os.Chmod(path, 0o600)
	l := &Listener{ln: ln, path: path, on: onArgs}
	go l.loop()
	return l, nil
}

func (l *Listener) loop() {
	for {
		conn, err := l.ln.Accept()
		if err != nil {
			return
		}
		l.serve(conn)
	}
}

func (l *Listener) serve(conn net.Conn) {
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(2 * time.Second))
	args, err := readArgs(conn)
	if err != nil {
		_, _ = conn.Write([]byte("bad\n"))
		return
	}
	if l.on != nil {
		l.on(args)
	}
	_, _ = conn.Write([]byte("ok\n"))
}

func handoff(path string, args []string) error {
	conn, err := net.DialTimeout("unix", path, time.Second)
	if err != nil {
		return err
	}
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(2 * time.Second))
	if args == nil {
		args = []string{}
	}
	body, err := json.Marshal(struct {
		Args []string `json:"args"`
	}{Args: args})
	if err != nil {
		return err
	}
	if len(body) > maxMessage {
		return fmt.Errorf("instance: message too long")
	}
	if _, err := conn.Write(append(body, '\n')); err != nil {
		return err
	}
	reply, err := bufio.NewReader(conn).ReadString('\n')
	if err != nil {
		return err
	}
	if strings.TrimSpace(reply) != "ok" {
		return fmt.Errorf("instance: primary rejected the handoff")
	}
	return nil
}

func readArgs(conn net.Conn) ([]string, error) {
	reader := bufio.NewReaderSize(conn, maxMessage+1)
	line, err := reader.ReadSlice('\n')
	if err != nil && len(line) == 0 {
		return nil, err
	}
	if len(line) > maxMessage {
		return nil, fmt.Errorf("instance: message too long")
	}
	var msg struct {
		Args []string `json:"args"`
	}
	if err := json.Unmarshal(line, &msg); err != nil {
		return nil, err
	}
	if msg.Args == nil {
		msg.Args = []string{}
	}
	return append([]string(nil), msg.Args...), nil
}

func socketPath(dir, id string) (string, error) {
	name := sanitize(id)
	path := filepath.Join(dir, "xgoef-"+name+".sock")
	if len(path) > 100 {
		sum := sha256.Sum256([]byte(id))
		path = filepath.Join(dir, "xgoef-"+hex.EncodeToString(sum[:16])+".sock")
	}
	if filepath.Dir(path) != filepath.Clean(dir) {
		return "", fmt.Errorf("instance: socket path left %s", dir)
	}
	return path, nil
}

func sanitize(id string) string {
	var b strings.Builder
	for _, r := range id {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '.', r == '_', r == '-':
			b.WriteRune(r)
		default:
			b.WriteByte('_')
		}
	}
	name := b.String()
	if name == "" || name == "." || name == ".." {
		return "app"
	}
	return name
}
