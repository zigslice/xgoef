package ipc

import (
	"context"
	"errors"
	"fmt"
	"testing"
)

func TestFailAndDescribe(t *testing.T) {
	err := Fail("greet.empty", "name is empty")
	code, msg := Describe(err)
	if code != "greet.empty" || msg != "name is empty" {
		t.Fatalf("%s %s", code, msg)
	}
	if CodeOf(err) != "greet.empty" {
		t.Fatal(CodeOf(err))
	}
	wrapped := fmt.Errorf("handler: %w", err)
	code, msg = Describe(wrapped)
	if code != "greet.empty" || msg != "handler: name is empty" {
		t.Fatalf("wrapped %s %s", code, msg)
	}

	plain := errors.New("boom")
	code, msg = Describe(plain)
	if code != CodeFailed || msg != "boom" {
		t.Fatalf("plain %s %s", code, msg)
	}
	if CodeOf(plain) != "" {
		t.Fatal(CodeOf(plain))
	}

	bad := Fail("No Spaces", "kept")
	code, msg = Describe(bad)
	if code != CodeFailed || msg != "kept" {
		t.Fatalf("bad code %s %s", code, msg)
	}
	if !errors.Is(bad, Fail(CodeFailed, "other")) {
		t.Fatal("invalid codes should compare as command.failed")
	}

	long := Fail("app."+repeat('a', 80), "too long")
	if CodeOf(long) != CodeFailed {
		t.Fatalf("long code %s", CodeOf(long))
	}

	if _, msg := Describe(Fail("fs.denied", "")); msg != "invoke failed" {
		t.Fatalf("empty message %q", msg)
	}
	if code, msg := Describe(nil); code != CodeFailed || msg != "invoke failed" {
		t.Fatalf("nil %s %s", code, msg)
	}
}

func TestSentinelCodes(t *testing.T) {
	if code, msg := Describe(ErrCanceled); code != CodeCanceled || msg != "ipc: canceled" {
		t.Fatalf("%s %s", code, msg)
	}
	if code, msg := Describe(ErrTimeout); code != CodeTimeout || msg != "ipc: timeout" {
		t.Fatalf("%s %s", code, msg)
	}
	if code, msg := Describe(context.DeadlineExceeded); code != CodeTimeout || msg != ErrTimeout.Error() {
		t.Fatalf("deadline %s %s", code, msg)
	}
	if code, msg := Describe(context.Canceled); code != CodeCanceled || msg != ErrCanceled.Error() {
		t.Fatalf("cancel %s %s", code, msg)
	}
	if !errors.Is(Fail(CodeCanceled, "ipc: canceled"), ErrCanceled) {
		t.Fatal("same code should match")
	}
	if errors.Is(ErrCanceled, ErrTimeout) {
		t.Fatal("different codes matched")
	}
	if errors.Is(ErrCanceled, context.Canceled) {
		t.Fatal("sentinel should not match context.Canceled")
	}
}

func repeat(b byte, n int) string {
	buf := make([]byte, n)
	for i := range buf {
		buf[i] = b
	}
	return string(buf)
}
