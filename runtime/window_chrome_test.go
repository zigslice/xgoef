package runtime

import (
	"strings"
	"testing"

	"github.com/zigslice/xgoef/cef"
	"github.com/zigslice/xgoef/conf"
)

func TestBuildArgsChrome(t *testing.T) {
	dec := false
	tr := true
	dark := New(conf.File{App: conf.AppSection{Windows: []conf.Window{{
		Label: "main", Title: "t", Width: 800, Height: 600,
		Decorations: &dec, Theme: "dark", Transparent: &tr,
	}}}}, nil)
	args := strings.Join(dark.buildArgs("about:blank"), "\n")
	if !strings.Contains(args, "--xgoef-chrome=0,dark,1") || !strings.Contains(args, "--force-dark-mode") ||
		strings.Contains(args, "--force-light-mode") {
		t.Fatalf("dark %s", args)
	}

	light := New(conf.File{App: conf.AppSection{Windows: []conf.Window{{
		Label: "main", Theme: "light", Width: 800, Height: 600,
	}}}}, nil)
	args = strings.Join(light.buildArgs("about:blank"), "\n")
	if !strings.Contains(args, "--xgoef-chrome=1,light,0") || !strings.Contains(args, "--force-light-mode") ||
		strings.Contains(args, "--force-dark-mode") {
		t.Fatalf("light %s", args)
	}

	system := New(conf.File{App: conf.AppSection{Windows: []conf.Window{{
		Label: "main", Width: 800, Height: 600,
	}}}}, nil)
	args = strings.Join(system.buildArgs("about:blank"), "\n")
	if !strings.Contains(args, "--xgoef-chrome=1,system,0") || strings.Contains(args, "--force-dark-mode") ||
		strings.Contains(args, "--force-light-mode") {
		t.Fatalf("system %s", args)
	}
}

func TestCreateWindowChrome(t *testing.T) {
	app := New(conf.File{}, nil)
	br := app.Bridge.(*cef.NativeBridge)
	spec := cef.WindowSpec{
		Label: "aux", Title: "t", Width: 100, Height: 80,
		Chrome: cef.WindowChrome{Frameless: true, Theme: "dark", Transparent: true},
	}
	if err := app.CreateWindow(spec); err != nil {
		t.Fatal(err)
	}
	got := br.LastCreate()
	if !got.Chrome.Frameless || got.Chrome.Theme != "dark" || !got.Chrome.Transparent {
		t.Fatalf("%+v", got.Chrome)
	}
	enc, err := got.Chrome.Encode()
	if err != nil || enc != "0,dark,1" {
		t.Fatalf("%q %v", enc, err)
	}
	if err := app.CreateWindow(cef.WindowSpec{Chrome: cef.WindowChrome{Theme: "blue"}}); err == nil {
		t.Fatal("accepted blue")
	}
}
