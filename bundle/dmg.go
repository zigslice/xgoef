package bundle

import (
	"bytes"
	"encoding/base64"
	"encoding/binary"
	"hash/crc32"
	"path"
	"strings"
)

func writeDMG(pathName string, spec Spec, files []fileItem) error {
	app := segment(spec.Product) + ".app"
	bin := binName(spec.Product)
	tree := map[string][]byte{}
	var binary []byte
	var haveBin bool
	for _, file := range files {
		rel := path.Clean(file.Rel)
		tree[path.Join(app, "Contents", "Resources", rel)] = file.Data
		if rel == bin {
			binary = file.Data
			haveBin = true
		}
	}
	if haveBin {
		tree[path.Join(app, "Contents", "MacOS", bin)] = binary
	}
	tree[path.Join(app, "Contents", "Info.plist")] = []byte(infoPlist(spec, bin))
	img, err := fatImage(tree)
	if err != nil {
		return err
	}
	for len(img)%512 != 0 {
		img = append(img, 0)
	}
	return writeFile(pathName, wrapUDIF(img), 0o644)
}

func infoPlist(spec Spec, bin string) string {
	version, _ := msiVersion(spec.Version)
	id := spec.Identifier
	if id == "" {
		id = "xgoef." + segment(spec.Product)
	}
	var b strings.Builder
	b.WriteString(`<?xml version="1.0" encoding="UTF-8"?>` + "\n")
	b.WriteString(`<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">` + "\n")
	b.WriteString("<plist version=\"1.0\"><dict>\n")
	plistKey(&b, "CFBundleName", spec.Product)
	plistKey(&b, "CFBundleIdentifier", id)
	plistKey(&b, "CFBundleVersion", version)
	plistKey(&b, "CFBundleShortVersionString", version)
	plistKey(&b, "CFBundleExecutable", bin)
	plistKey(&b, "CFBundlePackageType", "APPL")
	if len(spec.Schemes) > 0 {
		b.WriteString("<key>CFBundleURLTypes</key><array><dict><key>CFBundleURLSchemes</key><array>")
		for _, scheme := range spec.Schemes {
			b.WriteString("<string>")
			b.WriteString(xmlEscape(scheme))
			b.WriteString("</string>")
		}
		b.WriteString("</array></dict></array>\n")
	}
	b.WriteString("</dict></plist>\n")
	return b.String()
}

func plistKey(b *strings.Builder, key, value string) {
	b.WriteString("<key>")
	b.WriteString(xmlEscape(key))
	b.WriteString("</key><string>")
	b.WriteString(xmlEscape(value))
	b.WriteString("</string>\n")
}

func xmlEscape(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	return s
}

func wrapUDIF(img []byte) []byte {
	sectors := len(img) / 512
	mish := mishBlock(sectors, len(img))
	xml := udifXML(mish)
	koly := kolyHeader(uint64(len(img)), uint64(len(xml)), uint64(sectors), crc32.ChecksumIEEE(img))
	var out bytes.Buffer
	out.Write(img)
	out.Write(xml)
	out.Write(koly)
	return out.Bytes()
}

func mishBlock(sectors, length int) []byte {
	// 204-byte BLKX header, then a raw run and a terminator.
	buf := make([]byte, 204+80)
	binary.BigEndian.PutUint32(buf[0:], 0x6d697368)
	binary.BigEndian.PutUint32(buf[4:], 1)
	binary.BigEndian.PutUint64(buf[16:], uint64(sectors))
	binary.BigEndian.PutUint32(buf[200:], 2)
	putRun(buf[204:], 0x00000001, 0, uint64(sectors), 0, uint64(length))
	putRun(buf[244:], 0xFFFFFFFF, uint64(sectors), 0, 0, 0)
	return buf
}

func putRun(buf []byte, kind uint32, sector, count, compOff, compLen uint64) {
	binary.BigEndian.PutUint32(buf[0:], kind)
	binary.BigEndian.PutUint64(buf[8:], sector)
	binary.BigEndian.PutUint64(buf[16:], count)
	binary.BigEndian.PutUint64(buf[24:], compOff)
	binary.BigEndian.PutUint64(buf[32:], compLen)
}

func udifXML(mish []byte) []byte {
	body := `<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0"><dict><key>resource-fork</key><dict><key>blkx</key><array><dict>
<key>Attributes</key><string>0x0050</string>
<key>CFName</key><string>whole disk</string>
<key>Data</key><data>` + base64.StdEncoding.EncodeToString(mish) + `</data>
<key>ID</key><string>0</string>
<key>Name</key><string>whole disk</string>
</dict></array></dict></dict></plist>
`
	return []byte(body)
}

func kolyHeader(dataLen, xmlLen, sectors uint64, sum uint32) []byte {
	buf := make([]byte, 512)
	binary.BigEndian.PutUint32(buf[0:], 0x6b6f6c79)
	binary.BigEndian.PutUint32(buf[4:], 4)
	binary.BigEndian.PutUint32(buf[8:], 512)
	binary.BigEndian.PutUint64(buf[24:], 0)
	binary.BigEndian.PutUint64(buf[32:], dataLen)
	binary.BigEndian.PutUint32(buf[60:], 1)
	binary.BigEndian.PutUint32(buf[80:], 2)
	binary.BigEndian.PutUint32(buf[84:], 32)
	binary.BigEndian.PutUint32(buf[88:], sum)
	binary.BigEndian.PutUint64(buf[216:], dataLen)
	binary.BigEndian.PutUint64(buf[224:], xmlLen)
	binary.BigEndian.PutUint32(buf[488:], 2)
	binary.BigEndian.PutUint64(buf[492:], sectors)
	return buf
}
