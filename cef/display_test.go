package cef

import (
	"strings"
	"testing"
)

func TestDisplayTableRoundTrip(t *testing.T) {
	table := DisplayTable{
		HasWindow:   true,
		WindowID:    10,
		WindowScale: 1.5,
		Displays: []Display{
			{
				ID: 10, ScaleFactor: 1.5,
				X: 0, Y: 0, Width: 1280, Height: 800,
				WorkX: 0, WorkY: 32, WorkWidth: 1280, WorkHeight: 768,
				PixelX: 0, PixelY: 0, PixelWidth: 1920, PixelHeight: 1200,
				WorkPixelX: 0, WorkPixelY: 48, WorkPixelWidth: 1920, WorkPixelHeight: 1152,
				Rotation: 0, Primary: true,
			},
			{
				ID: 11, ScaleFactor: 1,
				X: 1280, Y: 0, Width: 1920, Height: 1080,
				WorkX: 1280, WorkY: 0, WorkWidth: 1920, WorkHeight: 1040,
				PixelX: 1280, PixelY: 0, PixelWidth: 1920, PixelHeight: 1080,
				WorkPixelX: 1280, WorkPixelY: 0, WorkPixelWidth: 1920, WorkPixelHeight: 1040,
				Rotation: 90, Primary: false,
			},
		},
	}
	text, err := FormatDisplayTable(table)
	if err != nil {
		t.Fatal(err)
	}
	got, err := ParseDisplayTable(text)
	if err != nil {
		t.Fatal(err)
	}
	if !got.HasWindow || got.WindowID != 10 || got.WindowScale != 1.5 || len(got.Displays) != 2 {
		t.Fatalf("%+v", got)
	}
	if got.Displays[0].PixelWidth != 1920 || !got.Displays[0].Primary || got.Displays[1].Rotation != 90 || got.Displays[1].X != 1280 {
		t.Fatalf("%+v", got.Displays)
	}
	again, err := FormatDisplayTable(got)
	if err != nil || again != text {
		t.Fatalf("stable\n%s\n%s\n%v", text, again, err)
	}
}

func TestDisplayTableRejects(t *testing.T) {
	good, err := FormatDisplayTable(DisplayTable{
		HasWindow: false,
		Displays: []Display{{
			ID: 1, ScaleFactor: 2,
			Width: 800, Height: 600, WorkWidth: 800, WorkHeight: 600,
			PixelWidth: 1600, PixelHeight: 1200, WorkPixelWidth: 1600, WorkPixelHeight: 1200,
			Primary: true,
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(good, "window - 0.000000\n") {
		t.Fatalf("%q", good)
	}
	none, err := ParseDisplayTable("window - 0.000000\n")
	if err != nil || none.HasWindow || len(none.Displays) != 0 {
		t.Fatalf("%+v %v", none, err)
	}
	for _, raw := range []string{
		"",
		"1 1.000000 0 0 1 1 0 0 1 1 0 0 1 1 0 0 1 1 0 1\n",
		"window 1 1.000000\n1 1.000000 0 0 1 1 0 0 1 1 0 0 1 1 0 0 1 1 0 1\n2 1.000000 2 0 1 1 2 0 1 1 2 0 1 1 2 0 1 1 0 1\n",
		"window 1 1.000000\n1 1.000000 0 0 1 1 0 0 1 1 0 0 1 1 0 0 1 1 0 1\n1 1.000000 2 0 1 1 2 0 1 1 2 0 1 1 2 0 1 1 0 0\n",
		"window 1 1.000000\n1 0.000000 0 0 1 1 0 0 1 1 0 0 1 1 0 0 1 1 0 1\n",
		"window 1 1.000000\n1 1.000000 0 0 0 1 0 0 1 1 0 0 1 1 0 0 1 1 0 1\n",
		"window 1 1.000000\n1 1.000000 0 0 1 1 0 0 1 1 0 0 1 1 0 0 1 1 45 1\n",
		"window 1 1.000000\n1 1.000000 0 0 1 1 0 0 1 1 0 0 1 1 0 0 1 1 0 0\n",
	} {
		if _, err := ParseDisplayTable(raw); err == nil {
			t.Fatalf("accepted %q", raw)
		}
	}
}

func TestDefaultDisplayTable(t *testing.T) {
	got, err := ParseDisplayTable(DefaultDisplayTable())
	if err != nil {
		t.Fatal(err)
	}
	if !got.HasWindow || got.WindowID != 1 || got.WindowScale != 1 || len(got.Displays) != 1 || !got.Displays[0].Primary || got.Displays[0].Width != 1920 {
		t.Fatalf("%+v", got)
	}
}
