// Package update publishes and applies a signed frontend update.
// The asset is a standalone xgoef.assets seal. The manifest is a separate
// Ed25519 signature over the version, notes, and asset hash.
package update

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/zigslice/xgoef/asset"
)

const (
	manifestName = "update.json"
	assetName    = "xgoef.assets"
	messageHead  = "xgoef-update-v1\n"
	maxManifest  = 64 << 10
	maxNotes     = 4096
)

// Manifest is the file an update endpoint serves next to xgoef.assets.
type Manifest struct {
	Version   string `json:"version"`
	Notes     string `json:"notes,omitempty"`
	SHA256    string `json:"sha256"`
	Signature string `json:"signature"`
}

// Offer is a verified update newer than the running version.
type Offer struct {
	Version string `json:"version"`
	Notes   string `json:"notes,omitempty"`
	Current string `json:"current"`
}

// Sign builds a manifest for a standalone seal.
func Sign(version, notes string, sealed []byte, priv ed25519.PrivateKey) (Manifest, error) {
	sum := hashAsset(sealed)
	msg, err := message(version, notes, sum)
	if err != nil {
		return Manifest{}, err
	}
	if len(priv) != ed25519.PrivateKeySize {
		return Manifest{}, fmt.Errorf("updater: private key must be %d bytes", ed25519.PrivateKeySize)
	}
	sig := ed25519.Sign(priv, msg)
	return Manifest{
		Version:   version,
		Notes:     notes,
		SHA256:    sum,
		Signature: base64.StdEncoding.EncodeToString(sig),
	}, nil
}

// Verify checks the manifest signature, the asset hash, and the seal trailer.
func Verify(m Manifest, sealed []byte, pub ed25519.PublicKey) error {
	sum := hashAsset(sealed)
	if m.SHA256 != sum {
		return fmt.Errorf("updater: asset hash mismatch")
	}
	msg, err := message(m.Version, m.Notes, m.SHA256)
	if err != nil {
		return err
	}
	sig, err := base64.StdEncoding.DecodeString(m.Signature)
	if err != nil || len(sig) != ed25519.SignatureSize {
		return fmt.Errorf("updater: signature is not %d bytes", ed25519.SignatureSize)
	}
	if len(pub) != ed25519.PublicKeySize || !ed25519.Verify(pub, msg, sig) {
		return fmt.Errorf("updater: signature mismatch")
	}
	if _, err := asset.VerifyStandalone(sealed, pub); err != nil {
		return err
	}
	return nil
}

// Check verifies the bundle and reports it when version is newer than current.
// An older or equal version is not an error.
func Check(current string, m Manifest, sealed []byte, pub ed25519.PublicKey) (Offer, bool, error) {
	if err := Verify(m, sealed, pub); err != nil {
		return Offer{}, false, err
	}
	newer, err := Newer(current, m.Version)
	if err != nil {
		return Offer{}, false, err
	}
	if !newer {
		return Offer{}, false, nil
	}
	return Offer{Version: m.Version, Notes: m.Notes, Current: current}, true, nil
}

// Newer reports whether next sorts after current. Both must be dotted integers.
func Newer(current, next string) (bool, error) {
	a, err := parseVersion(current)
	if err != nil {
		return false, err
	}
	b, err := parseVersion(next)
	if err != nil {
		return false, err
	}
	n := len(a)
	if len(b) > n {
		n = len(b)
	}
	for i := 0; i < n; i++ {
		av, bv := 0, 0
		if i < len(a) {
			av = a[i]
		}
		if i < len(b) {
			bv = b[i]
		}
		if av < bv {
			return true, nil
		}
		if av > bv {
			return false, nil
		}
	}
	return false, nil
}

// WriteDir writes update.json beside an existing xgoef.assets in dir.
func WriteDir(dir, version, notes string, sealed []byte, priv ed25519.PrivateKey) error {
	m, err := Sign(version, notes, sealed, priv)
	if err != nil {
		return err
	}
	raw, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	raw = append(raw, '\n')
	return os.WriteFile(filepath.Join(dir, manifestName), raw, 0o644)
}

// Resolve turns a configured endpoint into a directory path or an https URL.
// Relative paths are joined to base. http URLs are rejected.
func Resolve(endpoint, base string) (string, error) {
	endpoint = strings.TrimSpace(endpoint)
	if endpoint == "" {
		return "", fmt.Errorf("updater: endpoint is not configured")
	}
	if strings.HasPrefix(endpoint, "https://") {
		u, err := url.Parse(endpoint)
		if err != nil || u.Host == "" || u.User != nil {
			return "", fmt.Errorf("updater: bad https endpoint")
		}
		return endpoint, nil
	}
	if strings.HasPrefix(endpoint, "http://") {
		return "", fmt.Errorf("updater: endpoint must be https or a directory")
	}
	p := endpoint
	if strings.HasPrefix(p, "file://") {
		p = strings.TrimPrefix(p, "file://")
	}
	if !filepath.IsAbs(p) {
		if base == "" {
			return "", fmt.Errorf("updater: relative endpoint without an app directory")
		}
		p = filepath.Join(base, p)
	}
	return filepath.Clean(p), nil
}

// Fetch reads a directory endpoint or an https update.json URL.
func Fetch(endpoint string) (Manifest, []byte, error) {
	return FetchHTTP(endpoint, nil)
}

// FetchHTTP is Fetch with a caller-supplied client. A nil client uses the default.
func FetchHTTP(endpoint string, client *http.Client) (Manifest, []byte, error) {
	if strings.HasPrefix(endpoint, "https://") {
		return fetchHTTPS(endpoint, client)
	}
	return loadDir(endpoint)
}

// Install replaces the host trailer and the sibling seal with sealed.
func Install(hostPath, siblingPath string, sealed []byte, pub ed25519.PublicKey) error {
	if hostPath == "" {
		return fmt.Errorf("updater: host binary is not set")
	}
	return asset.InstallSeal(hostPath, siblingPath, sealed, pub)
}

func loadDir(dir string) (Manifest, []byte, error) {
	raw, err := os.ReadFile(filepath.Join(dir, manifestName))
	if err != nil {
		return Manifest{}, nil, err
	}
	if len(raw) > maxManifest {
		return Manifest{}, nil, fmt.Errorf("updater: manifest is too large")
	}
	sealed, err := os.ReadFile(filepath.Join(dir, assetName))
	if err != nil {
		return Manifest{}, nil, err
	}
	if len(sealed) > asset.MaxSealBytes+128 {
		return Manifest{}, nil, fmt.Errorf("updater: asset is too large")
	}
	var m Manifest
	if err := json.Unmarshal(raw, &m); err != nil {
		return Manifest{}, nil, fmt.Errorf("updater: %w", err)
	}
	return m, sealed, nil
}

func fetchHTTPS(manifestURL string, client *http.Client) (Manifest, []byte, error) {
	u, err := url.Parse(manifestURL)
	if err != nil || u.Scheme != "https" || u.Host == "" || u.User != nil {
		return Manifest{}, nil, fmt.Errorf("updater: bad https endpoint")
	}
	if client == nil {
		client = http.DefaultClient
	}
	c := *client
	if c.Timeout == 0 {
		c.Timeout = 30 * time.Second
	}
	c.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		if len(via) >= 5 {
			return fmt.Errorf("updater: too many redirects")
		}
		if req.URL.Scheme != "https" || req.URL.User != nil {
			return fmt.Errorf("updater: redirect left https")
		}
		return nil
	}
	body, err := getLimited(&c, manifestURL, maxManifest)
	if err != nil {
		return Manifest{}, nil, err
	}
	var m Manifest
	if err := json.Unmarshal(body, &m); err != nil {
		return Manifest{}, nil, fmt.Errorf("updater: %w", err)
	}
	assetURL := *u
	assetURL.Path = path.Join(path.Dir(u.Path), assetName)
	assetURL.RawQuery = ""
	assetURL.Fragment = ""
	sealed, err := getLimited(&c, assetURL.String(), asset.MaxSealBytes+128)
	if err != nil {
		return Manifest{}, nil, err
	}
	return m, sealed, nil
}

func getLimited(client *http.Client, rawURL string, limit int64) ([]byte, error) {
	resp, err := client.Get(rawURL)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("updater: %s returned %s", rawURL, resp.Status)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, limit+1))
	if err != nil {
		return nil, err
	}
	if int64(len(body)) > limit {
		return nil, fmt.Errorf("updater: response too large")
	}
	return body, nil
}

func hashAsset(sealed []byte) string {
	sum := sha256.Sum256(sealed)
	return hex.EncodeToString(sum[:])
}

func message(version, notes, sum string) ([]byte, error) {
	if _, err := parseVersion(version); err != nil {
		return nil, err
	}
	if strings.ContainsAny(notes, "\r\n") || len(notes) > maxNotes {
		return nil, fmt.Errorf("updater: notes must be a single line")
	}
	if len(sum) != sha256.Size*2 || strings.ToLower(sum) != sum {
		return nil, fmt.Errorf("updater: sha256 must be %d lowercase hex characters", sha256.Size*2)
	}
	if _, err := hex.DecodeString(sum); err != nil {
		return nil, fmt.Errorf("updater: sha256 must be %d lowercase hex characters", sha256.Size*2)
	}
	return []byte(messageHead + version + "\n" + notes + "\n" + sum + "\n"), nil
}

func parseVersion(s string) ([]int, error) {
	if s == "" || len(s) > 64 {
		return nil, fmt.Errorf("updater: bad version %q", s)
	}
	parts := strings.Split(s, ".")
	if len(parts) > 8 {
		return nil, fmt.Errorf("updater: bad version %q", s)
	}
	out := make([]int, len(parts))
	for i, part := range parts {
		if part == "" || len(part) > 9 {
			return nil, fmt.Errorf("updater: bad version %q", s)
		}
		for _, r := range part {
			if r < '0' || r > '9' {
				return nil, fmt.Errorf("updater: bad version %q", s)
			}
		}
		n, err := strconv.Atoi(part)
		if err != nil {
			return nil, fmt.Errorf("updater: bad version %q", s)
		}
		out[i] = n
	}
	return out, nil
}
