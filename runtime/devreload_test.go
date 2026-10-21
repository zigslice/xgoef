package runtime

import (
	"context"
	"net"
	"path/filepath"
	"testing"
	"time"

	"github.com/zigslice/xgoef/conf"
)

func TestDevReloadEmits(t *testing.T) {
	dir := t.TempDir()
	sock := filepath.Join(dir, "dev.sock")
	ln, err := net.Listen("unix", sock)
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	t.Setenv("XGOEF_DEV_SOCK", sock)
	app := New(conf.File{}, nil)
	got := make(chan struct{}, 1)
	app.Events.Listen("dev-reload", func(any) {
		select {
		case got <- struct{}{}:
		default:
		}
	})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go app.serveDevReload(ctx)
	conn, err := ln.Accept()
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	if _, err := conn.Write([]byte("reload\n")); err != nil {
		t.Fatal(err)
	}
	select {
	case <-got:
	case <-time.After(2 * time.Second):
		t.Fatal("dev-reload was not emitted")
	}
}

func TestDevReloadIgnoresEmptyEnv(t *testing.T) {
	t.Setenv("XGOEF_DEV_SOCK", "")
	app := New(conf.File{}, nil)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	done := make(chan struct{})
	go func() {
		app.serveDevReload(ctx)
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("serveDevReload blocked")
	}
}
