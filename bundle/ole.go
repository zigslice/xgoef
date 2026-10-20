package bundle

import (
	"encoding/binary"
	"fmt"
)

const (
	oleFree  = 0xFFFFFFFF
	oleEnd   = 0xFFFFFFFE
	oleFAT   = 0xFFFFFFFD
	oleDIFAT = 0xFFFFFFFC
	oleCut   = 4096
	oleSec   = 512
	oleMini  = 64
)

type oleStream struct {
	name []uint16
	data []byte
}

func writeOLE(streams []oleStream) ([]byte, error) {
	for _, stream := range streams {
		if len(stream.name) == 0 || len(stream.name) > 31 {
			return nil, fmt.Errorf("bundle: OLE stream name length %d", len(stream.name))
		}
	}
	miniBlob, miniFAT, miniIndex := packMini(streams)
	miniFATBytes := uint32s(miniFAT)
	dirCount := 1 + len(streams)
	dirBytes := make([]byte, ((dirCount*128+oleSec-1)/oleSec)*oleSec)
	entries := make([]dirEnt, dirCount)
	entries[0] = dirEnt{name: utf16Name("Root Entry"), typ: 5, color: 1, left: oleFree, right: oleFree, child: oleFree, start: oleEnd}
	copy(entries[0].clsid[:], msiCLSID[:])
	for i, stream := range streams {
		entries[i+1] = dirEnt{
			name:  append([]uint16(nil), stream.name...),
			typ:   2,
			color: 1,
			left:  oleFree,
			right: oleFree,
			child: oleFree,
			start: oleEnd,
			size:  uint64(len(stream.data)),
		}
	}
	rootChild := buildTree(entries[1:])
	entries[0].child = rootChild
	for i := range entries {
		putDir(dirBytes[i*128:], entries[i])
	}
	type part struct {
		data  []byte
		start uint32
		count int
	}
	var parts []part
	add := func(data []byte) int {
		n := 0
		if len(data) > 0 {
			n = (len(data) + oleSec - 1) / oleSec
		}
		parts = append(parts, part{data: data, count: n})
		return len(parts) - 1
	}
	miniFATPart := -1
	miniPart := -1
	if len(miniBlob) > 0 {
		miniFATPart = add(miniFATBytes)
		miniPart = add(miniBlob)
	}
	dirPart := add(dirBytes)
	bigPart := map[int]int{}
	for i, stream := range streams {
		if len(stream.data) >= oleCut {
			bigPart[i] = add(stream.data)
		}
	}
	nData := 0
	for _, part := range parts {
		nData += part.count
	}
	nFat, nDifat := 1, 0
	for {
		total := nData + nFat + nDifat
		needFat := (total + 127) / 128
		needDifat := 0
		if needFat > 109 {
			needDifat = (needFat - 109 + 126) / 127
		}
		if needFat == nFat && needDifat == nDifat {
			break
		}
		nFat, nDifat = needFat, needDifat
		if nFat > 1000000 {
			return nil, fmt.Errorf("bundle: OLE image is too large")
		}
	}
	dataBase := nFat + nDifat
	at := dataBase
	for i := range parts {
		if parts[i].count == 0 {
			parts[i].start = oleEnd
			continue
		}
		parts[i].start = uint32(at)
		at += parts[i].count
	}
	total := nData + nFat + nDifat
	fat := make([]uint32, total)
	for i := range fat {
		fat[i] = oleFree
	}
	for i := 0; i < nFat; i++ {
		fat[i] = oleFAT
	}
	for i := 0; i < nDifat; i++ {
		fat[nFat+i] = oleDIFAT
	}
	chain := func(start uint32, count int) {
		if count == 0 {
			return
		}
		for i := 0; i < count-1; i++ {
			fat[int(start)+i] = start + uint32(i) + 1
		}
		fat[int(start)+count-1] = oleEnd
	}
	for _, part := range parts {
		chain(part.start, part.count)
	}
	if miniPart >= 0 {
		entries[0].start = parts[miniPart].start
		entries[0].size = uint64(len(miniBlob))
		putDir(dirBytes[0:], entries[0])
	}
	for i, stream := range streams {
		ent := &entries[i+1]
		if len(stream.data) < oleCut {
			if len(stream.data) == 0 {
				ent.start = oleEnd
			} else {
				ent.start = miniIndex[i]
			}
		} else {
			ent.start = parts[bigPart[i]].start
		}
		putDir(dirBytes[(i+1)*128:], *ent)
	}
	file := make([]byte, (1+total)*oleSec)
	var miniStart uint32 = oleEnd
	miniCount := 0
	if miniFATPart >= 0 {
		miniStart = parts[miniFATPart].start
		miniCount = parts[miniFATPart].count
	}
	copy(file[0:], oleHeader(nFat, nDifat, parts[dirPart].start, miniStart, miniCount))
	for i := 0; i < nFat; i++ {
		off := (1 + i) * oleSec
		for j := 0; j < 128; j++ {
			idx := i*128 + j
			val := uint32(oleFree)
			if idx < len(fat) {
				val = fat[idx]
			}
			binary.LittleEndian.PutUint32(file[off+j*4:], val)
		}
	}
	writeDIFAT(file, nFat, nDifat)
	for i, part := range parts {
		if part.count == 0 {
			continue
		}
		off := (1 + int(part.start)) * oleSec
		copy(file[off:], part.data)
		_ = i
	}
	return file, nil
}

func packMini(all []oleStream) (blob []byte, fat []uint32, index map[int]uint32) {
	index = map[int]uint32{}
	at := 0
	for i, stream := range all {
		if len(stream.data) == 0 || len(stream.data) >= oleCut {
			continue
		}
		n := (len(stream.data) + oleMini - 1) / oleMini
		index[i] = uint32(at)
		for j := 0; j < n; j++ {
			if j+1 < n {
				fat = append(fat, uint32(at+1))
			} else {
				fat = append(fat, oleEnd)
			}
			at++
		}
		pad := make([]byte, n*oleMini)
		copy(pad, stream.data)
		blob = append(blob, pad...)
	}
	if len(fat) == 0 {
		return nil, nil, index
	}
	for len(fat)%128 != 0 {
		fat = append(fat, oleFree)
	}
	return blob, fat, index
}

func uint32s(vals []uint32) []byte {
	buf := make([]byte, len(vals)*4)
	for i, val := range vals {
		binary.LittleEndian.PutUint32(buf[i*4:], val)
	}
	return buf
}

type dirEnt struct {
	name  []uint16
	typ   byte
	color byte
	left  uint32
	right uint32
	child uint32
	start uint32
	size  uint64
	clsid [16]byte
}

func putDir(dst []byte, ent dirEnt) {
	for i := range dst {
		dst[i] = 0
	}
	for i, ch := range ent.name {
		if i >= 31 {
			break
		}
		binary.LittleEndian.PutUint16(dst[i*2:], ch)
	}
	binary.LittleEndian.PutUint16(dst[64:], uint16((len(ent.name)+1)*2))
	dst[66] = ent.typ
	dst[67] = ent.color
	binary.LittleEndian.PutUint32(dst[68:], ent.left)
	binary.LittleEndian.PutUint32(dst[72:], ent.right)
	binary.LittleEndian.PutUint32(dst[76:], ent.child)
	copy(dst[80:], ent.clsid[:])
	binary.LittleEndian.PutUint32(dst[116:], ent.start)
	binary.LittleEndian.PutUint64(dst[120:], ent.size)
}

func utf16Name(s string) []uint16 {
	out := make([]uint16, 0, len(s))
	for _, r := range s {
		out = append(out, uint16(r))
	}
	return out
}

func buildTree(entries []dirEnt) uint32 {
	if len(entries) == 0 {
		return oleFree
	}
	idx := make([]int, len(entries))
	for i := range idx {
		idx[i] = i
	}
	var sort func(ids []int)
	less := func(a, b int) bool {
		return cmpU16(entries[a].name, entries[b].name) < 0
	}
	sort = func(ids []int) {
		for i := 1; i < len(ids); i++ {
			j := i
			for j > 0 && less(ids[j], ids[j-1]) {
				ids[j], ids[j-1] = ids[j-1], ids[j]
				j--
			}
		}
	}
	sort(idx)
	var link func([]int) uint32
	link = func(ids []int) uint32 {
		if len(ids) == 0 {
			return oleFree
		}
		mid := len(ids) / 2
		id := ids[mid]
		entries[id].left = link(ids[:mid])
		entries[id].right = link(ids[mid+1:])
		entries[id].color = 1
		return uint32(id + 1)
	}
	return link(idx)
}

func cmpU16(a, b []uint16) int {
	n := len(a)
	if len(b) < n {
		n = len(b)
	}
	for i := 0; i < n; i++ {
		if a[i] < b[i] {
			return -1
		}
		if a[i] > b[i] {
			return 1
		}
	}
	switch {
	case len(a) < len(b):
		return -1
	case len(a) > len(b):
		return 1
	default:
		return 0
	}
}

var msiCLSID = [16]byte{0x84, 0x10, 0x0c, 0x00, 0x00, 0x00, 0x00, 0x00, 0xc0, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x46}

func oleHeader(nFat, nDifat int, dirStart, miniStart uint32, miniCount int) []byte {
	hdr := make([]byte, oleSec)
	copy(hdr, []byte{0xD0, 0xCF, 0x11, 0xE0, 0xA1, 0xB1, 0x1A, 0xE1})
	binary.LittleEndian.PutUint16(hdr[0x18:], 0x003E)
	binary.LittleEndian.PutUint16(hdr[0x1A:], 3)
	binary.LittleEndian.PutUint16(hdr[0x1C:], 0xFFFE)
	binary.LittleEndian.PutUint16(hdr[0x1E:], 9)
	binary.LittleEndian.PutUint16(hdr[0x20:], 6)
	binary.LittleEndian.PutUint32(hdr[0x2C:], uint32(nFat))
	binary.LittleEndian.PutUint32(hdr[0x30:], dirStart)
	binary.LittleEndian.PutUint32(hdr[0x38:], oleCut)
	if miniCount > 0 {
		binary.LittleEndian.PutUint32(hdr[0x3C:], miniStart)
		binary.LittleEndian.PutUint32(hdr[0x40:], uint32(miniCount))
	} else {
		binary.LittleEndian.PutUint32(hdr[0x3C:], oleEnd)
	}
	if nDifat == 0 {
		binary.LittleEndian.PutUint32(hdr[0x44:], oleEnd)
	} else {
		binary.LittleEndian.PutUint32(hdr[0x44:], uint32(nFat))
		binary.LittleEndian.PutUint32(hdr[0x48:], uint32(nDifat))
	}
	for i := 0; i < 109; i++ {
		val := uint32(oleFree)
		if i < nFat {
			val = uint32(i)
		}
		binary.LittleEndian.PutUint32(hdr[0x4C+i*4:], val)
	}
	return hdr
}

func writeDIFAT(file []byte, nFat, nDifat int) {
	if nDifat == 0 {
		return
	}
	rest := make([]uint32, 0, nFat-109)
	for i := 109; i < nFat; i++ {
		rest = append(rest, uint32(i))
	}
	for s := 0; s < nDifat; s++ {
		off := (1 + nFat + s) * oleSec
		for i := 0; i < 127; i++ {
			idx := s*127 + i
			val := uint32(oleFree)
			if idx < len(rest) {
				val = rest[idx]
			}
			binary.LittleEndian.PutUint32(file[off+i*4:], val)
		}
		next := uint32(oleEnd)
		if s+1 < nDifat {
			next = uint32(nFat + s + 1)
		}
		binary.LittleEndian.PutUint32(file[off+127*4:], next)
	}
}
