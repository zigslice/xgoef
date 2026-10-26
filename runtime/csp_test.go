package runtime

import (
	"strings"
	"testing"

	"github.com/zigslice/xgoef/conf"
)

func TestBuildArgsCSP(t *testing.T) {
	omitted := New(conf.File{ProductName: "Hello"}, nil)
	if !hasArg(omitted.buildArgs("about:blank"), "--xgoef-csp="+conf.DefaultCSP) {
		t.Fatalf("%q", omitted.buildArgs("about:blank"))
	}
	if hasPrefix(omitted.buildArgs("about:blank"), "--xgoef-no-csp") {
		t.Fatal("omitted csp disabled the header")
	}

	empty := ""
	off := New(conf.File{App: conf.AppSection{CSP: &empty}}, nil)
	if !hasArg(off.buildArgs("about:blank"), "--xgoef-no-csp") {
		t.Fatalf("%q", off.buildArgs("about:blank"))
	}
	if hasPrefix(off.buildArgs("about:blank"), "--xgoef-csp=") {
		t.Fatal("empty csp still sent a policy")
	}

	custom := "default-src 'none'"
	set := New(conf.File{App: conf.AppSection{CSP: &custom}}, nil)
	if !hasArg(set.buildArgs("about:blank"), "--xgoef-csp="+custom) {
		t.Fatalf("%q", set.buildArgs("about:blank"))
	}
}

func hasArg(args []string, want string) bool {
	for _, arg := range args {
		if arg == want {
			return true
		}
	}
	return false
}

func hasPrefix(args []string, prefix string) bool {
	for _, arg := range args {
		if strings.HasPrefix(arg, prefix) {
			return true
		}
	}
	return false
}
