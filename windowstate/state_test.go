package windowstate

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadNoteAndEncode(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".xgoef", "window-state.json")
	st, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := st.Get("main"); ok {
		t.Fatal("missing file returned a place")
	}
	if err := st.Note("main", 10, 20, 800, 600, false, false); err != nil {
		t.Fatal(err)
	}
	if err := st.Note("aux", 1, 2, 3, 4, false, false); err != nil {
		t.Fatal(err)
	}
	if err := st.Note("main", 0, 0, 0, 0, true, false); err != nil {
		t.Fatal(err)
	}
	p, ok := st.Get("main")
	if !ok || !p.Maximized || p.Minimized || p.Width != 800 || p.Height != 600 || p.X != 10 || p.Y != 20 {
		t.Fatalf("kept size %+v", p)
	}
	const want = "aux\t1\t2\t3\t4\tp\n" +
		"main\t10\t20\t800\t600\tpm\n"
	if st.Encode() != want {
		t.Fatalf("encode:\n%q", st.Encode())
	}
	if err := st.Note("main", 0, 0, 0, 0, false, true); err != nil {
		t.Fatal(err)
	}
	p, ok = st.Get("main")
	if !ok || !p.Maximized || !p.Minimized || p.Width != 800 || p.Height != 600 || p.X != 10 || p.Y != 20 {
		t.Fatalf("minimized %+v", p)
	}
	const wantMin = "aux\t1\t2\t3\t4\tp\n" +
		"main\t10\t20\t800\t600\tpmn\n"
	if st.Encode() != wantMin {
		t.Fatalf("encode min:\n%q", st.Encode())
	}
	again, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if again.Encode() != wantMin {
		t.Fatalf("reload:\n%q", again.Encode())
	}
	if err := st.Note("main", 30, 40, 900, 700, false, false); err != nil {
		t.Fatal(err)
	}
	p, ok = st.Get("main")
	if !ok || p.Maximized || p.Minimized || p.Width != 900 || p.Height != 700 || p.X != 30 || p.Y != 40 {
		t.Fatalf("restored %+v", p)
	}
	if !strings.Contains(st.Encode(), "main\t30\t40\t900\t700\tp\n") {
		t.Fatalf("encode normal:\n%q", st.Encode())
	}
	if err := st.Note("main", 0, 0, 0, 0, false, false); err == nil {
		t.Fatal("zero size accepted")
	}
	if err := st.Note("bad\tlabel", 1, 2, 3, 4, false, false); err == nil {
		t.Fatal("tab label accepted")
	}
}

func TestLoadDropsBadEntries(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "window-state.json")
	body := `{
	  "windows": {
	    "main": {"x": 1, "y": 2, "width": 3, "height": 4},
	    "huge": {"x": 0, "y": 0, "width": 999999, "height": 10},
	    "max": {"x": 0, "y": 0, "width": 0, "height": 0, "maximized": true},
	    "min": {"x": 0, "y": 0, "width": 0, "height": 0, "minimized": true}
	  }
	}`
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	st, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := st.Get("huge"); ok {
		t.Fatal("huge kept")
	}
	if _, ok := st.Get("main"); !ok {
		t.Fatal("main dropped")
	}
	if p, ok := st.Get("max"); !ok || !p.Maximized || p.Minimized || p.Positioned() {
		t.Fatalf("max %+v %v", p, ok)
	}
	if p, ok := st.Get("min"); !ok || p.Maximized || !p.Minimized || p.Positioned() {
		t.Fatalf("min %+v %v", p, ok)
	}
	enc := st.Encode()
	if !strings.Contains(enc, "max\t0\t0\t0\t0\tm\n") || !strings.Contains(enc, "min\t0\t0\t0\t0\tn\n") {
		t.Fatalf("encode:\n%s", enc)
	}
}

func TestMinimizeOnlyAndMaximizeClearsIt(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "window-state.json")
	st, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := st.Note("main", 0, 0, 0, 0, false, true); err != nil {
		t.Fatal(err)
	}
	if st.Encode() != "main\t0\t0\t0\t0\tn\n" {
		t.Fatalf("encode:\n%q", st.Encode())
	}
	if err := st.Note("main", 10, 20, 800, 600, false, false); err != nil {
		t.Fatal(err)
	}
	if err := st.Note("main", 0, 0, 0, 0, false, true); err != nil {
		t.Fatal(err)
	}
	if err := st.Note("main", 0, 0, 0, 0, true, false); err != nil {
		t.Fatal(err)
	}
	p, ok := st.Get("main")
	if !ok || !p.Maximized || p.Minimized || p.Width != 800 || p.Height != 600 {
		t.Fatalf("maximize after minimize %+v", p)
	}
	if st.Encode() != "main\t10\t20\t800\t600\tpm\n" {
		t.Fatalf("encode:\n%q", st.Encode())
	}
}

func TestLoadRejectsBrokenJSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "window-state.json")
	if err := os.WriteFile(path, []byte("{"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(path); err == nil {
		t.Fatal("broken json accepted")
	}
}
