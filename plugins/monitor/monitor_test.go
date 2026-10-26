package monitor

import (
	"strings"
	"testing"

	"github.com/zigslice/xgoef/cef"
	"github.com/zigslice/xgoef/ipc"
)

type host struct{ r *ipc.Router }

func (h host) Register(name string, fn ipc.Handler) { h.r.Handle(name, fn) }

func TestListPrimaryCurrentAndScale(t *testing.T) {
	text, err := cef.FormatDisplayTable(cef.DisplayTable{
		HasWindow:   true,
		WindowID:    11,
		WindowScale: 2,
		Displays: []cef.Display{
			{ID: 10, ScaleFactor: 1, Width: 800, Height: 600, WorkWidth: 800, WorkHeight: 580, PixelWidth: 800, PixelHeight: 600, WorkPixelWidth: 800, WorkPixelHeight: 580, Primary: true},
			{ID: 11, ScaleFactor: 2, X: 800, Width: 640, Height: 480, WorkX: 800, WorkWidth: 640, WorkHeight: 480, PixelX: 1600, PixelWidth: 1280, PixelHeight: 960, WorkPixelX: 1600, WorkPixelWidth: 1280, WorkPixelHeight: 960},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	p := New(func(window string) (string, error) {
		if window != "aux" {
			t.Fatalf("window %q", window)
		}
		return text, nil
	}, nil)
	r := ipc.NewRouter()
	if err := p.Setup(host{r}); err != nil {
		t.Fatal(err)
	}
	got, err := r.InvokeFrom("aux", "monitor:list", []byte(`{}`))
	if err != nil || !strings.Contains(string(got), `"id":11`) || !strings.Contains(string(got), `"scaleFactor":2`) {
		t.Fatalf("%s %v", got, err)
	}
	got, err = r.InvokeFrom("aux", "monitor:primary", []byte(`{}`))
	if err != nil || !strings.Contains(string(got), `"id":10`) || strings.Contains(string(got), `"id":11`) {
		t.Fatalf("primary %s %v", got, err)
	}
	got, err = r.InvokeFrom("aux", "monitor:current", []byte(`{}`))
	if err != nil || !strings.Contains(string(got), `"id":11`) || !strings.Contains(string(got), `"x":800`) {
		t.Fatalf("current %s %v", got, err)
	}
	got, err = r.InvokeFrom("aux", "monitor:scale", []byte(`{}`))
	if err != nil || string(got) != "2" {
		t.Fatalf("scale %s %v", got, err)
	}
}

func TestScaleFallsBackToPrimary(t *testing.T) {
	text, err := cef.FormatDisplayTable(cef.DisplayTable{
		Displays: []cef.Display{{
			ID: 1, ScaleFactor: 1.25, Width: 100, Height: 100, WorkWidth: 100, WorkHeight: 100,
			PixelWidth: 125, PixelHeight: 125, WorkPixelWidth: 125, WorkPixelHeight: 125, Primary: true,
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	p := New(func(string) (string, error) { return text, nil }, nil)
	r := ipc.NewRouter()
	if err := p.Setup(host{r}); err != nil {
		t.Fatal(err)
	}
	got, err := r.InvokeFrom("main", "monitor:current", []byte(`{}`))
	if err != nil || string(got) != "null" {
		t.Fatalf("current %s %v", got, err)
	}
	got, err = r.InvokeFrom("main", "monitor:scale", []byte(`{}`))
	if err != nil || string(got) != "1.25" {
		t.Fatalf("scale %s %v", got, err)
	}
}

func TestMove(t *testing.T) {
	var gotWindow string
	var gotX, gotY int
	p := New(func(string) (string, error) { return cef.DefaultDisplayTable(), nil }, func(window string, x, y int) error {
		gotWindow, gotX, gotY = window, x, y
		return nil
	})
	r := ipc.NewRouter()
	if err := p.Setup(host{r}); err != nil {
		t.Fatal(err)
	}
	if _, err := r.InvokeFrom("main", "monitor:move", []byte(`{"x":1920,"y":-40}`)); err != nil {
		t.Fatal(err)
	}
	if gotWindow != "main" || gotX != 1920 || gotY != -40 {
		t.Fatalf("%s %d %d", gotWindow, gotX, gotY)
	}
	if _, err := r.InvokeFrom("", "monitor:move", []byte(`{"x":1.5,"y":0}`)); err == nil || ipc.CodeOf(err) != "monitor.invalid" {
		t.Fatalf("%v", err)
	}
	if _, err := r.InvokeFrom("main", "monitor:move", []byte(`{"x":1}`)); err == nil || ipc.CodeOf(err) != "monitor.invalid" {
		t.Fatalf("%v", err)
	}
	if _, err := r.InvokeFrom("main", "monitor:move", []byte(`{"x":100001,"y":0}`)); err == nil || ipc.CodeOf(err) != "monitor.invalid" {
		t.Fatalf("%v", err)
	}
	if gotX != 1920 {
		t.Fatal("invalid move ran")
	}
}

func TestUnavailable(t *testing.T) {
	p := New(func(string) (string, error) { return "", errSample }, nil)
	r := ipc.NewRouter()
	if err := p.Setup(host{r}); err != nil {
		t.Fatal(err)
	}
	if _, err := r.Invoke("monitor:list", []byte(`{}`)); err == nil || ipc.CodeOf(err) != "monitor.unavailable" || !strings.Contains(err.Error(), "offline") {
		t.Fatalf("%v", err)
	}
	if _, err := r.Invoke("monitor:scale", []byte(`{}`)); err == nil || ipc.CodeOf(err) != "monitor.unavailable" {
		t.Fatalf("%v", err)
	}
	bare := New(nil, nil)
	if err := bare.Setup(host{r}); err != nil {
		t.Fatal(err)
	}
	if _, err := r.Invoke("monitor:move", []byte(`{"x":0,"y":0}`)); err == nil || ipc.CodeOf(err) != "monitor.unavailable" {
		t.Fatalf("%v", err)
	}
}

type sampleErr string

func (e sampleErr) Error() string { return string(e) }

const errSample sampleErr = "monitor offline"
