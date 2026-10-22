//go:build linux

package globalshortcut

import (
	"errors"
	"os"
	"testing"

	"github.com/jezek/xgb/xproto"
)

func TestGrabMasksAndMatch(t *testing.T) {
	base := uint16(xproto.ModMaskControl | xproto.ModMaskShift)
	ignore := uint16(xproto.ModMaskLock | xproto.ModMask2)
	masks := grabMasks(base, ignore)
	if len(masks) != 4 || masks[0] != base {
		t.Fatalf("%v", masks)
	}
	state := base | xproto.ModMask2 | pointerButtons
	if !modifierMatch(base, ignore|pointerButtons, state) {
		t.Fatal("num lock and buttons should still match")
	}
	if modifierMatch(base, ignore|pointerButtons, xproto.ModMaskControl) {
		t.Fatal("shift is part of the shortcut")
	}
	if modifierMatch(base, ignore|pointerButtons, base|xproto.ModMask1) {
		t.Fatal("alt is not ignored")
	}
}

func TestClassifyModifiers(t *testing.T) {
	var mods [8][]xproto.Keysym
	mods[3] = []xproto.Keysym{symAltL}
	mods[4] = []xproto.Keysym{0xff7f}
	mods[6] = []xproto.Keysym{symSuperL}
	alt, super, ignore := classifyModifiers(mods)
	if alt != xproto.ModMask1 || super != xproto.ModMask4 {
		t.Fatalf("alt %d super %d", alt, super)
	}
	if ignore&xproto.ModMask1 != 0 || ignore&xproto.ModMask4 != 0 || ignore&xproto.ModMaskLock == 0 || ignore&xproto.ModMask2 == 0 {
		t.Fatalf("ignore %d", ignore)
	}

	var empty [8][]xproto.Keysym
	alt, super, ignore = classifyModifiers(empty)
	if alt != xproto.ModMask1 || super != xproto.ModMask4 || ignore&xproto.ModMask5 == 0 {
		t.Fatalf("fallback alt %d super %d ignore %d", alt, super, ignore)
	}

	var meta [8][]xproto.Keysym
	meta[6] = []xproto.Keysym{symMetaL}
	_, super, _ = classifyModifiers(meta)
	if super != xproto.ModMask4 {
		t.Fatal(super)
	}
}

func TestKeysymsAndFind(t *testing.T) {
	syms := keysymsFor("A")
	if len(syms) != 2 || syms[0] != 'a' || syms[1] != 'A' {
		t.Fatal(syms)
	}
	if got := keysymsFor("F1"); len(got) != 1 || got[0] != 0xffbe {
		t.Fatal(got)
	}
	if got := keysymsFor("Escape"); len(got) != 1 || got[0] != 0xff1b {
		t.Fatal(got)
	}
	keys := keymap{min: 8, per: 2, syms: []xproto.Keysym{'a', 'A', 0, 0}}
	code, ok := keys.find(keysymsFor("A"))
	if !ok || code != 8 {
		t.Fatalf("%d %v", code, ok)
	}
	if _, ok := keys.find(keysymsFor("B")); ok {
		t.Fatal("found B")
	}
}

func TestOpenWithoutDisplay(t *testing.T) {
	t.Setenv("DISPLAY", "")
	_, err := defaultOpen(nil)
	if !errors.Is(err, errUnavailable) {
		t.Fatal(err)
	}
}

func TestX11GrabAndUngrab(t *testing.T) {
	if os.Getenv("DISPLAY") == "" {
		t.Skip("no DISPLAY")
	}
	sess, err := openX11(func(Shortcut, string) {})
	if err != nil {
		t.Fatal(err)
	}
	defer sess.Close()
	xs := sess.(*x11Session)
	raw := "Control+Alt+Shift+Super+F24"
	sc, err := Parse(raw)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := xs.keys.find(keysymsFor(sc.Key)); !ok {
		raw = "Control+Alt+Shift+Super+K"
		sc, err = Parse(raw)
		if err != nil {
			t.Fatal(err)
		}
		if _, ok := xs.keys.find(keysymsFor(sc.Key)); !ok {
			t.Fatal("keymap has neither F24 nor K")
		}
	}
	if err := sess.Grab(sc); err != nil {
		t.Fatal(err)
	}
	if err := sess.Ungrab(sc); err != nil {
		t.Fatal(err)
	}
	if err := sess.Grab(sc); err != nil {
		t.Fatal(err)
	}
	if err := sess.Ungrab(sc); err != nil {
		t.Fatal(err)
	}
}
