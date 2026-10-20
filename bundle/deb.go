package bundle

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"fmt"
	"io"
	"path"
	"strings"
	"time"
)

func writeDeb(pathName string, spec Spec, files []fileItem) error {
	pkg, err := packageName(spec.Product)
	if err != nil {
		return err
	}
	version, err := debianVersion(spec.Version)
	if err != nil {
		return err
	}
	bin := binName(spec.Product)
	prefix := path.Join("usr/lib", pkg)
	var entries []tarEntry
	dirs := map[string]bool{}
	addDir := func(name string) {
		name = path.Clean(name)
		if name == "." || dirs[name] {
			return
		}
		var chain []string
		for name != "." && name != "/" && !dirs[name] {
			chain = append(chain, name)
			dirs[name] = true
			name = path.Dir(name)
		}
		for i := len(chain) - 1; i >= 0; i-- {
			entries = append(entries, tarEntry{name: chain[i] + "/", dir: true, mode: 0o755})
		}
	}
	var installed int64
	hasBin := false
	for _, file := range files {
		rel := path.Clean(file.Rel)
		if rel == "." || strings.HasPrefix(rel, "../") {
			return fmt.Errorf("bundle: path %q leaves the package", file.Rel)
		}
		dest := path.Join(prefix, rel)
		addDir(path.Dir(dest))
		mode := file.Mode
		if mode == 0 {
			mode = 0o644
		}
		if rel == bin {
			mode = 0o755
			hasBin = true
		}
		entries = append(entries, tarEntry{name: dest, mode: mode, data: file.Data})
		installed += int64(len(file.Data))
	}
	if hasBin {
		linkDir := "usr/bin"
		addDir(linkDir)
		entries = append(entries, tarEntry{
			name: path.Join(linkDir, pkg),
			mode: 0o777,
			link: path.Join("..", "lib", pkg, bin),
		})
	}
	desktop := desktopFile(spec, "/usr/lib/"+pkg+"/"+bin, hasBin)
	addDir("usr/share/applications")
	entries = append(entries, tarEntry{
		name: path.Join("usr/share/applications", pkg+".desktop"),
		mode: 0o644,
		data: []byte(desktop),
	})
	dataTar, err := packTar(entries)
	if err != nil {
		return err
	}
	control := controlFile(pkg, version, spec, installed)
	controlTar, err := packTar([]tarEntry{{name: "control", mode: 0o644, data: []byte(control)}})
	if err != nil {
		return err
	}
	var body bytes.Buffer
	if err := writeAr(&body, []arMember{
		{name: "debian-binary", data: []byte("2.0\n")},
		{name: "control.tar.gz", data: gzipBytes(controlTar)},
		{name: "data.tar.gz", data: gzipBytes(dataTar)},
	}); err != nil {
		return err
	}
	return writeFile(pathName, body.Bytes(), 0o644)
}

func controlFile(pkg, version string, spec Spec, installed int64) string {
	kb := installed / 1024
	if installed%1024 != 0 {
		kb++
	}
	who := spec.Product
	mail := "xgoef@localhost"
	if spec.Identifier != "" {
		mail = spec.Identifier + "@localhost"
	}
	return fmt.Sprintf("Package: %s\nVersion: %s\nArchitecture: %s\nMaintainer: %s <%s>\nInstalled-Size: %d\nSection: utils\nPriority: optional\nDescription: %s\n",
		pkg, version, spec.Arch, who, mail, kb, spec.Product)
}

func desktopFile(spec Spec, exec string, hasBin bool) string {
	var b strings.Builder
	b.WriteString("[Desktop Entry]\nVersion=1.0\nType=Application\nName=")
	b.WriteString(spec.Product)
	b.WriteByte('\n')
	if hasBin {
		b.WriteString("Exec=")
		b.WriteString(quoteDesktop(exec))
		if len(spec.Schemes) > 0 {
			b.WriteString(" %u")
		}
		b.WriteByte('\n')
	}
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

func quoteDesktop(pathName string) string {
	var b strings.Builder
	b.WriteByte('"')
	for _, r := range pathName {
		if r == '\\' || r == '"' || r == '$' || r == '`' {
			b.WriteByte('\\')
		}
		b.WriteRune(r)
	}
	b.WriteByte('"')
	return b.String()
}

type tarEntry struct {
	name string
	mode int64
	data []byte
	link string
	dir  bool
}

func packTar(entries []tarEntry) ([]byte, error) {
	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)
	seen := map[string]bool{}
	for _, entry := range entries {
		if seen[entry.name] {
			continue
		}
		seen[entry.name] = true
		hdr := &tar.Header{
			Name:    entry.name,
			Mode:    entry.mode,
			Uid:     0,
			Gid:     0,
			Uname:   "root",
			Gname:   "root",
			ModTime: time.Unix(0, 0),
		}
		switch {
		case entry.dir:
			hdr.Typeflag = tar.TypeDir
			hdr.Mode = 0o755
		case entry.link != "":
			hdr.Typeflag = tar.TypeSymlink
			hdr.Linkname = entry.link
			hdr.Mode = 0o777
		default:
			hdr.Typeflag = tar.TypeReg
			hdr.Size = int64(len(entry.data))
		}
		if err := tw.WriteHeader(hdr); err != nil {
			return nil, err
		}
		if hdr.Typeflag == tar.TypeReg {
			if _, err := tw.Write(entry.data); err != nil {
				return nil, err
			}
		}
	}
	if err := tw.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func gzipBytes(data []byte) []byte {
	var buf bytes.Buffer
	zw := gzip.NewWriter(&buf)
	_, _ = zw.Write(data)
	_ = zw.Close()
	return buf.Bytes()
}

type arMember struct {
	name string
	data []byte
}

func writeAr(w io.Writer, members []arMember) error {
	if _, err := io.WriteString(w, "!<arch>\n"); err != nil {
		return err
	}
	for _, member := range members {
		if err := writeArMember(w, member.name, member.data); err != nil {
			return err
		}
	}
	return nil
}

func writeArMember(w io.Writer, name string, data []byte) error {
	var hdr [60]byte
	for i := range hdr {
		hdr[i] = ' '
	}
	field := name
	if !strings.HasSuffix(field, "/") {
		field += "/"
	}
	if len(field) > 16 {
		return fmt.Errorf("bundle: ar name %q is too long", name)
	}
	copy(hdr[:], field)
	copy(hdr[16:], "0")
	copy(hdr[28:], "0")
	copy(hdr[34:], "0")
	copy(hdr[40:], "100644")
	size := fmt.Sprintf("%d", len(data))
	if len(size) > 10 {
		return fmt.Errorf("bundle: ar member is too large")
	}
	copy(hdr[48:], size)
	hdr[58] = '`'
	hdr[59] = '\n'
	if _, err := w.Write(hdr[:]); err != nil {
		return err
	}
	if _, err := w.Write(data); err != nil {
		return err
	}
	if len(data)%2 == 1 {
		if _, err := w.Write([]byte{'\n'}); err != nil {
			return err
		}
	}
	return nil
}
