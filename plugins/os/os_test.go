package osinfo

import (
	"runtime"
	"testing"

	"github.com/zigslice/xgoef/ipc"
)

type host struct{ r *ipc.Router }

func (h host) Register(name string, fn ipc.Handler) { h.r.Handle(name, fn) }

func TestPlatform(t *testing.T) {
	p := New()
	r := ipc.NewRouter()
	if err := p.Setup(host{r: r}); err != nil {
		t.Fatal(err)
	}
	out, err := r.Invoke("os:platform", []byte("{}"))
	if err != nil {
		t.Fatal(err)
	}
	if string(out) != `"`+runtime.GOOS+`"` {
		t.Fatalf("got %s", out)
	}
	out, err = r.Invoke("os:arch", []byte("{}"))
	if err != nil {
		t.Fatal(err)
	}
	if string(out) != `"`+runtime.GOARCH+`"` {
		t.Fatalf("got %s", out)
	}
}
