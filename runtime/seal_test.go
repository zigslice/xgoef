package runtime

import (
	"crypto/ed25519"
	"crypto/rand"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/zigslice/xgoef/asset"
	"github.com/zigslice/xgoef/conf"
)

func TestSealedFrontendReplacesLooseDir(t *testing.T) {
	dir := t.TempDir()
	web := filepath.Join(dir, "web")
	if err := os.MkdirAll(web, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(web, "index.html"), []byte("loose"), 0o644); err != nil {
		t.Fatal(err)
	}
	sealedWeb := filepath.Join(dir, "srcweb")
	if err := os.MkdirAll(sealedWeb, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sealedWeb, "index.html"), []byte("sealed"), 0o644); err != nil {
		t.Fatal(err)
	}
	payload, err := asset.PackDir(sealedWeb)
	if err != nil {
		t.Fatal(err)
	}
	_, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	pub := priv.Public().(ed25519.PublicKey)
	host := filepath.Join(dir, "xgoef-hello")
	if err := os.WriteFile(host, []byte("host"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := asset.AppendSeal(host, payload, priv); err != nil {
		t.Fatal(err)
	}
	app := New(conf.File{Build: conf.Build{
		FrontendDist:   "web",
		AssetPublicKey: asset.PublicKeyString(pub),
	}}, nil)
	app.SetBase(dir)
	app.SetExecutable(host)
	got, err := app.NavigationURL("xgoef://app/index.html")
	if err != nil || got != "xgoef://app/index.html" {
		t.Fatalf("%q %v", got, err)
	}
	body, err := os.ReadFile(filepath.Join(app.assets.Dir(), "index.html"))
	if err != nil || string(body) != "sealed" {
		t.Fatalf("served %q %v", body, err)
	}
	args := strings.Join(app.buildArgs("xgoef://app/index.html"), "\n")
	if !strings.Contains(args, "--xgoef-asset-root="+app.assets.Dir()) {
		t.Fatalf("args %s", args)
	}

	_, other, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	if err := asset.WriteSealFile(filepath.Join(dir, "xgoef.assets"), payload, other); err != nil {
		t.Fatal(err)
	}
	bare := filepath.Join(dir, "bare")
	if err := os.WriteFile(bare, []byte("no-trailer"), 0o755); err != nil {
		t.Fatal(err)
	}
	app.SetExecutable(bare)
	if _, err := app.NavigationURL("xgoef://app/index.html"); err == nil || !strings.Contains(err.Error(), "signature") {
		t.Fatalf("bad sibling: %v", err)
	}
}

func TestMissingSealRefusesWhenKeyPresent(t *testing.T) {
	dir := t.TempDir()
	web := filepath.Join(dir, "web")
	if err := os.MkdirAll(web, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(web, "index.html"), []byte("loose"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	host := filepath.Join(dir, "host")
	if err := os.WriteFile(host, []byte("host"), 0o755); err != nil {
		t.Fatal(err)
	}
	app := New(conf.File{Build: conf.Build{
		FrontendDist:   "web",
		AssetPublicKey: asset.PublicKeyString(priv.Public().(ed25519.PublicKey)),
	}}, nil)
	app.SetBase(dir)
	app.SetExecutable(host)
	if _, err := app.NavigationURL("xgoef://app/index.html"); err == nil || !strings.Contains(err.Error(), "missing") {
		t.Fatalf("got %v", err)
	}
}
