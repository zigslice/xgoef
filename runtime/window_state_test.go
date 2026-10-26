package runtime

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/zigslice/xgoef/cef"
	"github.com/zigslice/xgoef/conf"
)

func TestWindowStateRemembered(t *testing.T) {
	dir := t.TempDir()
	app := New(conf.File{
		App: conf.AppSection{Windows: []conf.Window{{
			Label: "main", Title: "t", Width: 960, Height: 640,
		}}},
	}, nil)
	app.SetBase(dir)
	if err := app.OpenWindowState(); err != nil {
		t.Fatal(err)
	}
	if err := app.noteWindow("main", 10, 20, 800, 600, false, false); err != nil {
		t.Fatal(err)
	}
	if err := app.noteWindow("main", 0, 0, 0, 0, true, false); err != nil {
		t.Fatal(err)
	}
	br := app.Bridge.(*cef.NativeBridge)
	const maximized = "main\t10\t20\t800\t600\tpm\n"
	if br.WindowState() != maximized {
		t.Fatalf("bridge %q", br.WindowState())
	}
	if err := app.noteWindow("main", 0, 0, 0, 0, false, true); err != nil {
		t.Fatal(err)
	}
	const line = "main\t10\t20\t800\t600\tpmn\n"
	if br.WindowState() != line {
		t.Fatalf("bridge min %q", br.WindowState())
	}
	body, err := os.ReadFile(filepath.Join(dir, ".xgoef", "window-state.json"))
	text := string(body)
	if err != nil || !strings.Contains(text, `"width": 800`) || !strings.Contains(text, `"maximized": true`) || !strings.Contains(text, `"minimized": true`) {
		t.Fatalf("file %s %v", body, err)
	}
	if err := app.CreateWindow(cef.WindowSpec{Label: "main", Title: "t", Width: 100, Height: 80}); err != nil {
		t.Fatal(err)
	}
	got := br.LastCreate()
	if got.Width != 800 || got.Height != 600 {
		t.Fatalf("create %+v", got)
	}
	args := strings.Join(app.buildArgs("xgoef://app/index.html"), "\n")
	if !strings.Contains(args, "--xgoef-size=800,600") {
		t.Fatalf("args %s", args)
	}

	off := false
	app.Config.App.WindowState = &off
	if err := app.OpenWindowState(); err != nil {
		t.Fatal(err)
	}
	if br.WindowState() != "" {
		t.Fatalf("still published %q", br.WindowState())
	}
	if err := app.noteWindow("main", 1, 2, 3, 4, false, false); err == nil {
		t.Fatal("note while off")
	}
	if err := app.CreateWindow(cef.WindowSpec{Label: "main", Width: 100, Height: 80}); err != nil {
		t.Fatal(err)
	}
	if br.LastCreate().Width != 100 {
		t.Fatalf("off %+v", br.LastCreate())
	}
}
