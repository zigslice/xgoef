package ipc

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"
)

func TestRouterInvoke(t *testing.T) {
	r := NewRouter()
	r.Handle("greet", func(ctx Context, argsJSON []byte) (any, error) {
		var args struct {
			Name string `json:"name"`
		}
		if err := json.Unmarshal(argsJSON, &args); err != nil {
			return nil, err
		}
		return "hi " + args.Name, nil
	})

	out, err := r.Invoke("greet", []byte(`{"name":"xgo"}`))
	if err != nil {
		t.Fatal(err)
	}
	var s string
	if err := json.Unmarshal(out, &s); err != nil {
		t.Fatal(err)
	}
	if s != "hi xgo" {
		t.Fatalf("got %q", s)
	}
}

func TestRouterUnknown(t *testing.T) {
	r := NewRouter()
	_, err := r.Invoke("nope", nil)
	if err == nil {
		t.Fatal("expected error")
	}
	code, msg := Describe(err)
	if code != CodeUnknownCommand || msg != `ipc: unknown command "nope"` {
		t.Fatalf("%s %s", code, msg)
	}
}

func TestInvokeContextCancel(t *testing.T) {
	r := NewRouter()
	started := make(chan struct{})
	r.Handle("slow", func(ctx Context, _ []byte) (any, error) {
		if ctx.Done() == nil {
			return nil, errors.New("missing done")
		}
		close(started)
		<-ctx.Done()
		return nil, ctx.Err()
	})
	ctx, cancel := context.WithCancel(context.Background())
	errc := make(chan error, 1)
	go func() {
		_, err := r.InvokeContext(ctx, "main", "slow", []byte("{}"))
		errc <- err
	}()
	<-started
	cancel()
	select {
	case err := <-errc:
		if !errors.Is(err, ErrCanceled) {
			t.Fatal(err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("handler did not return")
	}
}

func TestInvokeFromContextDoesNotEnd(t *testing.T) {
	r := NewRouter()
	r.Handle("ping", func(ctx Context, _ []byte) (any, error) {
		if ctx.Err() != nil {
			return nil, errors.New("background context ended")
		}
		select {
		case <-ctx.Done():
			return nil, errors.New("background context ended")
		default:
		}
		return "ok", nil
	})
	out, err := r.InvokeFrom("main", "ping", []byte("{}"))
	if err != nil {
		t.Fatal(err)
	}
	if string(out) != `"ok"` {
		t.Fatalf("%s", out)
	}
}
