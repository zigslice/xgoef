package bundle

import (
	"encoding/binary"
	"fmt"
	"sort"
	"strings"
)

const (
	sqMagic       = 0x73717368
	sqMetaMax     = 8192
	sqBlockSize   = 1 << 20
	sqBlockLog    = 20
	sqInvalidFrag = 0xffffffff
	sqInvalidOff  = ^uint64(0)
	sqNoI         = 1 << 0
	sqNoD         = 1 << 1
	sqNoF         = 1 << 3
	sqNoFrag      = 1 << 4
	sqNoX         = 1 << 8
	sqNoXattr     = 1 << 9
	sqNoID        = 1 << 11
)

type sqNode struct {
	name   string
	data   []byte
	link   string
	mode   uint16
	isDir  bool
	isLink bool
	kids   []*sqNode
	ino    uint32
	iblock uint32
	ioff   uint32
	dblock uint32
	doff   uint32
	dlen   int
	parent uint32
	dataAt uint32
}

func writeSquashFS(paths map[string][]byte, modes map[string]uint16, links map[string]string) ([]byte, error) {
	root := &sqNode{name: "", isDir: true, mode: 0o755, kids: nil}
	add := func(path string, node *sqNode) error {
		path = strings.Trim(path, "/")
		if path == "" || path == "." {
			return fmt.Errorf("bundle: squashfs path %q is empty", path)
		}
		parts := strings.Split(path, "/")
		cur := root
		for i, part := range parts {
			if part == "" || part == "." || part == ".." || strings.ContainsRune(part, 0) {
				return fmt.Errorf("bundle: squashfs path %q leaves the image", path)
			}
			var next *sqNode
			for _, kid := range cur.kids {
				if kid.name == part {
					next = kid
					break
				}
			}
			if i == len(parts)-1 {
				if next != nil {
					return fmt.Errorf("bundle: squashfs path %q is duplicated", path)
				}
				node.name = part
				cur.kids = append(cur.kids, node)
				return nil
			}
			if next != nil {
				if !next.isDir {
					return fmt.Errorf("bundle: squashfs path %q is duplicated", path)
				}
				cur = next
				continue
			}
			next = &sqNode{name: part, isDir: true, mode: 0o755}
			cur.kids = append(cur.kids, next)
			cur = next
		}
		return nil
	}
	var names []string
	for name := range paths {
		names = append(names, name)
	}
	for name := range links {
		names = append(names, name)
	}
	sort.Strings(names)
	seen := map[string]bool{}
	for _, name := range names {
		if seen[name] {
			return nil, fmt.Errorf("bundle: squashfs path %q is duplicated", name)
		}
		seen[name] = true
		if target, ok := links[name]; ok {
			if err := add(name, &sqNode{isLink: true, link: target, mode: 0o777}); err != nil {
				return nil, err
			}
			continue
		}
		mode := modes[name]
		if mode == 0 {
			mode = 0o644
		}
		if err := add(name, &sqNode{data: paths[name], mode: mode & 0o777}); err != nil {
			return nil, err
		}
	}
	var flat []*sqNode
	var walk func(*sqNode)
	walk = func(n *sqNode) {
		sort.Slice(n.kids, func(i, j int) bool { return n.kids[i].name < n.kids[j].name })
		n.ino = uint32(len(flat) + 1)
		flat = append(flat, n)
		for _, kid := range n.kids {
			walk(kid)
		}
	}
	walk(root)
	if len(flat) > 65535 {
		return nil, fmt.Errorf("bundle: squashfs has too many inodes")
	}
	var image []byte
	image = append(image, make([]byte, 96)...)
	for _, n := range flat {
		if n.isDir || n.isLink {
			continue
		}
		if len(image) > 0xffffffff-len(n.data) {
			return nil, fmt.Errorf("bundle: squashfs image is too large")
		}
		n.dataAt = uint32(len(image))
		image = append(image, n.data...)
	}
	for _, n := range flat {
		if !n.isDir {
			continue
		}
		n.parent = root.ino
		if n != root {
			parent := parentOf(root, n)
			if parent != nil {
				n.parent = parent.ino
			}
		}
	}
	if err := placeInodes(flat); err != nil {
		return nil, err
	}
	dirBytes, err := packDirs(flat)
	if err != nil {
		return nil, err
	}
	inodeBlocks, err := emitInodes(flat)
	if err != nil {
		return nil, err
	}
	inodeAt := len(image)
	image = append(image, inodeBlocks...)
	dirAt := len(image)
	image = append(image, dirBytes...)
	idAt := len(image)
	image = append(image, metaBlock([]byte{0, 0, 0, 0})...)
	indexAt := len(image)
	image = append(image, putU64(nil, uint64(idAt))...)
	rootRef := uint64(root.iblock)<<16 | uint64(root.ioff)
	flags := uint16(sqNoI | sqNoD | sqNoF | sqNoFrag | sqNoX | sqNoXattr | sqNoID)
	binary.LittleEndian.PutUint32(image[0:], sqMagic)
	binary.LittleEndian.PutUint32(image[4:], uint32(len(flat)))
	binary.LittleEndian.PutUint32(image[8:], 0)
	binary.LittleEndian.PutUint32(image[12:], sqBlockSize)
	binary.LittleEndian.PutUint32(image[16:], 0)
	binary.LittleEndian.PutUint16(image[20:], 1)
	binary.LittleEndian.PutUint16(image[22:], sqBlockLog)
	binary.LittleEndian.PutUint16(image[24:], flags)
	binary.LittleEndian.PutUint16(image[26:], 1)
	binary.LittleEndian.PutUint16(image[28:], 4)
	binary.LittleEndian.PutUint16(image[30:], 0)
	binary.LittleEndian.PutUint64(image[32:], rootRef)
	binary.LittleEndian.PutUint64(image[40:], uint64(len(image)))
	binary.LittleEndian.PutUint64(image[48:], uint64(indexAt))
	binary.LittleEndian.PutUint64(image[56:], sqInvalidOff)
	binary.LittleEndian.PutUint64(image[64:], uint64(inodeAt))
	binary.LittleEndian.PutUint64(image[72:], uint64(dirAt))
	binary.LittleEndian.PutUint64(image[80:], sqInvalidOff)
	binary.LittleEndian.PutUint64(image[88:], sqInvalidOff)
	return image, nil
}

func parentOf(root, target *sqNode) *sqNode {
	var found *sqNode
	var walk func(*sqNode)
	walk = func(n *sqNode) {
		for _, kid := range n.kids {
			if kid == target {
				found = n
				return
			}
			if kid.isDir {
				walk(kid)
			}
		}
	}
	walk(root)
	return found
}

func inodeSize(n *sqNode) (int, error) {
	switch {
	case n.isDir:
		return 32, nil
	case n.isLink:
		return 24 + len(n.link), nil
	default:
		blocks := 0
		if len(n.data) > 0 {
			blocks = (len(n.data) + sqBlockSize - 1) / sqBlockSize
		}
		return 32 + 4*blocks, nil
	}
}

func placeInodes(nodes []*sqNode) error {
	cur := 0
	block := uint32(0)
	for _, n := range nodes {
		size, err := inodeSize(n)
		if err != nil {
			return err
		}
		if size > sqMetaMax {
			return fmt.Errorf("bundle: squashfs inode for %q is too large", n.name)
		}
		if cur+size > sqMetaMax {
			block++
			cur = 0
		}
		n.iblock = block
		n.ioff = uint32(cur)
		cur += size
	}
	return nil
}

func emitInodes(nodes []*sqNode) ([]byte, error) {
	var blocks [][]byte
	cur := []byte{}
	for _, n := range nodes {
		body, err := encodeInode(n)
		if err != nil {
			return nil, err
		}
		size, err := inodeSize(n)
		if err != nil {
			return nil, err
		}
		if len(body) != size {
			return nil, fmt.Errorf("bundle: squashfs inode for %q is %d bytes", n.name, len(body))
		}
		if len(cur)+len(body) > sqMetaMax {
			blocks = append(blocks, cur)
			cur = nil
		}
		cur = append(cur, body...)
	}
	if len(cur) > 0 {
		blocks = append(blocks, cur)
	}
	var out []byte
	for _, block := range blocks {
		out = append(out, metaBlock(block)...)
	}
	return out, nil
}

func encodeInode(n *sqNode) ([]byte, error) {
	var b []byte
	kind := uint16(2)
	switch {
	case n.isDir:
		kind = 1
	case n.isLink:
		kind = 3
	}
	b = append(b, putU16(nil, kind)...)
	b = append(b, putU16(nil, n.mode)...)
	b = append(b, putU16(nil, 0)...)
	b = append(b, putU16(nil, 0)...)
	b = append(b, putU32(nil, 0)...)
	b = append(b, putU32(nil, n.ino)...)
	switch {
	case n.isDir:
		nlink := uint32(2)
		for _, kid := range n.kids {
			if kid.isDir {
				nlink++
			}
		}
		b = append(b, putU32(nil, n.dblock)...)
		b = append(b, putU32(nil, nlink)...)
		size := n.dlen + 3
		if size > 65535 {
			return nil, fmt.Errorf("bundle: squashfs directory %q is too large", n.name)
		}
		b = append(b, putU16(nil, uint16(size))...)
		b = append(b, putU16(nil, uint16(n.doff))...)
		b = append(b, putU32(nil, n.parent)...)
	case n.isLink:
		if len(n.link) > 65535 {
			return nil, fmt.Errorf("bundle: squashfs symlink %q is too long", n.name)
		}
		b = append(b, putU32(nil, 1)...)
		b = append(b, putU32(nil, uint32(len(n.link)))...)
		b = append(b, []byte(n.link)...)
	default:
		b = append(b, putU32(nil, n.dataAt)...)
		b = append(b, putU32(nil, sqInvalidFrag)...)
		b = append(b, putU32(nil, 0)...)
		b = append(b, putU32(nil, uint32(len(n.data)))...)
		rest := len(n.data)
		for rest > 0 {
			n := rest
			if n > sqBlockSize {
				n = sqBlockSize
			}
			b = append(b, putU32(nil, uint32(n)|0x01000000)...)
			rest -= n
		}
	}
	return b, nil
}

func packDirs(nodes []*sqNode) ([]byte, error) {
	var stream []byte
	for _, n := range nodes {
		if !n.isDir {
			continue
		}
		body, err := encodeDir(n)
		if err != nil {
			return nil, err
		}
		n.dlen = len(body)
		n.dblock = uint32(len(stream) / sqMetaMax)
		n.doff = uint32(len(stream) % sqMetaMax)
		if n.doff != 0 && int(n.doff)+len(body) > sqMetaMax {
			pad := sqMetaMax - int(n.doff)
			stream = append(stream, make([]byte, pad)...)
			n.dblock = uint32(len(stream) / sqMetaMax)
			n.doff = 0
		}
		stream = append(stream, body...)
	}
	var out []byte
	for len(stream) > 0 {
		n := len(stream)
		if n > sqMetaMax {
			n = sqMetaMax
		}
		out = append(out, metaBlock(stream[:n])...)
		stream = stream[n:]
	}
	return out, nil
}

func encodeDir(n *sqNode) ([]byte, error) {
	if len(n.kids) == 0 {
		return nil, fmt.Errorf("bundle: squashfs directory %q is empty", n.name)
	}
	type group struct {
		block uint32
		kids  []*sqNode
	}
	var groups []group
	for _, kid := range n.kids {
		if len(groups) == 0 || groups[len(groups)-1].block != kid.iblock || len(groups[len(groups)-1].kids) == 256 {
			groups = append(groups, group{block: kid.iblock})
		}
		g := &groups[len(groups)-1]
		if len(g.kids) > 0 {
			min, max := g.kids[0].ino, g.kids[0].ino
			for _, have := range g.kids {
				if have.ino < min {
					min = have.ino
				}
				if have.ino > max {
					max = have.ino
				}
			}
			if kid.ino < min {
				min = kid.ino
			}
			if kid.ino > max {
				max = kid.ino
			}
			if max-min > 32767 {
				groups = append(groups, group{block: kid.iblock, kids: []*sqNode{kid}})
				continue
			}
		}
		g.kids = append(g.kids, kid)
	}
	var b []byte
	for _, g := range groups {
		min := g.kids[0].ino
		for _, kid := range g.kids {
			if kid.ino < min {
				min = kid.ino
			}
		}
		b = append(b, putU32(nil, uint32(len(g.kids)-1))...)
		b = append(b, putU32(nil, g.block)...)
		b = append(b, putU32(nil, min)...)
		for _, kid := range g.kids {
			kind := uint16(2)
			switch {
			case kid.isDir:
				kind = 1
			case kid.isLink:
				kind = 3
			}
			b = append(b, putU16(nil, uint16(kid.ioff))...)
			b = append(b, putU16(nil, uint16(kid.ino-min))...)
			b = append(b, putU16(nil, kind)...)
			name := []byte(kid.name)
			if len(name) == 0 || len(name) > 256 {
				return nil, fmt.Errorf("bundle: squashfs name %q is invalid", kid.name)
			}
			b = append(b, putU16(nil, uint16(len(name)-1))...)
			b = append(b, name...)
		}
	}
	return b, nil
}

func metaBlock(payload []byte) []byte {
	hdr := uint16(len(payload)) | 0x8000
	return append(putU16(nil, hdr), payload...)
}

func putU16(b []byte, v uint16) []byte {
	var buf [2]byte
	binary.LittleEndian.PutUint16(buf[:], v)
	return append(b, buf[:]...)
}

func putU32(b []byte, v uint32) []byte {
	var buf [4]byte
	binary.LittleEndian.PutUint32(buf[:], v)
	return append(b, buf[:]...)
}

func putU64(b []byte, v uint64) []byte {
	var buf [8]byte
	binary.LittleEndian.PutUint64(buf[:], v)
	return append(b, buf[:]...)
}
