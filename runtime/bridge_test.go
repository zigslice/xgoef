package runtime

import (
	"strings"
	"testing"

	"github.com/zigslice/xgoef/conf"
)

func TestBuildArgsBridgeSwitch(t *testing.T) {
	on := New(conf.File{App: conf.AppSection{Windows: []conf.Window{{
		Label: "main", Width: 800, Height: 600,
	}}}}, nil)
	args := strings.Join(on.buildArgs("about:blank"), "\n")
	if strings.Contains(args, "--xgoef-no-bridge") {
		t.Fatalf("default injects: %s", args)
	}

	off := false
	disabled := New(conf.File{App: conf.AppSection{
		WithGlobalXgoef: &off,
		Windows:         []conf.Window{{Label: "main", Width: 800, Height: 600}},
	}}, nil)
	args = strings.Join(disabled.buildArgs("about:blank"), "\n")
	if !strings.Contains(args, "--xgoef-no-bridge") {
		t.Fatalf("missing switch: %s", args)
	}
}
