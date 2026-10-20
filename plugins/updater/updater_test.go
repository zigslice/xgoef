package updater

import (
	"crypto/ed25519"
	"crypto/rand"
	"os"
	"path/filepath"
	"testing"

	"github.com/zigslice/xgoef/asset"
	"github.com/zigslice/xgoef/ipc"
	"github.com/zigslice/xgoef/plugin"
	"github.com/zigslice/xgoef/update"
)

type host struct{ h map[string]ipc.Handler }

func (h *host) Register(name string, fn ipc.Handler) {
	if h.h == nil {
		h.h = map[string]ipc.Handler{}
	}
	h.h[name] = fn
}

func TestCheckAndInstall(t *testing.T) {
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	sealedPath := filepath.Join(dir, "xgoef.assets")
	if err := asset.WriteSealFile(sealedPath, []byte("next"), priv); err != nil {
		t.Fatal(err)
	}
	sealed, err := os.ReadFile(sealedPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := update.WriteDir(dir, "0.0.2", "notes", sealed, priv); err != nil {
		t.Fatal(err)
	}
	appDir := t.TempDir()
	hostPath := filepath.Join(appDir, "xgoef-hello")
	if err := os.WriteFile(hostPath, []byte("bin"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := asset.AppendSeal(hostPath, []byte("old"), priv); err != nil {
		t.Fatal(err)
	}
	p, err := New("0.0.1", dir, asset.PublicKeyString(pub), appDir, hostPath)
	if err != nil {
		t.Fatal(err)
	}
	h := &host{}
	if err := p.Setup(h); err != nil {
		t.Fatal(err)
	}
	out, err := h.h["updater:check"](ipc.Context{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	offer := out.(update.Offer)
	if offer.Version != "0.0.2" {
		t.Fatalf("%+v", offer)
	}
	installed, err := h.h["updater:install"](ipc.Context{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	got := installed.(map[string]any)
	if got["version"] != "0.0.2" || got["restart"] != true {
		t.Fatalf("%v", got)
	}
	body, err := asset.ReadSealedPayload(hostPath, pub)
	if err != nil || string(body) != "next" {
		t.Fatalf("%q %v", body, err)
	}
	if _, err := h.h["updater:install"](ipc.Context{}, nil); err == nil {
		t.Fatal("second install")
	}
}

func TestMissingEndpoint(t *testing.T) {
	p, err := New("0.0.1", "", "", "", "")
	if err != nil {
		t.Fatal(err)
	}
	h := &host{}
	if err := p.Setup(h); err != nil {
		t.Fatal(err)
	}
	if _, err := h.h["updater:check"](ipc.Context{}, nil); err == nil {
		t.Fatal("check without endpoint")
	}
	var _ plugin.Plugin = p
}
