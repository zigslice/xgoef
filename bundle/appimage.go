package bundle

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"path"
	"runtime"
	"strings"

	_ "embed"
)

//go:embed appruntime.linux-amd64
var appRuntime []byte

func writeAppImage(pathName string, spec Spec, files []fileItem) error {
	body, err := buildAppImage(spec, files)
	if err != nil {
		return err
	}
	return writeFile(pathName, body, 0o755)
}

func buildAppImage(spec Spec, files []fileItem) ([]byte, error) {
	pkg, err := packageName(spec.Product)
	if err != nil {
		return nil, err
	}
	bin := binName(spec.Product)
	paths := map[string][]byte{}
	modes := map[string]uint16{}
	links := map[string]string{}
	hasBin := false
	for _, file := range files {
		rel := path.Clean(file.Rel)
		if rel == "." || strings.HasPrefix(rel, "../") || strings.HasPrefix(rel, "/") {
			return nil, fmt.Errorf("bundle: path %q leaves the package", file.Rel)
		}
		if rel == bin {
			hasBin = true
		}
		dest := path.Join("usr/lib", pkg, rel)
		mode := uint16(file.Mode & 0o777)
		if mode == 0 {
			mode = 0o644
		}
		if rel == bin {
			mode = 0o755
		}
		paths[dest] = file.Data
		modes[dest] = mode
	}
	if hasBin {
		links[path.Join("usr/bin", pkg)] = path.Join("..", "lib", pkg, bin)
	}
	paths["AppRun"] = []byte(appRunScript(pkg, hasBin))
	modes["AppRun"] = 0o755
	paths[pkg+".desktop"] = []byte(appDesktop(spec, pkg, hasBin))
	modes[pkg+".desktop"] = 0o644
	paths[pkg+".png"] = appIcon
	modes[pkg+".png"] = 0o644
	links[".DirIcon"] = pkg + ".png"
	image, err := writeSquashFS(paths, modes, links)
	if err != nil {
		return nil, err
	}
	return prefixRuntime(image)
}

func appRunScript(pkg string, hasBin bool) string {
	if !hasBin {
		return "#!/bin/sh\necho \"xgoef-appimage: host binary is not in this image\" >&2\nexit 1\n"
	}
	return "#!/bin/sh\nset -eu\nhere=$(CDPATH= cd -- \"$(dirname \"$0\")\" && pwd)\nexec \"$here/usr/bin/" + pkg + "\" \"$@\"\n"
}

func appDesktop(spec Spec, pkg string, hasBin bool) string {
	var b strings.Builder
	b.WriteString("[Desktop Entry]\nVersion=1.0\nType=Application\nName=")
	b.WriteString(strings.ReplaceAll(spec.Product, "\n", " "))
	b.WriteByte('\n')
	if hasBin {
		b.WriteString("Exec=")
		b.WriteString(pkg)
		if len(spec.Schemes) > 0 {
			b.WriteString(" %u")
		}
		b.WriteByte('\n')
	}
	b.WriteString("Icon=")
	b.WriteString(pkg)
	b.WriteByte('\n')
	if len(spec.Schemes) > 0 {
		b.WriteString("MimeType=")
		for _, scheme := range spec.Schemes {
			b.WriteString("x-scheme-handler/")
			b.WriteString(scheme)
			b.WriteByte(';')
		}
		b.WriteByte('\n')
	}
	b.WriteString("Terminal=false\nCategories=Utility;\n")
	return b.String()
}

func prefixRuntime(image []byte) ([]byte, error) {
	if runtime.GOOS != "linux" || runtime.GOARCH != "amd64" {
		return image, nil
	}
	runtimeBytes := append([]byte(nil), appRuntime...)
	marker := []byte("XGOEF_SQ_OFFSET!")
	at := bytes.Index(runtimeBytes, marker)
	if at < 0 || at+24 > len(runtimeBytes) {
		return nil, fmt.Errorf("bundle: appimage runtime marker is missing")
	}
	binary.LittleEndian.PutUint64(runtimeBytes[at+16:], uint64(len(runtimeBytes)))
	return append(runtimeBytes, image...), nil
}

// appIcon is a 1x1 PNG referenced by the desktop file and .DirIcon.
var appIcon = []byte{
	0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a, 0x00, 0x00, 0x00, 0x0d, 0x49, 0x48, 0x44, 0x52,
	0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x01, 0x08, 0x02, 0x00, 0x00, 0x00, 0x90, 0x77, 0x53,
	0xde, 0x00, 0x00, 0x00, 0x0c, 0x49, 0x44, 0x41, 0x54, 0x78, 0x9c, 0x63, 0x60, 0x60, 0x60, 0x00,
	0x00, 0x00, 0x04, 0x00, 0x01, 0xf6, 0x17, 0x38, 0x55, 0x00, 0x00, 0x00, 0x00, 0x49, 0x45, 0x4e,
	0x44, 0xae, 0x42, 0x60, 0x82,
}
