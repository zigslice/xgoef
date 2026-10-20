package clipboard

import (
	"strings"
	"testing"

	"github.com/zigslice/xgoef/ipc"
)

type host struct{ r *ipc.Router }

func (h host) Register(name string, fn ipc.Handler) { h.r.Handle(name, fn) }

func TestMemoryRoundTrip(t *testing.T) {
	p := New(nil)
	r := ipc.NewRouter()
	if err := p.Setup(host{r: r}); err != nil {
		t.Fatal(err)
	}
	out, err := r.Invoke("clipboard:read_text", []byte("{}"))
	if err != nil || string(out) != `""` {
		t.Fatalf("empty %s %v", out, err)
	}
	if _, err := r.Invoke("clipboard:write_text", []byte(`{"text":"from xgoef"}`)); err != nil {
		t.Fatal(err)
	}
	out, err = r.Invoke("clipboard:read_text", []byte("{}"))
	if err != nil || string(out) != `"from xgoef"` {
		t.Fatalf("read %s %v", out, err)
	}
	big := strings.Repeat("a", MaxBytes+1)
	if _, err := r.Invoke("clipboard:write_text", []byte(`{"text":"`+big+`"}`)); err == nil {
		t.Fatal("expected size error")
	}
}
