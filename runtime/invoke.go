package runtime

import (
	"context"
	"encoding/json"
	"strconv"
	"sync"
	"time"

	"github.com/zigslice/xgoef/acl"
	"github.com/zigslice/xgoef/ipc"
)

const maxInvokeTimeout = 24 * time.Hour

// invokeGate tracks page invokes so a later cancel or a deadline can answer
// without waiting for a handler that ignores its context.
type invokeGate struct {
	mu      sync.Mutex
	def     time.Duration
	byMsg   map[string]*invokeSlot
	byQuery map[int64]*invokeSlot
}

type invokeSlot struct {
	once    sync.Once
	cancel  context.CancelFunc
	respond func(string)
	id      int
	query   int64
	window  string
}

// SetInvokeTimeout caps every page invoke that does not ask for a shorter deadline.
// Zero clears the cap. Menu activation and Router.Invoke stay uncapped.
func (a *App) SetInvokeTimeout(d time.Duration) {
	if a == nil || a.invokes == nil {
		return
	}
	a.invokes.setDefault(d)
}

func newInvokeGate() *invokeGate {
	return &invokeGate{
		byMsg:   map[string]*invokeSlot{},
		byQuery: map[int64]*invokeSlot{},
	}
}

func (g *invokeGate) setDefault(d time.Duration) {
	if d < 0 {
		d = 0
	}
	g.mu.Lock()
	g.def = d
	g.mu.Unlock()
}

func (g *invokeGate) run(window string, id int, queryID int64, perCall time.Duration, respond func(string), work func(context.Context) string) {
	ctx, cancel := g.context(perCall)
	sl := &invokeSlot{
		cancel:  cancel,
		respond: respond,
		id:      id,
		query:   queryID,
		window:  window,
	}
	g.track(sl)
	go func() {
		<-ctx.Done()
		sl.send(invokeStatus(id, ctx.Err()))
	}()
	go func() {
		defer func() {
			g.forget(sl)
			cancel()
		}()
		defer func() {
			if rec := recover(); rec != nil {
				sl.send(failResponse(id, ipc.Fail(ipc.CodePanic, "ipc: panic: %v", rec)))
			}
		}()
		body := work(ctx)
		if err := ctx.Err(); err != nil {
			sl.send(invokeStatus(id, err))
			return
		}
		sl.send(body)
	}()
}

func (g *invokeGate) context(perCall time.Duration) (context.Context, context.CancelFunc) {
	d := perCall
	g.mu.Lock()
	def := g.def
	g.mu.Unlock()
	if def > 0 && (d <= 0 || def < d) {
		d = def
	}
	if d > 0 {
		return context.WithTimeout(context.Background(), d)
	}
	return context.WithCancel(context.Background())
}

func (g *invokeGate) track(sl *invokeSlot) {
	g.mu.Lock()
	defer g.mu.Unlock()
	if sl.id >= 1 {
		key := flightKey(sl.window, sl.id)
		if old := g.byMsg[key]; old != nil && old != sl {
			old.cancel()
		}
		g.byMsg[key] = sl
	}
	if sl.query != 0 {
		if old := g.byQuery[sl.query]; old != nil && old != sl {
			old.cancel()
		}
		g.byQuery[sl.query] = sl
	}
}

func (g *invokeGate) forget(sl *invokeSlot) {
	g.mu.Lock()
	defer g.mu.Unlock()
	if sl.id >= 1 && g.byMsg[flightKey(sl.window, sl.id)] == sl {
		delete(g.byMsg, flightKey(sl.window, sl.id))
	}
	if sl.query != 0 && g.byQuery[sl.query] == sl {
		delete(g.byQuery, sl.query)
	}
}

func (g *invokeGate) cancelID(window string, id int) {
	if g == nil || id < 1 {
		return
	}
	g.mu.Lock()
	sl := g.byMsg[flightKey(window, id)]
	g.mu.Unlock()
	if sl != nil {
		sl.cancel()
	}
}

func (g *invokeGate) cancelQuery(queryID int64) {
	if g == nil || queryID == 0 {
		return
	}
	g.mu.Lock()
	sl := g.byQuery[queryID]
	g.mu.Unlock()
	if sl != nil {
		sl.cancel()
	}
}

func (s *invokeSlot) send(body string) {
	s.once.Do(func() {
		if s.respond != nil {
			s.respond(body)
		}
	})
}

func flightKey(window string, id int) string {
	return window + "\x00" + strconv.Itoa(id)
}

func invokeStatus(id int, err error) string {
	if err == context.DeadlineExceeded {
		return failResponse(id, ipc.ErrTimeout)
	}
	return failResponse(id, ipc.ErrCanceled)
}

// dispatchFrontend delivers one page envelope. Invokes run off the caller so
// the CEF query callback can return. respond is called exactly once.
func dispatchFrontend(guard *acl.Guard, router *ipc.Router, events *ipc.Bus, gate *invokeGate, window, requestJSON string, queryID int64, respond func(string)) {
	if respond == nil {
		respond = func(string) {}
	}
	var req struct {
		Type    string          `json:"type"`
		ID      int             `json:"id"`
		Cmd     string          `json:"cmd"`
		Args    json.RawMessage `json:"args"`
		Timeout *float64        `json:"timeout"`
		Invoke  int             `json:"invoke"`
	}
	if err := json.Unmarshal([]byte(requestJSON), &req); err != nil {
		respond(failResponse(0, ipc.Fail(ipc.CodeInvalidMessage, "invalid invoke JSON: %s", err.Error())))
		return
	}
	switch req.Type {
	case "cancel":
		if gate != nil {
			gate.cancelID(window, req.Invoke)
		}
		respond(okResponse(req.ID, map[string]any{"ok": true}))
		return
	case "", "invoke":
		if gate == nil {
			respond(handleMessage(guard, router, events, window, requestJSON))
			return
		}
		if req.Cmd == "" {
			respond(failResponse(req.ID, ipc.Fail(ipc.CodeMissingCmd, "missing cmd")))
			return
		}
		if guard == nil || !guard.Allows(window, req.Cmd) {
			respond(failResponse(req.ID, &acl.DenyError{Window: window, Command: req.Cmd}))
			return
		}
		per, err := invokeTimeout(req.Timeout)
		if err != nil {
			respond(failResponse(req.ID, err))
			return
		}
		args := []byte(req.Args)
		if len(args) == 0 {
			args = []byte("{}")
		}
		id, cmd := req.ID, req.Cmd
		gate.run(window, id, queryID, per, respond, func(ctx context.Context) string {
			return invokeBody(router, window, id, cmd, args, ctx)
		})
		return
	default:
		respond(handleMessage(guard, router, events, window, requestJSON))
	}
}

func invokeTimeout(ms *float64) (time.Duration, error) {
	if ms == nil || *ms == 0 {
		return 0, nil
	}
	v := *ms
	limit := float64(maxInvokeTimeout / time.Millisecond)
	if v < 0 || v > limit || v != float64(int64(v)) {
		return 0, ipc.ErrInvalidTimeout
	}
	return time.Duration(int64(v)) * time.Millisecond, nil
}

func invokeBody(router *ipc.Router, window string, id int, cmd string, args []byte, ctx context.Context) string {
	if router == nil {
		return failResponse(id, ipc.Fail(ipc.CodeNoRouter, "ipc: no router"))
	}
	out, err := router.InvokeContext(ctx, window, cmd, args)
	if err != nil {
		if ctx.Err() != nil {
			return invokeStatus(id, ctx.Err())
		}
		return failResponse(id, err)
	}
	if ctx.Err() != nil {
		return invokeStatus(id, ctx.Err())
	}
	var payload any
	if err := json.Unmarshal(out, &payload); err != nil {
		return failResponse(id, ipc.Fail(ipc.CodeBadResponse, "marshal response: %s", err.Error()))
	}
	return okResponse(id, payload)
}
