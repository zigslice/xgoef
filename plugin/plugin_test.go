package plugin

import (
	"testing"

	"github.com/zigslice/xgoef/ipc"
)

type fake struct {
	name  string
	perms []string
	cmds  []string
}

func (f fake) Name() string          { return f.name }
func (f fake) Permissions() []string { return f.perms }
func (f fake) Teardown(Host) error   { return nil }
func (f fake) Setup(app Host) error {
	for _, c := range f.cmds {
		app.Register(c, func(ctx ipc.Context, args []byte) (any, error) {
			return c, nil
		})
	}
	return nil
}

type host struct {
	router *ipc.Router
}

func (h host) Register(name string, fn ipc.Handler) {
	h.router.Handle(name, fn)
}

func TestRegistrySetupRegistersCommands(t *testing.T) {
	r := NewRegistry()
	if err := r.Add(fake{name: "fs", perms: []string{"fs:read_text"}, cmds: []string{"fs:read_text"}}); err != nil {
		t.Fatal(err)
	}
	if err := r.Add(fake{name: "fs"}); err == nil {
		t.Fatal("expected duplicate error")
	}
	router := ipc.NewRouter()
	if err := r.Setup(host{router: router}); err != nil {
		t.Fatal(err)
	}
	out, err := router.InvokeFrom("main", "fs:read_text", []byte("{}"))
	if err != nil {
		t.Fatal(err)
	}
	if string(out) != `"fs:read_text"` {
		t.Fatalf("got %s", out)
	}
}
