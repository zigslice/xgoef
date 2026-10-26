package cef

import (
	"strings"
	"testing"
)

func TestWindowTitleAndSize(t *testing.T) {
	if !WindowTitleOK("") || !WindowTitleOK("hello") || !WindowTitleOK("窗口") || !WindowTitleOK(strings.Repeat("a", WindowTitleMax)) {
		t.Fatal("accepted")
	}
	if WindowTitleOK(strings.Repeat("a", WindowTitleMax+1)) || WindowTitleOK("a\nb") || WindowTitleOK("a\x00b") || WindowTitleOK("\xff") {
		t.Fatal("rejected")
	}
	if !WindowSizeOK(1, 1) || !WindowSizeOK(WindowSizeMax, WindowSizeMax) {
		t.Fatal("size")
	}
	if WindowSizeOK(0, 1) || WindowSizeOK(1, WindowSizeMax+1) || WindowSizeOK(-1, 1) {
		t.Fatal("size range")
	}
	if !WindowOpKnown(WindowOpMinimize) || !WindowOpKnown(WindowOpSetSize) || WindowOpKnown(0) || WindowOpKnown(13) {
		t.Fatal("ops")
	}
	if !WindowQueryKnown(WindowQueryVisible) || WindowQueryKnown(0) {
		t.Fatal("queries")
	}
}

func TestStubWindowOp(t *testing.T) {
	b := NewNativeBridge()
	if err := b.WindowOp("main", WindowOpSetTitle, 0, 0, "hello"); err != nil {
		t.Fatal(err)
	}
	if err := b.WindowOp("main", WindowOpSetSize, 800, 600, ""); err != nil {
		t.Fatal(err)
	}
	if err := b.WindowOp("main", WindowOpToggleMaximize, 0, 0, ""); err != nil {
		t.Fatal(err)
	}
	on, err := b.WindowQuery("main", WindowQueryMaximized)
	if err != nil || !on {
		t.Fatalf("%v %v", on, err)
	}
	if err := b.WindowOp("main", WindowOpToggleMaximize, 0, 0, ""); err != nil {
		t.Fatal(err)
	}
	on, err = b.WindowQuery("main", WindowQueryMaximized)
	if err != nil || on {
		t.Fatalf("second %v %v", on, err)
	}
	last, ok := b.LastWindowOp()
	if !ok || last.Label != "main" || last.Op != WindowOpToggleMaximize {
		t.Fatalf("%+v %v", last, ok)
	}
	if err := b.WindowOp("main", WindowOpSetTitle, 0, 0, "bad\n"); err != ErrWindowRejected {
		t.Fatal(err)
	}
	if err := b.WindowOp("main", WindowOpSetSize, 0, 1, ""); err != ErrWindowRejected {
		t.Fatal(err)
	}
	b.SetWindowUnavailable(true)
	if err := b.WindowOp("main", WindowOpShow, 0, 0, ""); err == nil || err == ErrWindowRejected {
		t.Fatal(err)
	}
	if _, err := b.WindowQuery("main", WindowQueryVisible); err == nil {
		t.Fatal("query")
	}
}
