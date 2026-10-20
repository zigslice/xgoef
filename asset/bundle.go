package asset

import (
	"archive/zip"
	"bytes"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// SealMagic marks a signed frontend trailer. It sits at the end of the
// host binary and of the standalone xgoef.assets file.
const SealMagic = "XGOEFAS1"

// MaxSealBytes is the largest frontend archive we will read back.
const MaxSealBytes = 256 << 20

const (
	sealSigLen = ed25519.SignatureSize
	sealLenLen = 8
	sealMagLen = len(SealMagic)
	sealTail   = sealSigLen + sealLenLen + sealMagLen
)

// ErrNoSeal means the file has no asset trailer.
var ErrNoSeal = fmt.Errorf("asset: no sealed frontend")

// zipEpoch is the earliest timestamp the zip format stores reliably.
var zipEpoch = time.Date(1980, 1, 1, 0, 0, 0, 0, time.UTC)

// PackDir archives the regular files under dir. Names are slash-separated
// and relative. The bytes are deterministic for a given tree.
func PackDir(dir string) ([]byte, error) {
	var names []string
	err := filepath.WalkDir(dir, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		if d.Type()&os.ModeSymlink != 0 {
			return fmt.Errorf("asset: %s is a symlink", p)
		}
		rel, err := filepath.Rel(dir, p)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		if !cleanZipName(rel) {
			return fmt.Errorf("asset: path %q is outside the frontend root", rel)
		}
		names = append(names, rel)
		return nil
	})
	if err != nil {
		return nil, err
	}
	if len(names) == 0 {
		return nil, fmt.Errorf("asset: frontend %s is empty", dir)
	}
	sort.Strings(names)
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for _, name := range names {
		body, err := os.ReadFile(filepath.Join(dir, filepath.FromSlash(name)))
		if err != nil {
			return nil, err
		}
		h := &zip.FileHeader{Name: name, Method: zip.Store, Modified: zipEpoch}
		w, err := zw.CreateHeader(h)
		if err != nil {
			return nil, err
		}
		if _, err := w.Write(body); err != nil {
			return nil, err
		}
	}
	if err := zw.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// EnsureKey reads a raw Ed25519 private key, or generates one at path.
func EnsureKey(path string) (ed25519.PrivateKey, error) {
	b, err := os.ReadFile(path)
	if err == nil {
		if len(b) != ed25519.PrivateKeySize {
			return nil, fmt.Errorf("asset: %s is not a %d-byte ed25519 private key", path, ed25519.PrivateKeySize)
		}
		return ed25519.PrivateKey(b), nil
	}
	if !os.IsNotExist(err) {
		return nil, err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}
	_, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil, err
	}
	if err := os.WriteFile(path, priv, 0o600); err != nil {
		return nil, err
	}
	return priv, nil
}

// PublicKeyString encodes a public key for xgoef.conf.json.
func PublicKeyString(pub ed25519.PublicKey) string {
	return base64.StdEncoding.EncodeToString(pub)
}

// ParsePublicKey decodes a key written by PublicKeyString.
func ParsePublicKey(s string) (ed25519.PublicKey, error) {
	b, err := base64.StdEncoding.DecodeString(strings.TrimSpace(s))
	if err != nil || len(b) != ed25519.PublicKeySize {
		return nil, fmt.Errorf("asset: assetPublicKey must be %d bytes", ed25519.PublicKeySize)
	}
	return ed25519.PublicKey(b), nil
}

// AppendSeal strips any previous trailer and appends a signed archive.
// The file mode is preserved.
func AppendSeal(path string, payload []byte, priv ed25519.PrivateKey) error {
	st, err := os.Stat(path)
	if err != nil {
		return err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	body, _, err := splitTrailer(data)
	if err != nil && err != ErrNoSeal {
		return err
	}
	if err == ErrNoSeal {
		body = data
	}
	blob, err := encodeSeal(payload, priv)
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(body, blob...), st.Mode().Perm())
}

// WriteSealFile writes a standalone signed archive (the same trailer bytes
// that AppendSeal adds to a host binary).
func WriteSealFile(path string, payload []byte, priv ed25519.PrivateKey) error {
	blob, err := encodeSeal(payload, priv)
	if err != nil {
		return err
	}
	return os.WriteFile(path, blob, 0o644)
}

// ReadSealedPayload verifies the trailer at the end of path and returns the
// zip bytes. A file with no trailer returns ErrNoSeal.
func ReadSealedPayload(path string, pub ed25519.PublicKey) ([]byte, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	_, payload, err := splitTrailer(data)
	if err != nil {
		return nil, err
	}
	if err := verifyPayload(payload, data, pub); err != nil {
		return nil, err
	}
	return payload, nil
}

func verifyPayload(payload, file []byte, pub ed25519.PublicKey) error {
	if len(pub) != ed25519.PublicKeySize {
		return fmt.Errorf("asset: assetPublicKey must be %d bytes", ed25519.PublicKeySize)
	}
	sig := file[len(file)-sealTail : len(file)-sealTail+sealSigLen]
	if !ed25519.Verify(pub, payload, sig) {
		return fmt.Errorf("asset: frontend signature mismatch")
	}
	return nil
}

// ExtractZip unpacks a payload from PackDir into dest.
// Names that would leave dest are rejected.
func ExtractZip(payload []byte, dest string) error {
	abs, err := filepath.Abs(dest)
	if err != nil {
		return err
	}
	if err := os.RemoveAll(abs); err != nil {
		return err
	}
	if err := os.MkdirAll(abs, 0o755); err != nil {
		return err
	}
	r, err := zip.NewReader(bytes.NewReader(payload), int64(len(payload)))
	if err != nil {
		return fmt.Errorf("asset: %w", err)
	}
	for _, f := range r.File {
		name := f.Name
		if !cleanZipName(name) {
			return fmt.Errorf("asset: path %q is outside the frontend root", name)
		}
		if f.FileInfo().IsDir() || f.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("asset: path %q is outside the frontend root", name)
		}
		target := filepath.Join(abs, filepath.FromSlash(path.Clean(name)))
		if !within(target, abs) {
			return fmt.Errorf("asset: path %q is outside the frontend root", name)
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		rc, err := f.Open()
		if err != nil {
			return err
		}
		err = writeZipFile(target, rc)
		rc.Close()
		if err != nil {
			return err
		}
	}
	return nil
}

func writeZipFile(target string, r io.Reader) error {
	out, err := os.OpenFile(target, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.Copy(out, r)
	return err
}

func encodeSeal(payload []byte, priv ed25519.PrivateKey) ([]byte, error) {
	if len(priv) != ed25519.PrivateKeySize {
		return nil, fmt.Errorf("asset: private key must be %d bytes", ed25519.PrivateKeySize)
	}
	if len(payload) == 0 || len(payload) > MaxSealBytes {
		return nil, fmt.Errorf("asset: frontend archive size %d", len(payload))
	}
	sig := ed25519.Sign(priv, payload)
	var buf bytes.Buffer
	buf.Write(payload)
	buf.Write(sig)
	var n [8]byte
	binary.LittleEndian.PutUint64(n[:], uint64(len(payload)))
	buf.Write(n[:])
	buf.WriteString(SealMagic)
	return buf.Bytes(), nil
}

// splitTrailer returns the bytes before the trailer and the zip payload.
func splitTrailer(data []byte) (body, payload []byte, err error) {
	if len(data) < sealTail || string(data[len(data)-sealMagLen:]) != SealMagic {
		return nil, nil, ErrNoSeal
	}
	n := binary.LittleEndian.Uint64(data[len(data)-sealMagLen-sealLenLen : len(data)-sealMagLen])
	if n == 0 || n > MaxSealBytes || int(n) > len(data)-sealTail {
		return nil, nil, fmt.Errorf("asset: sealed frontend is truncated")
	}
	end := len(data) - sealTail
	start := end - int(n)
	return data[:start], data[start:end], nil
}

func cleanZipName(name string) bool {
	if name == "" || strings.Contains(name, `\`) || strings.ContainsRune(name, 0) {
		return false
	}
	if path.IsAbs(name) || strings.HasPrefix(name, "/") {
		return false
	}
	cleaned := path.Clean(name)
	if cleaned == "." || cleaned == ".." || strings.HasPrefix(cleaned, "../") {
		return false
	}
	return cleaned == name
}
