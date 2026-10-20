package ipc

import (
	"encoding/json"
	"strconv"
	"strings"
	"testing"
)

type logEmitter struct {
	lines []string
}

func (e *logEmitter) EmitTo(label, jsonUTF8 string) error {
	e.lines = append(e.lines, label+" "+jsonUTF8)
	return nil
}

func (e *logEmitter) EmitAll(jsonUTF8 string) error {
	e.lines = append(e.lines, "all "+jsonUTF8)
	return nil
}

func TestChannelSendIsOrderedPerWindow(t *testing.T) {
	hub := NewHub()
	em := &logEmitter{}
	hub.SetEmitter(em)
	mainCh, err := hub.Open("main", 4)
	if err != nil {
		t.Fatal(err)
	}
	auxCh, err := hub.Open("aux", 4)
	if err != nil {
		t.Fatal(err)
	}
	for _, n := range []string{"one", "two"} {
		if err := mainCh.Send(n); err != nil {
			t.Fatal(err)
		}
	}
	if err := auxCh.Send("other"); err != nil {
		t.Fatal(err)
	}
	again, err := hub.Open("main", 4)
	if err != nil || again != mainCh {
		t.Fatalf("reopen %v %v", again, err)
	}
	if err := again.Send("three"); err != nil {
		t.Fatal(err)
	}
	if got := strings.Join(indexesOf(t, em.lines), ","); got != "main:4:0:one,main:4:1:two,aux:4:0:other,main:4:2:three" {
		t.Fatalf("lines %s", got)
	}
	hub.Close("main", 4)
	if err := mainCh.Send("nope"); err == nil || !strings.Contains(err.Error(), "closed") {
		t.Fatalf("closed send: %v", err)
	}
	if _, err := hub.Open("main", 4); err == nil {
		t.Fatal("open after close")
	}
	if _, err := hub.Open("aux", 4); err != nil {
		t.Fatalf("aux still open: %v", err)
	}
}

func TestParseChannelID(t *testing.T) {
	id, err := ParseChannelID([]byte(`{"channel":{"__channel":4},"name":"x"}`))
	if err != nil || id != 4 {
		t.Fatalf("%d %v", id, err)
	}
	id, err = ParseChannelID([]byte(`{"__channel":3}`))
	if err != nil || id != 3 {
		t.Fatalf("%d %v", id, err)
	}
	if _, err := ParseChannelID([]byte(`{"name":"x"}`)); err == nil {
		t.Fatal("missing channel accepted")
	}
	if _, err := ParseChannelID([]byte(`{"a":{"__channel":1},"b":{"__channel":2}}`)); err == nil {
		t.Fatal("two channels accepted")
	}
	if _, err := (Context{}).Channel([]byte(`{"__channel":1}`)); err == nil {
		t.Fatal("nil hub accepted")
	}
	r := NewRouter()
	hub := NewHub()
	r.SetChannels(hub)
	var got int64
	r.Handle("progress", func(ctx Context, args []byte) (any, error) {
		ch, err := ctx.Channel(args)
		if err != nil {
			return nil, err
		}
		got = ch.ID()
		return "ok", nil
	})
	out, err := r.InvokeFrom("main", "progress", []byte(`{"channel":{"__channel":9}}`))
	if err != nil || string(out) != `"ok"` || got != 9 {
		t.Fatalf("out %s err %v id %d", out, err, got)
	}
}

func indexesOf(t *testing.T, lines []string) []string {
	t.Helper()
	out := make([]string, 0, len(lines))
	for _, line := range lines {
		var msg struct {
			Channel int64  `json:"channel"`
			Index   int64  `json:"index"`
			Payload string `json:"payload"`
		}
		label, raw, ok := strings.Cut(line, " ")
		if !ok {
			t.Fatalf("line %q", line)
		}
		if err := json.Unmarshal([]byte(raw), &msg); err != nil {
			t.Fatal(err)
		}
		out = append(out, label+":"+strconv.FormatInt(msg.Channel, 10)+":"+strconv.FormatInt(msg.Index, 10)+":"+msg.Payload)
	}
	return out
}
