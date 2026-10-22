//go:build linux

package globalshortcut

import (
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/jezek/xgb"
	"github.com/jezek/xgb/xproto"
)

// XGrabKey matches a modifier mask exactly. NumLock and CapsLock are extra
// bits, so each accelerator is grabbed once for every combination of those
// locks. Wayland compositors may never deliver the key to XWayland.

func defaultOpen(hook func(Shortcut, string)) (Session, error) {
	if strings.TrimSpace(os.Getenv("DISPLAY")) == "" {
		return nil, errUnavailable
	}
	s, err := openX11(hook)
	if err != nil {
		return nil, fmt.Errorf("%w: %s", errUnavailable, err.Error())
	}
	return s, nil
}

const pointerButtons = xproto.KeyButMaskButton1 |
	xproto.KeyButMaskButton2 |
	xproto.KeyButMaskButton3 |
	xproto.KeyButMaskButton4 |
	xproto.KeyButMaskButton5

type grabbed struct {
	shortcut Shortcut
	code     xproto.Keycode
	mask     uint16
	ignore   uint16
}

type x11Session struct {
	conn   *xgb.Conn
	root   xproto.Window
	hook   func(Shortcut, string)
	keys   keymap
	alt    uint16
	super  uint16
	ignore uint16

	mu     sync.Mutex
	closed bool
	grabs  []grabbed
	down   map[xproto.Keycode]bool
	done   chan struct{}
}

func openX11(hook func(Shortcut, string)) (Session, error) {
	if hook == nil {
		hook = func(Shortcut, string) {}
	}
	conn, err := xgb.NewConn()
	if err != nil {
		return nil, err
	}
	setup := xproto.Setup(conn)
	if setup == nil || len(setup.Roots) == 0 {
		conn.Close()
		return nil, fmt.Errorf("x11: no screen")
	}
	keys, err := loadKeymap(conn, setup)
	if err != nil {
		conn.Close()
		return nil, err
	}
	alt, super, ignore, err := loadModifiers(conn, keys)
	if err != nil {
		conn.Close()
		return nil, err
	}
	sess := &x11Session{
		conn:   conn,
		root:   setup.DefaultScreen(conn).Root,
		hook:   hook,
		keys:   keys,
		alt:    alt,
		super:  super,
		ignore: ignore,
		down:   map[xproto.Keycode]bool{},
		done:   make(chan struct{}),
	}
	go func() {
		sess.loop()
		close(sess.done)
	}()
	return sess, nil
}

func (s *x11Session) Grab(sc Shortcut) error {
	s.mu.Lock()
	closed := s.closed
	s.mu.Unlock()
	if closed {
		return fmt.Errorf("x11: closed")
	}
	wants := keysymsFor(sc.Key)
	if len(wants) == 0 {
		return fmt.Errorf("x11: no keysym for %s", sc.Key)
	}
	code, ok := s.keys.find(wants)
	if !ok {
		return fmt.Errorf("x11: no keycode for %s", sc.Key)
	}
	base := s.modMask(sc)
	masks := grabMasks(base, s.ignore)
	for i, mask := range masks {
		err := xproto.GrabKeyChecked(s.conn, false, s.root, mask, code, xproto.GrabModeAsync, xproto.GrabModeAsync).Check()
		if err != nil {
			for _, done := range masks[:i] {
				_ = xproto.UngrabKeyChecked(s.conn, code, s.root, done).Check()
			}
			return err
		}
	}
	s.mu.Lock()
	s.grabs = append(s.grabs, grabbed{
		shortcut: sc,
		code:     code,
		mask:     base,
		ignore:   s.ignore | pointerButtons,
	})
	s.mu.Unlock()
	return nil
}

func (s *x11Session) Ungrab(sc Shortcut) error {
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return fmt.Errorf("x11: closed")
	}
	var found *grabbed
	kept := make([]grabbed, 0, len(s.grabs))
	for _, g := range s.grabs {
		if found == nil && g.shortcut.String() == sc.String() {
			copy := g
			found = &copy
			continue
		}
		kept = append(kept, g)
	}
	s.grabs = kept
	s.mu.Unlock()
	if found == nil {
		return nil
	}
	var first error
	for _, mask := range grabMasks(found.mask, s.ignore) {
		if err := xproto.UngrabKeyChecked(s.conn, found.code, s.root, mask).Check(); err != nil && first == nil {
			first = err
		}
	}
	return first
}

func (s *x11Session) Close() error {
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return nil
	}
	s.closed = true
	s.grabs = nil
	s.mu.Unlock()
	s.conn.Close()
	<-s.done
	return nil
}

func (s *x11Session) modMask(sc Shortcut) uint16 {
	var mask uint16
	if sc.Shift {
		mask |= xproto.ModMaskShift
	}
	if sc.Control {
		mask |= xproto.ModMaskControl
	}
	if sc.Alt {
		mask |= s.alt
	}
	if sc.Super {
		mask |= s.super
	}
	return mask
}

func (s *x11Session) loop() {
	for {
		ev, xerr := s.conn.WaitForEvent()
		if ev == nil && xerr == nil {
			return
		}
		if xerr != nil {
			continue
		}
		s.handle(ev)
	}
}

func (s *x11Session) handle(ev xgb.Event) {
	switch e := ev.(type) {
	case xproto.KeyPressEvent:
		if s.down[e.Detail] {
			return
		}
		s.down[e.Detail] = true
		s.dispatch(e.Detail, e.State, Pressed)
	case xproto.KeyReleaseEvent:
		// Auto-repeat is a release followed at once by a press of the same
		// key. Drop that pair so the page sees one press and one release.
		if s.repeated(e) {
			return
		}
		delete(s.down, e.Detail)
		s.dispatch(e.Detail, e.State, Released)
	}
}

func (s *x11Session) repeated(e xproto.KeyReleaseEvent) bool {
	time.Sleep(2 * time.Millisecond)
	next, xerr := s.conn.PollForEvent()
	if xerr != nil || next == nil {
		return false
	}
	press, ok := next.(xproto.KeyPressEvent)
	if ok && press.Detail == e.Detail {
		return true
	}
	s.handle(next)
	return false
}

func (s *x11Session) dispatch(code xproto.Keycode, state uint16, kind string) {
	s.mu.Lock()
	hook := s.hook
	var hits []Shortcut
	for _, g := range s.grabs {
		if g.code == code && modifierMatch(g.mask, g.ignore, state) {
			hits = append(hits, g.shortcut)
		}
	}
	s.mu.Unlock()
	for _, sc := range hits {
		hook(sc, kind)
	}
}

func modifierMatch(mask, ignore, state uint16) bool {
	return state&^ignore == mask
}

func grabMasks(base, ignore uint16) []uint16 {
	var bits []uint16
	for bit := uint16(1); bit != 0 && bit <= xproto.ModMask5; bit <<= 1 {
		if ignore&bit != 0 {
			bits = append(bits, bit)
		}
	}
	out := make([]uint16, 1<<len(bits))
	for i := range out {
		mask := base
		for b, bit := range bits {
			if i&(1<<b) != 0 {
				mask |= bit
			}
		}
		out[i] = mask
	}
	return out
}

type keymap struct {
	min  xproto.Keycode
	per  byte
	syms []xproto.Keysym
}

func loadKeymap(conn *xgb.Conn, setup *xproto.SetupInfo) (keymap, error) {
	min := setup.MinKeycode
	max := setup.MaxKeycode
	if max < min {
		return keymap{}, fmt.Errorf("x11: bad keycode range")
	}
	count := int(max-min) + 1
	if count > 255 {
		return keymap{}, fmt.Errorf("x11: bad keycode range")
	}
	reply, err := xproto.GetKeyboardMapping(conn, min, byte(count)).Reply()
	if err != nil {
		return keymap{}, err
	}
	if reply == nil || reply.KeysymsPerKeycode == 0 {
		return keymap{}, fmt.Errorf("x11: empty keymap")
	}
	return keymap{min: min, per: reply.KeysymsPerKeycode, syms: reply.Keysyms}, nil
}

func (k keymap) find(wants []xproto.Keysym) (xproto.Keycode, bool) {
	if k.per == 0 || len(wants) == 0 {
		return 0, false
	}
	n := len(k.syms) / int(k.per)
	for i := 0; i < n; i++ {
		base := i * int(k.per)
		for j := 0; j < int(k.per); j++ {
			sym := k.syms[base+j]
			if sym == 0 {
				continue
			}
			for _, want := range wants {
				if sym == want {
					return k.min + xproto.Keycode(i), true
				}
			}
		}
	}
	return 0, false
}

func (k keymap) symsOf(code xproto.Keycode) []xproto.Keysym {
	if k.per == 0 || code < k.min {
		return nil
	}
	base := int(code-k.min) * int(k.per)
	if base < 0 || base+int(k.per) > len(k.syms) {
		return nil
	}
	var out []xproto.Keysym
	for _, sym := range k.syms[base : base+int(k.per)] {
		if sym != 0 {
			out = append(out, sym)
		}
	}
	return out
}

func loadModifiers(conn *xgb.Conn, keys keymap) (uint16, uint16, uint16, error) {
	reply, err := xproto.GetModifierMapping(conn).Reply()
	if err != nil {
		return 0, 0, 0, err
	}
	var mods [8][]xproto.Keysym
	if reply != nil && reply.KeycodesPerModifier > 0 {
		per := int(reply.KeycodesPerModifier)
		for mod := 0; mod < 8; mod++ {
			for k := 0; k < per; k++ {
				idx := mod*per + k
				if idx >= len(reply.Keycodes) {
					break
				}
				code := reply.Keycodes[idx]
				if code == 0 {
					continue
				}
				mods[mod] = append(mods[mod], keys.symsOf(code)...)
			}
		}
	}
	alt, super, ignore := classifyModifiers(mods)
	return alt, super, ignore, nil
}

const (
	symAltL   xproto.Keysym = 0xffe9
	symAltR   xproto.Keysym = 0xffea
	symSuperL xproto.Keysym = 0xffeb
	symSuperR xproto.Keysym = 0xffec
	symMetaL  xproto.Keysym = 0xffe7
	symMetaR  xproto.Keysym = 0xffe8
)

var modBit = [8]uint16{
	xproto.ModMaskShift,
	xproto.ModMaskLock,
	xproto.ModMaskControl,
	xproto.ModMask1,
	xproto.ModMask2,
	xproto.ModMask3,
	xproto.ModMask4,
	xproto.ModMask5,
}

// classifyModifiers finds Alt and Super from the modifier map.
// Lock and every other Mod1–Mod5 bit is ignored so CapsLock and NumLock
// still match the grab. Missing keys fall back to Mod1 and Mod4.
func classifyModifiers(mods [8][]xproto.Keysym) (alt, super, ignore uint16) {
	var altBit, superBit, metaBit uint16
	for i, syms := range mods {
		bit := modBit[i]
		for _, sym := range syms {
			switch sym {
			case symAltL, symAltR:
				if altBit == 0 {
					altBit = bit
				}
			case symSuperL, symSuperR:
				if superBit == 0 {
					superBit = bit
				}
			case symMetaL, symMetaR:
				if metaBit == 0 {
					metaBit = bit
				}
			}
		}
	}
	if superBit == 0 {
		superBit = metaBit
	}
	if altBit == 0 {
		altBit = xproto.ModMask1
	}
	if superBit == 0 || superBit == altBit {
		if altBit != xproto.ModMask4 {
			superBit = xproto.ModMask4
		} else {
			superBit = xproto.ModMask3
		}
	}
	ignore = xproto.ModMaskLock
	for _, bit := range []uint16{xproto.ModMask1, xproto.ModMask2, xproto.ModMask3, xproto.ModMask4, xproto.ModMask5} {
		if bit != altBit && bit != superBit {
			ignore |= bit
		}
	}
	return altBit, superBit, ignore
}

func keysymsFor(key string) []xproto.Keysym {
	if len(key) == 1 {
		c := key[0]
		switch {
		case c >= 'A' && c <= 'Z':
			return []xproto.Keysym{xproto.Keysym(c + ('a' - 'A')), xproto.Keysym(c)}
		case c >= '0' && c <= '9':
			return []xproto.Keysym{xproto.Keysym(c)}
		}
	}
	if len(key) >= 2 && key[0] == 'F' {
		n := 0
		for _, c := range key[1:] {
			if c < '0' || c > '9' {
				n = 0
				break
			}
			n = n*10 + int(c-'0')
		}
		if n >= 1 && n <= 24 {
			return []xproto.Keysym{xproto.Keysym(0xffbe + n - 1)}
		}
	}
	if sym, ok := namedSym[key]; ok {
		return []xproto.Keysym{sym}
	}
	return nil
}

var namedSym = map[string]xproto.Keysym{
	"Escape":       0xff1b,
	"Tab":          0xff09,
	"Space":        0x0020,
	"Enter":        0xff0d,
	"Backspace":    0xff08,
	"ArrowUp":      0xff52,
	"ArrowDown":    0xff54,
	"ArrowLeft":    0xff51,
	"ArrowRight":   0xff53,
	"Home":         0xff50,
	"End":          0xff57,
	"PageUp":       0xff55,
	"PageDown":     0xff56,
	"Insert":       0xff63,
	"Delete":       0xffff,
	"Minus":        0x002d,
	"Equal":        0x003d,
	"Comma":        0x002c,
	"Period":       0x002e,
	"Slash":        0x002f,
	"Semicolon":    0x003b,
	"Quote":        0x0027,
	"BracketLeft":  0x005b,
	"BracketRight": 0x005d,
	"Backslash":    0x005c,
	"Backquote":    0x0060,
}
