package runtime

import (
	"crypto/ed25519"
	"crypto/rand"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/zigslice/xgoef/acl"
	"github.com/zigslice/xgoef/asset"
	"github.com/zigslice/xgoef/conf"
	"github.com/zigslice/xgoef/update"
)

func TestUpdaterFromConfig(t *testing.T) {
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	oldDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(oldDir, "index.html"), []byte("old"), 0o644); err != nil {
		t.Fatal(err)
	}
	oldPayload, err := asset.PackDir(oldDir)
	if err != nil {
		t.Fatal(err)
	}
	newDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(newDir, "index.html"), []byte("new"), 0o644); err != nil {
		t.Fatal(err)
	}
	newPayload, err := asset.PackDir(newDir)
	if err != nil {
		t.Fatal(err)
	}
	endpoint := t.TempDir()
	sealedPath := filepath.Join(endpoint, "xgoef.assets")
	if err := asset.WriteSealFile(sealedPath, newPayload, priv); err != nil {
		t.Fatal(err)
	}
	sealed, err := os.ReadFile(sealedPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := update.WriteDir(endpoint, "0.0.2", "ship", sealed, priv); err != nil {
		t.Fatal(err)
	}
	appDir := t.TempDir()
	host := filepath.Join(appDir, "xgoef-hello")
	if err := os.WriteFile(host, []byte("bin"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := asset.AppendSeal(host, oldPayload, priv); err != nil {
		t.Fatal(err)
	}
	app := New(conf.File{
		Version: "0.0.1",
		Build:   conf.Build{AssetPublicKey: asset.PublicKeyString(pub)},
		Plugins: []string{"updater"},
		Plugin:  conf.PluginSection{Updater: conf.UpdaterConfig{Endpoint: endpoint}},
		Capabilities: []acl.Capability{{
			Identifier:  "main",
			Windows:     []string{"main"},
			Permissions: []string{"updater:check", "updater:install"},
		}},
	}, nil)
	app.SetBase(appDir)
	app.SetExecutable(host)
	if err := app.OpenPlugins(); err != nil {
		t.Fatal(err)
	}
	if err := app.Plugins.Setup(app); err != nil {
		t.Fatal(err)
	}
	checked := handleMessage(app.ACL, app.Router, nil, "main",
		`{"type":"invoke","id":1,"cmd":"updater:check","args":{}}`)
	if !strings.Contains(checked, `"ok":true`) || !strings.Contains(checked, "0.0.2") {
		t.Fatalf("check %s", checked)
	}
	installed := handleMessage(app.ACL, app.Router, nil, "main",
		`{"type":"invoke","id":2,"cmd":"updater:install","args":{}}`)
	if !strings.Contains(installed, `"restart":true`) {
		t.Fatalf("install %s", installed)
	}
	body, err := asset.ReadSealedPayload(host, pub)
	if err != nil || string(body) != string(newPayload) {
		t.Fatalf("payload %d %v", len(body), err)
	}
}
