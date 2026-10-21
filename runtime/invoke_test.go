package runtime

import (
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/zigslice/xgoef/acl"
	"github.com/zigslice/xgoef/ipc"
)

func allowGuard(cmds ...string) *acl.Guard {
	return acl.NewGuard([]acl.Capability{{
		Identifier:  "main",
		Windows:     []string{"main", "aux"},
		Permissions: cmds,
	}})
}

type replyBox struct {
	mu   sync.Mutex
	body []string
	done chan struct{}
}

func newReplyBox() *replyBox {
	return &replyBox{done: make(chan struct{})}
}

func (b *replyBox) add(s string) {
	b.mu.Lock()
	b.body = append(b.body, s)
	if len(b.body) == 1 {
		close(b.done)
	}
	b.mu.Unlock()
}

func (b *replyBox) wait(t *testing.T) string {
	t.Helper()
	select {
	case <-b.done:
	case <-time.After(2 * time.Second):
		t.Fatal("no response")
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	if len(b.body) != 1 {
		t.Fatalf("responses %d: %v", len(b.body), b.body)
	}
	return b.body[0]
}

func TestDispatchInvokeAnswersOffTheCaller(t *testing.T) {
	r := ipc.NewRouter()
	started := make(chan struct{})
	release := make(chan struct{})
	r.Handle("greet", func(ctx ipc.Context, _ []byte) (any, error) {
		close(started)
		<-release
		return "hello", nil
	})
	box := newReplyBox()
	dispatchFrontend(allowGuard("greet"), r, nil, newInvokeGate(), "main",
		`{"type":"invoke","id":1,"cmd":"greet","args":{}}`, 9, box.add)
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("handler did not start")
	}
	select {
	case <-box.done:
		t.Fatal("response arrived before the handler returned")
	default:
	}
	close(release)
	got := box.wait(t)
	if !strings.Contains(got, `"ok":true`) || !strings.Contains(got, `"hello"`) {
		t.Fatalf("%s", got)
	}
}

func TestDispatchCancel(t *testing.T) {
	r := ipc.NewRouter()
	started := make(chan struct{})
	finished := make(chan struct{})
	r.Handle("slow", func(ctx ipc.Context, _ []byte) (any, error) {
		close(started)
		<-ctx.Done()
		close(finished)
		return nil, ctx.Err()
	})
	box := newReplyBox()
	gate := newInvokeGate()
	dispatchFrontend(allowGuard("slow"), r, nil, gate, "main",
		`{"type":"invoke","id":4,"cmd":"slow","args":{}}`, 11, box.add)
	<-started
	cancelBox := newReplyBox()
	dispatchFrontend(allowGuard("slow"), r, nil, gate, "main",
		`{"type":"cancel","invoke":4}`, 12, cancelBox.add)
	if !strings.Contains(cancelBox.wait(t), `"ok":true`) {
		t.Fatal(cancelBox.body)
	}
	got := box.wait(t)
	if !strings.Contains(got, `"ok":false`) || !strings.Contains(got, `"code":"ipc.canceled"`) || !strings.Contains(got, "ipc: canceled") {
		t.Fatalf("%s", got)
	}
	select {
	case <-finished:
	case <-time.After(time.Second):
		t.Fatal("handler kept running after cancel")
	}
}

func TestDispatchTimeoutUnblocksCaller(t *testing.T) {
	r := ipc.NewRouter()
	finished := make(chan struct{})
	r.Handle("slow", func(ctx ipc.Context, _ []byte) (any, error) {
		select {
		case <-ctx.Done():
			close(finished)
			return nil, ctx.Err()
		case <-time.After(2 * time.Second):
			return "late", nil
		}
	})
	box := newReplyBox()
	start := time.Now()
	dispatchFrontend(allowGuard("slow"), r, nil, newInvokeGate(), "main",
		`{"type":"invoke","id":2,"cmd":"slow","timeout":40}`, 3, box.add)
	got := box.wait(t)
	if time.Since(start) > time.Second {
		t.Fatal("timeout waited for the handler")
	}
	if !strings.Contains(got, `"code":"ipc.timeout"`) || !strings.Contains(got, "ipc: timeout") {
		t.Fatalf("%s", got)
	}
	select {
	case <-finished:
	case <-time.After(time.Second):
		t.Fatal("handler was not canceled at the deadline")
	}
}

func TestDispatchTimeoutIgnoresALateResult(t *testing.T) {
	r := ipc.NewRouter()
	r.Handle("block", func(ipc.Context, []byte) (any, error) {
		time.Sleep(200 * time.Millisecond)
		return "late", nil
	})
	box := newReplyBox()
	dispatchFrontend(allowGuard("block"), r, nil, newInvokeGate(), "main",
		`{"type":"invoke","id":8,"cmd":"block","timeout":30}`, 1, box.add)
	got := box.wait(t)
	if !strings.Contains(got, "ipc: timeout") {
		t.Fatalf("%s", got)
	}
	time.Sleep(250 * time.Millisecond)
	box.mu.Lock()
	n := len(box.body)
	box.mu.Unlock()
	if n != 1 {
		t.Fatalf("late result replaced the timeout: %v", box.body)
	}
}

func TestDispatchShorterDeadlineWins(t *testing.T) {
	r := ipc.NewRouter()
	r.Handle("slow", func(ctx ipc.Context, _ []byte) (any, error) {
		<-ctx.Done()
		return nil, ctx.Err()
	})
	gate := newInvokeGate()
	gate.setDefault(2 * time.Second)
	box := newReplyBox()
	start := time.Now()
	dispatchFrontend(allowGuard("slow"), r, nil, gate, "main",
		`{"type":"invoke","id":1,"cmd":"slow","timeout":40}`, 1, box.add)
	if !strings.Contains(box.wait(t), "ipc: timeout") || time.Since(start) > time.Second {
		t.Fatal(box.body)
	}

	gate.setDefault(40 * time.Millisecond)
	box = newReplyBox()
	start = time.Now()
	dispatchFrontend(allowGuard("slow"), r, nil, gate, "main",
		`{"type":"invoke","id":2,"cmd":"slow","timeout":5000}`, 2, box.add)
	if !strings.Contains(box.wait(t), "ipc: timeout") || time.Since(start) > time.Second {
		t.Fatal(box.body)
	}
}

func TestDispatchCancelIsPerWindow(t *testing.T) {
	r := ipc.NewRouter()
	auxStarted := make(chan struct{})
	r.Handle("slow", func(ctx ipc.Context, _ []byte) (any, error) {
		if ctx.Window == "aux" {
			close(auxStarted)
		}
		<-ctx.Done()
		return ctx.Window, ctx.Err()
	})
	gate := newInvokeGate()
	mainBox := newReplyBox()
	auxBox := newReplyBox()
	dispatchFrontend(allowGuard("slow"), r, nil, gate, "main",
		`{"type":"invoke","id":1,"cmd":"slow"}`, 1, mainBox.add)
	dispatchFrontend(allowGuard("slow"), r, nil, gate, "aux",
		`{"type":"invoke","id":1,"cmd":"slow"}`, 2, auxBox.add)
	<-auxStarted
	dispatchFrontend(allowGuard("slow"), r, nil, gate, "main",
		`{"type":"cancel","invoke":1}`, 3, func(string) {})
	if !strings.Contains(mainBox.wait(t), "ipc: canceled") {
		t.Fatal(mainBox.body)
	}
	select {
	case <-auxBox.done:
		t.Fatal("aux was canceled with main")
	case <-time.After(80 * time.Millisecond):
	}
	gate.cancelQuery(2)
	got := auxBox.wait(t)
	if !strings.Contains(got, "ipc: canceled") {
		t.Fatalf("%s", got)
	}
}

func TestDispatchRejectsBadTimeoutAndACL(t *testing.T) {
	r := ipc.NewRouter()
	called := false
	r.Handle("greet", func(ipc.Context, []byte) (any, error) {
		called = true
		return "no", nil
	})
	box := newReplyBox()
	dispatchFrontend(allowGuard("greet"), r, nil, newInvokeGate(), "main",
		`{"type":"invoke","id":1,"cmd":"greet","timeout":-1}`, 1, box.add)
	if !strings.Contains(box.wait(t), "ipc: invalid timeout") || called {
		t.Fatalf("called %v %v", called, box.body)
	}
	box = newReplyBox()
	dispatchFrontend(allowGuard("greet"), r, nil, newInvokeGate(), "main",
		`{"type":"invoke","id":2,"cmd":"greet","timeout":1.5}`, 2, box.add)
	if !strings.Contains(box.wait(t), "ipc: invalid timeout") {
		t.Fatal(box.body)
	}
	box = newReplyBox()
	dispatchFrontend(allowGuard("other"), r, nil, newInvokeGate(), "main",
		`{"type":"invoke","id":3,"cmd":"greet"}`, 3, box.add)
	if !strings.Contains(box.wait(t), `"code":"acl.denied"`) || !strings.Contains(box.body[0], "not permitted") || called {
		t.Fatalf("%v %v", called, box.body)
	}
}

func TestDispatchStructuredError(t *testing.T) {
	r := ipc.NewRouter()
	r.Handle("greet", func(ipc.Context, []byte) (any, error) {
		return nil, ipc.Fail("greet.empty", "name is empty")
	})
	r.Handle("boom", func(ipc.Context, []byte) (any, error) {
		return nil, errors.New("boom")
	})
	box := newReplyBox()
	dispatchFrontend(allowGuard("greet", "boom"), r, nil, newInvokeGate(), "main",
		`{"type":"invoke","id":1,"cmd":"greet","args":{}}`, 1, box.add)
	got := box.wait(t)
	if !strings.Contains(got, `"code":"greet.empty"`) || !strings.Contains(got, `"message":"name is empty"`) {
		t.Fatalf("%s", got)
	}
	box = newReplyBox()
	dispatchFrontend(allowGuard("greet", "boom"), r, nil, newInvokeGate(), "main",
		`{"type":"invoke","id":2,"cmd":"boom"}`, 2, box.add)
	got = box.wait(t)
	if !strings.Contains(got, `"code":"command.failed"`) || !strings.Contains(got, `"message":"boom"`) {
		t.Fatalf("%s", got)
	}
	box = newReplyBox()
	dispatchFrontend(allowGuard("greet", "boom"), r, nil, newInvokeGate(), "main",
		`{"type":"invoke","id":3,"cmd":"missing"}`, 3, box.add)
	if !strings.Contains(box.wait(t), `"code":"ipc.unknown_command"`) {
		t.Fatal(box.body)
	}
}

func TestDispatchPanicAndEvent(t *testing.T) {
	r := ipc.NewRouter()
	r.Handle("boom", func(ipc.Context, []byte) (any, error) {
		panic("nope")
	})
	box := newReplyBox()
	dispatchFrontend(allowGuard("boom"), r, nil, newInvokeGate(), "main",
		`{"type":"invoke","id":6,"cmd":"boom"}`, 4, box.add)
	if !strings.Contains(box.wait(t), "ipc: panic: nope") {
		t.Fatal(box.body)
	}
	bus := ipc.NewBus()
	got := ""
	bus.Listen("ping", func(payload any) {
		got, _ = payload.(string)
	})
	box = newReplyBox()
	dispatchFrontend(nil, r, bus, newInvokeGate(), "main",
		`{"type":"event","id":3,"event":"ping","payload":"from-js"}`, 5, box.add)
	if !strings.Contains(box.wait(t), `"ok":true`) || got != "from-js" {
		t.Fatalf("%s %q", box.body, got)
	}
}
