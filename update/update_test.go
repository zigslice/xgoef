package update

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/zigslice/xgoef/asset"
)

func sealOf(t *testing.T, priv ed25519.PrivateKey, payload string) []byte {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "xgoef.assets")
	if err := asset.WriteSealFile(path, []byte(payload), priv); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func TestCheckInstallAndReject(t *testing.T) {
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	oldSeal := sealOf(t, priv, "old")
	newSeal := sealOf(t, priv, "new")
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "xgoef.assets"), newSeal, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := WriteDir(dir, "0.0.2", "fixes", newSeal, priv); err != nil {
		t.Fatal(err)
	}
	m, sealed, err := Fetch(dir)
	if err != nil {
		t.Fatal(err)
	}
	offer, ok, err := Check("0.0.1", m, sealed, pub)
	if err != nil || !ok || offer.Version != "0.0.2" || offer.Notes != "fixes" {
		t.Fatalf("%+v %v %v", offer, ok, err)
	}
	if _, ok, err := Check("0.0.2", m, sealed, pub); err != nil || ok {
		t.Fatalf("same version %v %v", ok, err)
	}
	if _, ok, err := Check("1.0.0", m, sealed, pub); err != nil || ok {
		t.Fatalf("downgrade %v %v", ok, err)
	}
	m.Notes = "tampered"
	if _, _, err := Check("0.0.1", m, sealed, pub); err == nil {
		t.Fatal("tampered notes accepted")
	}

	host := filepath.Join(t.TempDir(), "xgoef-hello")
	if err := os.WriteFile(host, []byte("host"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := asset.AppendSeal(host, []byte("old"), priv); err != nil {
		t.Fatal(err)
	}
	sibling := filepath.Join(filepath.Dir(host), "xgoef.assets")
	if err := Install(host, sibling, oldSeal, pub); err != nil {
		t.Fatal(err)
	}
	if err := Install(host, sibling, newSeal, pub); err != nil {
		t.Fatal(err)
	}
	got, err := asset.ReadSealedPayload(host, pub)
	if err != nil || string(got) != "new" {
		t.Fatalf("host %q %v", got, err)
	}
	sib, err := asset.ReadSealedPayload(sibling, pub)
	if err != nil || string(sib) != "new" {
		t.Fatalf("sibling %q %v", sib, err)
	}
	if _, err := Resolve("http://example.com/update.json", ""); err == nil {
		t.Fatal("http endpoint accepted")
	}
}

func TestFetchHTTPS(t *testing.T) {
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	sealed := sealOf(t, priv, "remote")
	m, err := Sign("0.2.0", "", sealed, priv)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := json.Marshal(m)
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/app/update.json":
			_, _ = w.Write(raw)
		case "/app/xgoef.assets":
			_, _ = w.Write(sealed)
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()
	got, body, err := FetchHTTP(srv.URL+"/app/update.json", srv.Client())
	if err != nil {
		t.Fatal(err)
	}
	offer, ok, err := Check("0.1.0", got, body, pub)
	if err != nil || !ok || offer.Version != "0.2.0" {
		t.Fatalf("%+v %v %v", offer, ok, err)
	}
}
