package bundle

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// Paths are the packages written under root/bundle.
type Paths struct {
	Deb      string
	MSI      string
	DMG      string
	AppImage string
}

type fileItem struct {
	Rel  string
	Data []byte
	Mode int64
}

// Write packs the staged directory into a deb, an msi, a dmg, and an AppImage.
// root/bundle is not included in the payload.
func Write(root string, spec Spec) (Paths, error) {
	var err error
	spec, err = spec.prepare()
	if err != nil {
		return Paths{}, err
	}
	files, err := readTree(root)
	if err != nil {
		return Paths{}, err
	}
	if len(files) == 0 {
		return Paths{}, fmt.Errorf("bundle: no files")
	}
	pkg, err := packageName(spec.Product)
	if err != nil {
		return Paths{}, err
	}
	version, err := debianVersion(spec.Version)
	if err != nil {
		return Paths{}, err
	}
	out := filepath.Join(root, "bundle")
	if err := os.MkdirAll(out, 0o755); err != nil {
		return Paths{}, err
	}
	paths := Paths{
		Deb:      filepath.Join(out, pkg+"_"+version+"_"+spec.Arch+".deb"),
		MSI:      filepath.Join(out, pkg+"_"+version+"_"+spec.Arch+".msi"),
		DMG:      filepath.Join(out, pkg+"_"+version+".dmg"),
		AppImage: filepath.Join(out, pkg+"_"+version+"_"+spec.Arch+".AppImage"),
	}
	if err := writeDeb(paths.Deb, spec, files); err != nil {
		return Paths{}, err
	}
	if err := writeMSI(paths.MSI, spec, files); err != nil {
		return Paths{}, err
	}
	if err := writeDMG(paths.DMG, spec, files); err != nil {
		return Paths{}, err
	}
	if err := writeAppImage(paths.AppImage, spec, files); err != nil {
		return Paths{}, err
	}
	return paths, nil
}

func readTree(root string) ([]fileItem, error) {
	var files []fileItem
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		if rel == "." {
			return nil
		}
		if d.IsDir() {
			if rel == "bundle" || strings.HasPrefix(rel, "bundle/") {
				return filepath.SkipDir
			}
			return nil
		}
		if !d.Type().IsRegular() {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		mode := int64(0o644)
		if info, err := d.Info(); err == nil {
			mode = int64(info.Mode().Perm())
		}
		files = append(files, fileItem{Rel: rel, Data: data, Mode: mode})
		return nil
	})
	return files, err
}

func writeFile(path string, data []byte, mode os.FileMode) error {
	return os.WriteFile(path, data, mode)
}
