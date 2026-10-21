package api

import (
	"os/exec"
	"testing"
)

func TestFrontendErrorCodes(t *testing.T) {
	node, err := exec.LookPath("node")
	if err != nil {
		t.Fatal("node is required to test api/js/xgoef.js")
	}
	out, err := exec.Command(node, "js/xgoef_test.js").CombinedOutput()
	if err != nil {
		t.Fatalf("%v\n%s", err, out)
	}
}
