package bundle

import (
	"encoding/binary"
	"fmt"
	"path"
	"sort"
	"strings"
	"unicode/utf16"
)

type fatFile struct {
	name   string
	data   []byte
	isDir  bool
	parent *fatFile
	kids   []*fatFile
}

func fatImage(files map[string][]byte) ([]byte, error) {
	root := &fatFile{name: "", isDir: true}
	var paths []string
	for name := range files {
		paths = append(paths, name)
	}
	sort.Strings(paths)
	for _, name := range paths {
		if err := fatAdd(root, name, files[name]); err != nil {
			return nil, err
		}
	}
	const bps = 512
	spc := 1
	for {
		clusters, err := fatCount(root, spc*bps)
		if err != nil {
			return nil, err
		}
		if clusters <= 65524 {
			break
		}
		if spc >= 64 {
			return nil, fmt.Errorf("bundle: dmg image is too large")
		}
		spc *= 2
	}
	return renderFAT(root, spc)
}

func fatAdd(root *fatFile, name string, data []byte) error {
	name = path.Clean(strings.TrimPrefix(name, "/"))
	if name == "." || strings.HasPrefix(name, "../") {
		return fmt.Errorf("bundle: path %q leaves the image", name)
	}
	parts := strings.Split(name, "/")
	cur := root
	for i, part := range parts {
		if part == "" || part == "." || part == ".." {
			return fmt.Errorf("bundle: path %q leaves the image", name)
		}
		last := i == len(parts)-1
		var child *fatFile
		for _, kid := range cur.kids {
			if kid.name == part {
				child = kid
				break
			}
		}
		if child == nil {
			child = &fatFile{name: part, isDir: !last, parent: cur}
			cur.kids = append(cur.kids, child)
		}
		if last {
			if child.isDir {
				return fmt.Errorf("bundle: %q is a directory", name)
			}
			child.data = data
			return nil
		}
		if !child.isDir {
			return fmt.Errorf("bundle: %q is a file", part)
		}
		cur = child
	}
	return nil
}

func fatCount(dir *fatFile, clusterBytes int) (int, error) {
	total := 0
	var walk func(*fatFile) error
	walk = func(node *fatFile) error {
		if node.isDir {
			n := len(node.kids)
			if node != dir && node.name != "" {
				n += 2
			}
			entries := 0
			if node.name == "" && node == dir {
				entries = fatEntryCount(node, false)
			} else {
				entries = fatEntryCount(node, true)
			}
			total += clustersFor(entries*32, clusterBytes)
			for _, kid := range node.kids {
				if err := walk(kid); err != nil {
					return err
				}
			}
			return nil
		}
		total += clustersFor(len(node.data), clusterBytes)
		return nil
	}
	if err := walk(dir); err != nil {
		return 0, err
	}
	return total, nil
}

func fatEntryCount(dir *fatFile, dot bool) int {
	n := 0
	if dot {
		n += 2
	}
	for _, kid := range dir.kids {
		n += 1 + lfnCount(kid.name)
	}
	return n
}

func clustersFor(size, clusterBytes int) int {
	if size <= 0 {
		return 1
	}
	return (size + clusterBytes - 1) / clusterBytes
}

func lfnCount(name string) int {
	n := len(utf16.Encode([]rune(name)))
	if n == 0 {
		return 0
	}
	return (n + 12) / 13
}

type placed struct {
	node    *fatFile
	start   int
	count   int
	entries []byte
	payload []byte
}

func renderFAT(root *fatFile, spc int) ([]byte, error) {
	const bps = 512
	clusterBytes := spc * bps
	var order []*placed
	var place func(*fatFile) error
	place = func(node *fatFile) error {
		item := &placed{node: node}
		if node.isDir {
			item.count = clustersFor(fatEntryCount(node, node != root)*32, clusterBytes)
		} else if len(node.data) == 0 {
			item.count = 0
		} else {
			item.count = clustersFor(len(node.data), clusterBytes)
			item.payload = node.data
		}
		order = append(order, item)
		if node.isDir {
			kids := append([]*fatFile(nil), node.kids...)
			sort.Slice(kids, func(i, j int) bool { return kids[i].name < kids[j].name })
			node.kids = kids
			for _, kid := range kids {
				if err := place(kid); err != nil {
					return err
				}
			}
		}
		return nil
	}
	if err := place(root); err != nil {
		return nil, err
	}
	next := 2
	for _, item := range order {
		if item.node == root {
			item.start = 0
			item.count = 0
			continue
		}
		if item.count == 0 {
			item.start = 0
			continue
		}
		item.start = next
		next += item.count
	}
	clusterCount := next
	byNode := map[*fatFile]*placed{}
	for _, item := range order {
		byNode[item.node] = item
	}
	for _, item := range order {
		if !item.node.isDir {
			continue
		}
		raw := dirBytes(item.node, item.node == root, byNode)
		if item.node == root {
			item.entries = raw
			continue
		}
		item.payload = raw
	}
	rootEnts := 512
	if len(byNode[root].entries) > rootEnts*32 {
		return nil, fmt.Errorf("bundle: too many files in the dmg root")
	}
	rootSectors := rootEnts * 32 / bps
	fatEntries := clusterCount
	fatSectors := (fatEntries*2 + bps - 1) / bps
	dataClusters := clusterCount - 2
	if dataClusters < 0 {
		dataClusters = 0
	}
	dataSectors := dataClusters * spc
	reserved := 1
	total := reserved + fatSectors*2 + rootSectors + dataSectors
	img := make([]byte, total*bps)
	writeBoot(img, spc, rootEnts, fatSectors, total)
	fatStart := reserved * bps
	fat := make([]byte, fatSectors*bps)
	binary.LittleEndian.PutUint16(fat[0:], 0xFFF8)
	binary.LittleEndian.PutUint16(fat[2:], 0xFFFF)
	for _, item := range order {
		if item.start < 2 || item.count == 0 {
			continue
		}
		for i := 0; i < item.count; i++ {
			cluster := item.start + i
			var nextc uint16 = 0xFFFF
			if i+1 < item.count {
				nextc = uint16(cluster + 1)
			}
			binary.LittleEndian.PutUint16(fat[cluster*2:], nextc)
		}
	}
	copy(img[fatStart:], fat)
	copy(img[fatStart+fatSectors*bps:], fat)
	rootOff := (reserved + fatSectors*2) * bps
	copy(img[rootOff:], byNode[root].entries)
	dataOff := rootOff + rootSectors*bps
	for _, item := range order {
		if item.node == root || item.start < 2 {
			continue
		}
		off := dataOff + (item.start-2)*clusterBytes
		copy(img[off:], item.payload)
	}
	if img[510] != 0x55 || img[511] != 0xAA {
		return nil, fmt.Errorf("bundle: fat boot sector was not written")
	}
	return img, nil
}

func dirBytes(dir *fatFile, isRoot bool, byNode map[*fatFile]*placed) []byte {
	var entries []byte
	if !isRoot {
		self := byNode[dir].start
		parent := 0
		if dir.parent != nil && dir.parent.parent != nil {
			if p := byNode[dir.parent]; p != nil {
				parent = p.start
			}
		}
		entries = append(entries, shortOnly(".          ", 0x10, self)...)
		entries = append(entries, shortOnly("..         ", 0x10, parent)...)
	}
	usedShort := map[string]int{}
	for _, kid := range dir.kids {
		placedKid := byNode[kid]
		start := 0
		if placedKid != nil {
			start = placedKid.start
		}
		attr := byte(0x20)
		size := len(kid.data)
		if kid.isDir {
			attr = 0x10
			size = 0
		}
		short := uniqueShort(kid.name, usedShort)
		entries = append(entries, lfnEntries(kid.name, short)...)
		raw := shortOnly(string(short[:]), attr, start)
		if !kid.isDir {
			binary.LittleEndian.PutUint32(raw[28:], uint32(size))
		}
		entries = append(entries, raw...)
	}
	return entries
}

func shortOnly(name11 string, attr byte, cluster int) []byte {
	raw := make([]byte, 32)
	copy(raw[:11], name11)
	raw[11] = attr
	binary.LittleEndian.PutUint16(raw[26:], uint16(cluster))
	return raw
}

func uniqueShort(name string, used map[string]int) [11]byte {
	base := short11(name)
	key := string(base[:])
	n := used[key]
	used[key] = n + 1
	if n == 0 {
		return base
	}
	suffix := fmt.Sprintf("~%d", n)
	for i := 0; i < len(suffix); i++ {
		base[7-i] = suffix[len(suffix)-1-i]
	}
	return base
}

func short11(name string) [11]byte {
	base := name
	ext := ""
	if dot := strings.LastIndex(name, "."); dot > 0 {
		base = name[:dot]
		ext = name[dot+1:]
	}
	var out [11]byte
	for i := range out {
		out[i] = ' '
	}
	put := func(dst []byte, src string) {
		i := 0
		for _, r := range strings.ToUpper(src) {
			if i >= len(dst) {
				break
			}
			if (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') {
				dst[i] = byte(r)
				i++
			}
		}
	}
	put(out[:8], base)
	put(out[8:], ext)
	if out[0] == ' ' {
		out[0] = '_'
	}
	return out
}

func lfnEntries(name string, short [11]byte) []byte {
	units := utf16.Encode([]rune(name))
	if len(units) == 0 {
		return nil
	}
	n := (len(units) + 12) / 13
	sum := lfnSum(short)
	var out []byte
	for seq := n; seq >= 1; seq-- {
		raw := make([]byte, 32)
		raw[0] = byte(seq)
		if seq == n {
			raw[0] |= 0x40
		}
		raw[11] = 0x0F
		raw[13] = sum
		chunk := make([]uint16, 13)
		for i := range chunk {
			chunk[i] = 0xFFFF
		}
		start := (seq - 1) * 13
		for i := 0; i < 13 && start+i < len(units); i++ {
			chunk[i] = units[start+i]
		}
		if start+13 > len(units) && start < len(units) {
			// terminator sits in the first unused unit when the name ended inside this entry
			if len(units)%13 != 0 && seq == n {
				// already 0xFFFF padding; the first padding unit should be 0
				used := len(units) % 13
				if used != 0 {
					chunk[used] = 0
				}
			}
		}
		writeLFNChars(raw, chunk)
		out = append(out, raw...)
	}
	return out
}

func writeLFNChars(raw []byte, chars []uint16) {
	off := []int{1, 3, 5, 7, 9, 14, 16, 18, 20, 22, 24, 28, 30}
	for i, at := range off {
		binary.LittleEndian.PutUint16(raw[at:], chars[i])
	}
}

func lfnSum(short [11]byte) byte {
	var sum byte
	for _, b := range short {
		sum = ((sum & 1) << 7) + (sum >> 1) + b
	}
	return sum
}

// readImageFile returns one file from a FAT image or from a UDIF wrapper around one.
func readImageFile(img []byte, want string) ([]byte, error) {
	if len(img) >= 512 && string(img[len(img)-512:len(img)-508]) == "koly" {
		dataLen := binary.BigEndian.Uint64(img[len(img)-512+32:])
		if int(dataLen) > len(img) {
			return nil, fmt.Errorf("bundle: dmg data fork is too long")
		}
		img = img[:dataLen]
	}
	if len(img) < 512 || img[510] != 0x55 || img[511] != 0xAA {
		return nil, fmt.Errorf("bundle: not a fat image")
	}
	bps := int(binary.LittleEndian.Uint16(img[11:]))
	spc := int(img[13])
	reserved := int(binary.LittleEndian.Uint16(img[14:]))
	fats := int(img[16])
	rootEnts := int(binary.LittleEndian.Uint16(img[17:]))
	fatSectors := int(binary.LittleEndian.Uint16(img[22:]))
	rootOff := (reserved + fats*fatSectors) * bps
	dataOff := rootOff + rootEnts*32
	fat := img[reserved*bps:]
	root := parseDir(img[rootOff:rootOff+rootEnts*32], fat, img, dataOff, spc*bps)
	parts := strings.Split(want, "/")
	cur := root
	for i, part := range parts {
		kid, ok := cur.kids[part]
		if !ok {
			return nil, fmt.Errorf("bundle: %s not in image", want)
		}
		if i == len(parts)-1 {
			if kid.dir {
				return nil, fmt.Errorf("bundle: %s is a directory", want)
			}
			return kid.data, nil
		}
		cur = kid
	}
	return nil, fmt.Errorf("bundle: %s not in image", want)
}

type fatNode struct {
	dir  bool
	data []byte
	kids map[string]*fatNode
}

func parseDir(raw, fat, img []byte, dataOff, clusterBytes int) *fatNode {
	node := &fatNode{dir: true, kids: map[string]*fatNode{}}
	var lfn []uint16
	for off := 0; off+32 <= len(raw); off += 32 {
		if raw[off] == 0 {
			break
		}
		if raw[off] == 0xE5 {
			lfn = nil
			continue
		}
		if raw[off+11] == 0x0F {
			seq := int(raw[off] & 0x1F)
			chars := readLFNChars(raw[off:])
			for len(lfn) < seq*13 {
				lfn = append(lfn, 0)
			}
			copy(lfn[(seq-1)*13:], chars)
			continue
		}
		name := lfnString(lfn)
		lfn = nil
		if name == "" {
			name = shortString(raw[off : off+11])
		}
		if name == "." || name == ".." {
			continue
		}
		cluster := int(binary.LittleEndian.Uint16(raw[off+26:]))
		if raw[off+11]&0x10 != 0 {
			body := readChain(img, fat, dataOff, clusterBytes, cluster)
			node.kids[name] = parseDir(body, fat, img, dataOff, clusterBytes)
			continue
		}
		size := int(binary.LittleEndian.Uint32(raw[off+28:]))
		body := readChain(img, fat, dataOff, clusterBytes, cluster)
		if size < len(body) {
			body = body[:size]
		}
		node.kids[name] = &fatNode{data: append([]byte(nil), body...)}
	}
	return node
}

func lfnString(chars []uint16) string {
	end := 0
	for end < len(chars) && chars[end] != 0 && chars[end] != 0xFFFF {
		end++
	}
	return string(utf16.Decode(chars[:end]))
}

func readLFNChars(raw []byte) []uint16 {
	off := []int{1, 3, 5, 7, 9, 14, 16, 18, 20, 22, 24, 28, 30}
	out := make([]uint16, 13)
	for i, at := range off {
		out[i] = binary.LittleEndian.Uint16(raw[at:])
	}
	return out
}

func shortString(name []byte) string {
	base := strings.TrimRight(string(name[:8]), " ")
	ext := strings.TrimRight(string(name[8:]), " ")
	if ext == "" {
		return base
	}
	return base + "." + ext
}

func readChain(img, fat []byte, dataOff, clusterBytes, start int) []byte {
	if start < 2 {
		return nil
	}
	var out []byte
	seen := 0
	for start >= 2 && start < 0xFFF8 && seen < 100000 {
		off := dataOff + (start-2)*clusterBytes
		if off < 0 || off+clusterBytes > len(img) {
			break
		}
		out = append(out, img[off:off+clusterBytes]...)
		start = int(binary.LittleEndian.Uint16(fat[start*2:]))
		seen++
	}
	return out
}

func writeBoot(img []byte, spc, rootEnts, fatSectors, total int) {
	img[0] = 0xEB
	img[1] = 0x3C
	img[2] = 0x90
	copy(img[3:], "MSDOS5.0")
	binary.LittleEndian.PutUint16(img[11:], 512)
	img[13] = byte(spc)
	binary.LittleEndian.PutUint16(img[14:], 1)
	img[16] = 2
	binary.LittleEndian.PutUint16(img[17:], uint16(rootEnts))
	if total < 0x10000 {
		binary.LittleEndian.PutUint16(img[19:], uint16(total))
	}
	img[21] = 0xF8
	binary.LittleEndian.PutUint16(img[22:], uint16(fatSectors))
	binary.LittleEndian.PutUint16(img[24:], 32)
	binary.LittleEndian.PutUint16(img[26:], 64)
	binary.LittleEndian.PutUint32(img[32:], uint32(total))
	img[36] = 0x80
	img[38] = 0x29
	binary.LittleEndian.PutUint32(img[39:], 0x78676F65)
	copy(img[43:], "XGOEF      ")
	copy(img[54:], "FAT16   ")
	img[510] = 0x55
	img[511] = 0xAA
}
