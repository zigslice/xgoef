package cef

import "testing"

func TestWindowLimitsRoundTrip(t *testing.T) {
	def, err := ParseWindowLimits("")
	if err != nil || def != DefaultWindowLimits() {
		t.Fatalf("%+v %v", def, err)
	}
	text, err := def.Encode()
	if err != nil || text != "1,1,1,1,0,0,0,0" {
		t.Fatalf("%q %v", text, err)
	}
	lim, err := ParseWindowLimits("0,1,0,1,320,240,800,600")
	if err != nil || lim.Resizable || !lim.Minimizable || lim.Maximizable || !lim.Closable || lim.MinWidth != 320 || lim.MaxHeight != 600 {
		t.Fatalf("%+v %v", lim, err)
	}
	again, err := lim.Encode()
	if err != nil || again != "0,1,0,1,320,240,800,600" {
		t.Fatalf("%q %v", again, err)
	}
	for _, bad := range []string{"1,1,1,1,0,1,0,0", "1,1,1,1,800,600,320,240", "2,1,1,1,0,0,0,0", "1,1,1,1,01,1,0,0", "1,1,1"} {
		if _, err := ParseWindowLimits(bad); err == nil {
			t.Fatalf("accepted %s", bad)
		}
	}
	next, err := DefaultWindowLimits().Apply(WindowLimitResizable, 0, 0)
	if err != nil || next.Resizable {
		t.Fatal(err)
	}
	next, err = next.Apply(WindowLimitMinSize, 400, 300)
	if err != nil || next.MinWidth != 400 {
		t.Fatal(err)
	}
	if _, err := next.Apply(WindowLimitMaxSize, 100, 100); err != ErrWindowRejected {
		t.Fatal(err)
	}
	next, err = next.Apply(WindowLimitMinSize, 0, 0)
	if err != nil || next.MinWidth != 0 || next.MinHeight != 0 {
		t.Fatal(err)
	}
}

func TestStubWindowLimits(t *testing.T) {
	b := NewNativeBridge()
	if err := b.SetWindowLimits("main", "0,1,1,1,320,240,0,0"); err != nil {
		t.Fatal(err)
	}
	if err := b.WindowLimit("main", WindowLimitClosable, 0, 0); err != nil {
		t.Fatal(err)
	}
	a, c, err := b.WindowLimitQuery("main", WindowLimitResizable)
	if err != nil || a != 0 || c != 0 {
		t.Fatalf("%d %d %v", a, c, err)
	}
	w, h, err := b.WindowLimitQuery("main", WindowLimitMinSize)
	if err != nil || w != 320 || h != 240 {
		t.Fatalf("%d %d %v", w, h, err)
	}
	closable, _, err := b.WindowLimitQuery("", WindowLimitClosable)
	if err != nil || closable != 0 {
		t.Fatalf("%d %v", closable, err)
	}
	if err := b.WindowLimit("main", WindowLimitMaxSize, 100, 100); err != ErrWindowRejected {
		t.Fatal(err)
	}
	b.SetWindowUnavailable(true)
	if err := b.WindowLimit("main", WindowLimitResizable, 1, 0); err == nil || err == ErrWindowRejected {
		t.Fatal(err)
	}
}
