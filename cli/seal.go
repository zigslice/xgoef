package cli

import (
	"crypto/ed25519"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/zigslice/xgoef/asset"
	"github.com/zigslice/xgoef/conf"
	"github.com/zigslice/xgoef/update"
)

// sealStaged packs outDir/web, signs it, and appends the trailer to the host
// binary. The public key is written into the staged xgoef.conf.json.
// The private key stays in the app tree (.xgoef/asset.key) or XGOEF_ASSET_KEY.
func sealStaged(appDir, outDir, binPath string) error {
	keyPath := os.Getenv("XGOEF_ASSET_KEY")
	if keyPath == "" {
		keyPath = filepath.Join(appDir, ".xgoef", "asset.key")
	}
	priv, err := asset.EnsureKey(keyPath)
	if err != nil {
		return err
	}
	payload, err := asset.PackDir(filepath.Join(outDir, "web"))
	if err != nil {
		return err
	}
	assetsPath := filepath.Join(outDir, "xgoef.assets")
	if err := asset.WriteSealFile(assetsPath, payload, priv); err != nil {
		return err
	}
	if err := writeUpdateManifest(outDir, assetsPath, priv); err != nil {
		return err
	}
	st, err := os.Stat(binPath)
	if err != nil || st.IsDir() {
		return fmt.Errorf("seal: host binary %s missing", binPath)
	}
	if err := asset.AppendSeal(binPath, payload, priv); err != nil {
		return err
	}
	pub, ok := priv.Public().(ed25519.PublicKey)
	if !ok {
		return fmt.Errorf("seal: private key has no ed25519 public key")
	}
	return writeAssetPublicKey(filepath.Join(outDir, "xgoef.conf.json"), pub)
}

func writeUpdateManifest(outDir, assetsPath string, priv ed25519.PrivateKey) error {
	cfg, err := conf.Load(filepath.Join(outDir, "xgoef.conf.json"))
	if err != nil {
		return err
	}
	if cfg.Version == "" {
		return nil
	}
	sealed, err := os.ReadFile(assetsPath)
	if err != nil {
		return err
	}
	return update.WriteDir(outDir, cfg.Version, "", sealed, priv)
}

func writeAssetPublicKey(confPath string, pub ed25519.PublicKey) error {
	cfg, err := conf.Load(confPath)
	if err != nil {
		return err
	}
	cfg.Build.AssetPublicKey = asset.PublicKeyString(pub)
	raw, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	raw = append(raw, '\n')
	return os.WriteFile(confPath, raw, 0o644)
}
