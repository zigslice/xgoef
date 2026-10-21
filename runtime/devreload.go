package runtime

import (
	"bufio"
	"context"
	"net"
	"os"
	"strings"
	"time"
)

// serveDevReload dials XGOEF_DEV_SOCK and emits dev-reload when the dev
// process writes a reload line. An empty variable leaves the app alone.
func (a *App) serveDevReload(ctx context.Context) {
	sock := strings.TrimSpace(os.Getenv("XGOEF_DEV_SOCK"))
	if sock == "" || a == nil || a.Events == nil {
		return
	}
	var conn net.Conn
	var err error
	for {
		conn, err = net.Dial("unix", sock)
		if err == nil {
			break
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(50 * time.Millisecond):
		}
	}
	defer conn.Close()
	go func() {
		<-ctx.Done()
		_ = conn.Close()
	}()
	sc := bufio.NewScanner(conn)
	for sc.Scan() {
		if strings.TrimSpace(sc.Text()) != "reload" {
			continue
		}
		_ = a.Emit("dev-reload", map[string]any{})
	}
}
