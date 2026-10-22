package cef

import "testing"

func TestChromeEncodeDefaults(t *testing.T) {
	got, err := (WindowChrome{}).Encode()
	if err != nil || got != "1,system,0" {
		t.Fatalf("%q %v", got, err)
	}
	got, err = (WindowChrome{Frameless: true, Theme: "dark", Transparent: true}).Encode()
	if err != nil || got != "0,dark,1" {
		t.Fatalf("%q %v", got, err)
	}
	if _, err = (WindowChrome{Theme: "blue"}).Encode(); err == nil || err.Error() != `cef: window theme "blue"` {
		t.Fatal(err)
	}
}

func TestChromeRoundTrip(t *testing.T) {
	for _, raw := range []string{"", "1,system,0", "0,dark,1", "1,light,0", "0,system,0"} {
		c, err := ParseChrome(raw)
		if err != nil {
			t.Fatal(raw, err)
		}
		enc, err := c.Encode()
		if err != nil {
			t.Fatal(err)
		}
		want := raw
		if want == "" {
			want = "1,system,0"
		}
		if enc != want {
			t.Fatalf("%q encoded %q", raw, enc)
		}
		again, err := ParseChrome(enc)
		if err != nil {
			t.Fatal(err)
		}
		enc2, err := again.Encode()
		if err != nil || enc2 != enc {
			t.Fatalf("stable %q %q %v", enc, enc2, err)
		}
		if raw != "" && again != c {
			t.Fatalf("again %+v from %+v", again, c)
		}
	}
	for _, bad := range []string{"1,blue,0", "1,system", "1,system,0,1", "2,system,0", "1,system,2", "1,System,0", "1, system, 0"} {
		if _, err := ParseChrome(bad); err == nil {
			t.Fatalf("accepted %q", bad)
		}
	}
}
