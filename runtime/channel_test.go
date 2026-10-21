package runtime

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/zigslice/xgoef/acl"
	"github.com/zigslice/xgoef/ipc"
)

type recordEmitter struct {
	msgs []string
}

func (r *recordEmitter) EmitTo(label, jsonUTF8 string) error {
	r.msgs = append(r.msgs, label+" "+jsonUTF8)
	return nil
}

func (r *recordEmitter) EmitAll(jsonUTF8 string) error {
	r.msgs = append(r.msgs, "all "+jsonUTF8)
	return nil
}

func TestInvokeSendsChannelInOrder(t *testing.T) {
	r := ipc.NewRouter()
	hub := ipc.NewHub()
	em := &recordEmitter{}
	hub.SetEmitter(em)
	r.SetChannels(hub)
	r.Handle("progress", func(ctx ipc.Context, argsJSON []byte) (any, error) {
		ch, err := ctx.Channel(argsJSON)
		if err != nil {
			return nil, err
		}
		for _, n := range []int{1, 2, 3} {
			if err := ch.Send(n); err != nil {
				return nil, err
			}
		}
		return "1 2 3", nil
	})
	g := acl.NewGuard([]acl.Capability{{
		Identifier:  "main",
		Windows:     []string{"main"},
		Permissions: []string{"progress"},
	}})
	out := handleMessage(g, r, nil, "main", `{"type":"invoke","id":1,"cmd":"progress","args":{"channel":{"__channel":4}}}`)
	var resp struct {
		OK      bool   `json:"ok"`
		Payload string `json:"payload"`
		Error   string `json:"error"`
	}
	if err := json.Unmarshal([]byte(out), &resp); err != nil {
		t.Fatal(err)
	}
	if !resp.OK || resp.Payload != "1 2 3" {
		t.Fatalf("%s", out)
	}
	if len(em.msgs) != 3 {
		t.Fatalf("msgs %v", em.msgs)
	}
	for i, want := range []string{`"index":0`, `"index":1`, `"index":2`} {
		if !strings.Contains(em.msgs[i], "main ") || !strings.Contains(em.msgs[i], want) || !strings.Contains(em.msgs[i], `"type":"channel"`) {
			t.Fatalf("msg %d %s", i, em.msgs[i])
		}
	}
	closed := handleMessage(g, r, nil, "main", `{"type":"channel-close","id":2,"channel":4}`)
	if !strings.Contains(closed, `"ok":true`) {
		t.Fatalf("close %s", closed)
	}
	out = handleMessage(g, r, nil, "main", `{"type":"invoke","id":3,"cmd":"progress","args":{"channel":{"__channel":4}}}`)
	if !strings.Contains(out, `"ok":false`) || !strings.Contains(out, `"code":"ipc.channel_closed"`) || !strings.Contains(out, "closed") {
		t.Fatalf("after close %s", out)
	}
}
