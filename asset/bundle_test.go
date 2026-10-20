package asset

import (
	"archive/zip"
	"bytes"
	"crypto/ed25519"
	"crypto/rand"
	"os"
	"path/filepath"
	"testing"
)

func TestSealRoundTrip(t *testing.T) {
	dir := t.TempDir()
	web := filepath.Join(dir, "web")
	if err := os.MkdirAll(filepath.Join(web, "css"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(web, "index.html"), []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(web, "css", "app.css"), []byte("body{}"), 0o644); err != nil {
		t.Fatal(err)
	}
	payload, err := PackDir(web)
	if err != nil {
		t.Fatal(err)
	}
	again, err := PackDir(web)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(payload, again) {
		t.Fatal("pack is not deterministic")
	}
	_, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	bin := filepath.Join(dir, "xgoef-hello")
	if err := os.WriteFile(bin, []byte("ELF-host"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := AppendSeal(bin, payload, priv); err != nil {
		t.Fatal(err)
	}
	if err := AppendSeal(bin, payload, priv); err != nil {
		t.Fatal(err)
	}
	pub := priv.Public().(ed25519.PublicKey)
	got, err := ReadSealedPayload(bin, pub)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, payload) {
		t.Fatal("payload mismatch")
	}
	body, err := os.ReadFile(bin)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.HasPrefix(body, []byte("ELF-host")) {
		t.Fatalf("host prefix lost: %q", body[:16])
	}
	out := filepath.Join(dir, "out")
	if err := ExtractZip(got, out); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(filepath.Join(out, "css", "app.css"))
	if err != nil || string(b) != "body{}" {
		t.Fatalf("css %q %v", b, err)
	}
	keyPath := filepath.Join(dir, ".xgoef", "asset.key")
	stored, err := EnsureKey(keyPath)
	if err != nil {
		t.Fatal(err)
	}
	againKey, err := EnsureKey(keyPath)
	if err != nil || !bytes.Equal(stored, againKey) {
		t.Fatal("key not stable")
	}
	st, err := os.Stat(keyPath)
	if err != nil || st.Mode().Perm() != 0o600 {
		t.Fatalf("key mode %v", st.Mode())
	}
}

func TestSealRejectsTamper(t *testing.T) {
	_, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	pub := priv.Public().(ed25519.PublicKey)
	dir := t.TempDir()
	path := filepath.Join(dir, "xgoef.assets")
	if err := WriteSealFile(path, []byte("not-a-zip-but-signed"), priv); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	data[0] ^= 0xff
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := ReadSealedPayload(path, pub); err == nil || !bytes.Contains([]byte(err.Error()), []byte("signature")) {
		t.Fatalf("got %v", err)
	}
	other, _, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	fresh := filepath.Join(dir, "ok.assets")
	if err := WriteSealFile(fresh, []byte("payload-bytes"), priv); err != nil {
		t.Fatal(err)
	}
	if _, err := ReadSealedPayload(fresh, other); err == nil {
		t.Fatal("wrong key accepted")
	}
}

func TestExtractRejectsEscape(t *testing.T) {
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	w, err := zw.Create("../evil.txt")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := w.Write([]byte("no")); err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	dest := filepath.Join(t.TempDir(), "out")
	if err := ExtractZip(buf.Bytes(), dest); err == nil || !bytes.Contains([]byte(err.Error()), []byte("outside")) {
		t.Fatalf("got %v", err)
	}
	if _, err := os.Stat(filepath.Join(filepath.Dir(dest), "evil.txt")); !os.IsNotExist(err) {
		t.Fatal("escape file written")
	}
}
