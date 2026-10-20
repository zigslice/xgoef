package bundle

import (
	"crypto/sha1"
	"encoding/binary"
	"fmt"
	"path"
	"strings"
)

const (
	msiValid    = 0x0100
	msiLocal    = 0x0200
	msiString   = 0x0800
	msiNullable = 0x1000
	msiKey      = 0x2000
)

type msiCol struct {
	name  string
	width int
	typ   uint16
	str   bool
}

type msiCell struct {
	s    string
	n    int
	null bool
	str  bool
}

func sCell(v string) msiCell { return msiCell{s: v, str: true, null: v == ""} }
func nCell(v int) msiCell    { return msiCell{n: v} }
func zCell() msiCell         { return msiCell{null: true} }

func writeMSI(pathName string, spec Spec, files []fileItem) error {
	body, err := buildMSI(spec, files)
	if err != nil {
		return err
	}
	return writeFile(pathName, body, 0o644)
}

func buildMSI(spec Spec, files []fileItem) ([]byte, error) {
	if len(files) == 0 {
		return nil, fmt.Errorf("bundle: no files")
	}
	version, err := msiVersion(spec.Version)
	if err != nil {
		return nil, err
	}
	folder := segment(spec.Product)
	dirs := []msiDir{{id: "TARGETDIR", name: "SourceDir"}, {id: "ProgramFilesFolder", parent: "TARGETDIR", name: "."}, {id: "INSTALLDIR", parent: "ProgramFilesFolder", name: folder}}
	dirID := map[string]string{"": "INSTALLDIR"}
	nextDir := 0
	var cabNames []string
	var cabData [][]byte
	var fileRows [][]msiCell
	var compRows [][]msiCell
	var featRows [][]msiCell
	for i, file := range files {
		rel := path.Clean(file.Rel)
		dir := path.Dir(rel)
		if dir == "." {
			dir = ""
		}
		parent := "INSTALLDIR"
		acc := ""
		if dir != "" {
			for _, part := range strings.Split(dir, "/") {
				if acc == "" {
					acc = part
				} else {
					acc = acc + "/" + part
				}
				if id, ok := dirID[acc]; ok {
					parent = id
					continue
				}
				id := fmt.Sprintf("D%d", nextDir)
				nextDir++
				dirs = append(dirs, msiDir{id: id, parent: parent, name: segment(part)})
				dirID[acc] = id
				parent = id
			}
		}
		where := "INSTALLDIR"
		if dir != "" {
			where = dirID[dir]
		}
		id := fmt.Sprintf("F%d", i+1)
		comp := fmt.Sprintf("C%d", i+1)
		base := path.Base(rel)
		fileRows = append(fileRows, []msiCell{
			sCell(id), sCell(comp), sCell(msiFileName(base)), nCell(len(file.Data)),
			zCell(), zCell(), nCell(512), nCell(i + 1),
		})
		compRows = append(compRows, []msiCell{
			sCell(comp), sCell(stableGUID("component:" + spec.Product + ":" + rel)), sCell(where), nCell(0), zCell(), sCell(id),
		})
		featRows = append(featRows, []msiCell{sCell("Main"), sCell(comp)})
		cabNames = append(cabNames, id)
		cabData = append(cabData, file.Data)
	}
	var dirRows [][]msiCell
	for _, dir := range dirs {
		parent := dir.parent
		if parent == "" {
			dirRows = append(dirRows, []msiCell{sCell(dir.id), zCell(), sCell(dir.name)})
			continue
		}
		dirRows = append(dirRows, []msiCell{sCell(dir.id), sCell(parent), sCell(dir.name)})
	}
	product := stableGUID("product:" + spec.Identifier + ":" + spec.Version + ":" + spec.Product)
	upgrade := stableGUID("upgrade:" + spec.Identifier + ":" + spec.Product)
	maker := spec.Identifier
	if maker == "" {
		maker = spec.Product
	}
	tables := []msiTable{
		{name: "Property", cols: []msiCol{
			colStr("Property", 72, true, false, false),
			colStr("Value", 0, false, false, true),
		}, rows: [][]msiCell{
			{sCell("ProductName"), sCell(spec.Product)},
			{sCell("ProductVersion"), sCell(version)},
			{sCell("Manufacturer"), sCell(maker)},
			{sCell("ProductLanguage"), sCell("1033")},
			{sCell("ProductCode"), sCell(product)},
			{sCell("UpgradeCode"), sCell(upgrade)},
			{sCell("ALLUSERS"), sCell("1")},
		}},
		{name: "Directory", cols: []msiCol{
			colStr("Directory", 72, true, false, false),
			colStr("Directory_Parent", 72, false, true, false),
			colStr("DefaultDir", 255, false, false, true),
		}, rows: dirRows},
		{name: "Component", cols: []msiCol{
			colStr("Component", 72, true, false, false),
			colStr("ComponentId", 38, false, true, false),
			colStr("Directory_", 72, false, false, false),
			colInt("Attributes", 2, false, false),
			colStr("Condition", 255, false, true, false),
			colStr("KeyPath", 72, false, true, false),
		}, rows: compRows},
		{name: "Feature", cols: []msiCol{
			colStr("Feature", 38, true, false, false),
			colStr("Feature_Parent", 38, false, true, false),
			colStr("Title", 64, false, true, true),
			colStr("Description", 255, false, true, true),
			colInt("Display", 2, false, true),
			colInt("Level", 2, false, false),
			colStr("Directory_", 72, false, true, false),
			colInt("Attributes", 2, false, false),
		}, rows: [][]msiCell{
			{sCell("Main"), zCell(), zCell(), zCell(), nCell(2), nCell(1), zCell(), nCell(0)},
		}},
		{name: "FeatureComponents", cols: []msiCol{
			colStr("Feature_", 38, true, false, false),
			colStr("Component_", 72, true, false, false),
		}, rows: featRows},
		{name: "File", cols: []msiCol{
			colStr("File", 72, true, false, false),
			colStr("Component_", 72, false, false, false),
			colStr("FileName", 255, false, false, true),
			colInt("FileSize", 4, false, false),
			colStr("Version", 72, false, true, false),
			colStr("Language", 20, false, true, false),
			colInt("Attributes", 2, false, true),
			colInt("Sequence", 4, false, false),
		}, rows: fileRows},
		{name: "Media", cols: []msiCol{
			colInt("DiskId", 2, true, false),
			colInt("LastSequence", 4, false, false),
			colStr("DiskPrompt", 64, false, true, true),
			colStr("Cabinet", 255, false, true, false),
			colStr("VolumeLabel", 32, false, true, false),
			colStr("Source", 72, false, true, false),
		}, rows: [][]msiCell{
			{nCell(1), nCell(len(files)), zCell(), sCell("#cab1.cab"), zCell(), zCell()},
		}},
		{name: "InstallExecuteSequence", cols: []msiCol{
			colStr("Action", 72, true, false, false),
			colStr("Condition", 255, false, true, false),
			colInt("Sequence", 2, false, true),
		}, rows: sequenceRows()},
	}
	pool := newPool()
	streams := []oleStream{
		{name: encodeStream(true, "_StringData"), data: nil},
		{name: encodeStream(true, "_StringPool"), data: nil},
	}
	var tableStreams []oleStream
	var colRows [][]msiCell
	var tableRows [][]msiCell
	for _, table := range tables {
		tableRows = append(tableRows, []msiCell{sCell(table.name)})
		for i, col := range table.cols {
			colRows = append(colRows, []msiCell{sCell(table.name), nCell(i + 1), sCell(col.name), nCell(int(col.typ))})
		}
		raw, err := packTable(pool, table.cols, table.rows)
		if err != nil {
			return nil, err
		}
		tableStreams = append(tableStreams, oleStream{name: encodeStream(true, table.name), data: raw})
	}
	colCols := []msiCol{
		colStr("Table", 64, true, false, false),
		colInt("Number", 2, true, false),
		colStr("Name", 64, false, false, false),
		colInt("Type", 2, false, false),
	}
	tableCols := []msiCol{colStr("Name", 64, true, false, false)}
	colRaw, err := packTable(pool, colCols, colRows)
	if err != nil {
		return nil, err
	}
	tableRaw, err := packTable(pool, tableCols, tableRows)
	if err != nil {
		return nil, err
	}
	data, poolBytes := pool.bytes()
	streams[0].data = data
	streams[1].data = poolBytes
	streams = append(streams,
		oleStream{name: encodeStream(true, "_Columns"), data: colRaw},
		oleStream{name: encodeStream(true, "_Tables"), data: tableRaw},
	)
	streams = append(streams, tableStreams...)
	cab, err := buildCab(cabNames, cabData)
	if err != nil {
		return nil, err
	}
	streams = append(streams, oleStream{name: encodeStream(false, "cab1.cab"), data: cab})
	return writeOLE(streams)
}

type msiDir struct {
	id, parent, name string
}

type msiTable struct {
	name string
	cols []msiCol
	rows [][]msiCell
}

func colStr(name string, n int, key, null, local bool) msiCol {
	typ := uint16(msiValid | msiString | n)
	if key {
		typ |= msiKey
	}
	if null {
		typ |= msiNullable
	}
	if local {
		typ |= msiLocal
	}
	return msiCol{name: name, width: 2, typ: typ, str: true}
}

func colInt(name string, width int, key, null bool) msiCol {
	typ := uint16(msiValid | width)
	if key {
		typ |= msiKey
	}
	if null {
		typ |= msiNullable
	}
	return msiCol{name: name, width: width, typ: typ}
}

func sequenceRows() [][]msiCell {
	steps := []struct {
		name string
		n    int
	}{
		{"ValidateProductID", 700},
		{"CostInitialize", 800},
		{"FileCost", 900},
		{"CostFinalize", 1000},
		{"InstallValidate", 1400},
		{"InstallInitialize", 1500},
		{"ProcessComponents", 1600},
		{"UnpublishFeatures", 1800},
		{"RemoveFiles", 3500},
		{"InstallFiles", 4000},
		{"RegisterUser", 6000},
		{"RegisterProduct", 6100},
		{"PublishFeatures", 6300},
		{"PublishProduct", 6400},
		{"InstallFinalize", 6600},
	}
	var rows [][]msiCell
	for _, step := range steps {
		rows = append(rows, []msiCell{sCell(step.name), zCell(), nCell(step.n)})
	}
	return rows
}

func msiFileName(name string) string {
	short := short11(name)
	base := strings.TrimRight(string(short[:8]), " ")
	ext := strings.TrimRight(string(short[8:]), " ")
	shown := base
	if ext != "" {
		shown += "." + ext
	}
	if strings.EqualFold(shown, name) {
		return name
	}
	return shown + "|" + name
}

type strPool struct {
	vals []string
	refs []uint16
	idx  map[string]int
}

func newPool() *strPool {
	return &strPool{vals: []string{""}, refs: []uint16{0}, idx: map[string]int{}}
}

func (p *strPool) intern(s string) int {
	if s == "" {
		return 0
	}
	if id, ok := p.idx[s]; ok {
		p.refs[id]++
		return id
	}
	p.vals = append(p.vals, s)
	p.refs = append(p.refs, 1)
	id := len(p.vals) - 1
	p.idx[s] = id
	return id
}

func (p *strPool) bytes() ([]byte, []byte) {
	pool := make([]byte, len(p.vals)*4)
	binary.LittleEndian.PutUint32(pool[:4], 65001)
	var data []byte
	for id := 1; id < len(p.vals); id++ {
		b := []byte(p.vals[id])
		binary.LittleEndian.PutUint16(pool[id*4:], uint16(len(b)))
		binary.LittleEndian.PutUint16(pool[id*4+2:], p.refs[id])
		data = append(data, b...)
	}
	return data, pool
}

func packTable(pool *strPool, cols []msiCol, rows [][]msiCell) ([]byte, error) {
	if len(rows) == 0 {
		return []byte{}, nil
	}
	rowSize := 0
	for _, col := range cols {
		rowSize += col.width
	}
	raw := make([]byte, rowSize*len(rows))
	ofs := 0
	for c, col := range cols {
		for i, row := range rows {
			if c >= len(row) {
				return nil, fmt.Errorf("bundle: short row in MSI table")
			}
			cell := row[c]
			buf := make([]byte, col.width)
			if col.str {
				id := 0
				if !cell.null {
					id = pool.intern(cell.s)
				}
				if id > 0xFFFF {
					return nil, fmt.Errorf("bundle: too many MSI strings")
				}
				binary.LittleEndian.PutUint16(buf, uint16(id))
			} else if cell.null {
				// stored zero means null
			} else if col.width == 2 {
				binary.LittleEndian.PutUint16(buf, uint16(cell.n+0x8000))
			} else if col.width == 4 {
				binary.LittleEndian.PutUint32(buf, uint32(cell.n)^0x80000000)
			} else {
				return nil, fmt.Errorf("bundle: bad MSI column width %d", col.width)
			}
			copy(raw[ofs*len(rows)+i*col.width:], buf)
		}
		ofs += col.width
	}
	return raw, nil
}

func stableGUID(seed string) string {
	sum := sha1.Sum([]byte(seed))
	sum[6] = (sum[6] & 0x0f) | 0x50
	sum[8] = (sum[8] & 0x3f) | 0x80
	return fmt.Sprintf("{%02X%02X%02X%02X-%02X%02X-%02X%02X-%02X%02X-%02X%02X%02X%02X%02X%02X}",
		sum[0], sum[1], sum[2], sum[3], sum[4], sum[5], sum[6], sum[7],
		sum[8], sum[9], sum[10], sum[11], sum[12], sum[13], sum[14], sum[15])
}

func encodeStream(table bool, name string) []uint16 {
	var out []uint16
	if table {
		out = append(out, 0x4840)
	}
	b := []byte(name)
	for i := 0; i < len(b); {
		ch := int(b[i])
		i++
		if ch < 0x80 && mimeVal(ch) >= 0 {
			v := uint32(mimeVal(ch) + 0x4800)
			if i < len(b) && b[i] < 0x80 {
				next := mimeVal(int(b[i]))
				if next >= 0 {
					v += (uint32(next) + 0x3ffffc0) << 6
					i++
				}
			}
			out = append(out, uint16(v))
			continue
		}
		out = append(out, uint16(ch))
	}
	return out
}

func mimeVal(x int) int {
	switch {
	case x >= '0' && x <= '9':
		return x - '0'
	case x >= 'A' && x <= 'Z':
		return x - 'A' + 10
	case x >= 'a' && x <= 'z':
		return x - 'a' + 36
	case x == '.':
		return 62
	case x == '_':
		return 63
	default:
		return -1
	}
}

func buildCab(names []string, datas [][]byte) ([]byte, error) {
	var folder []byte
	var files []cabFile
	for i, data := range datas {
		files = append(files, cabFile{name: names[i], off: len(folder), data: data})
		folder = append(folder, data...)
	}
	const chunk = 32768
	var blocks [][]byte
	for len(folder) > 0 {
		n := chunk
		if n > len(folder) {
			n = len(folder)
		}
		blocks = append(blocks, folder[:n])
		folder = folder[n:]
	}
	if len(blocks) == 0 {
		blocks = [][]byte{{}}
	}
	var fileBytes []byte
	for _, file := range files {
		name := append([]byte(file.name), 0)
		buf := make([]byte, 16+len(name))
		binary.LittleEndian.PutUint32(buf[0:], uint32(len(file.data)))
		binary.LittleEndian.PutUint32(buf[4:], uint32(file.off))
		binary.LittleEndian.PutUint16(buf[8:], 0)
		binary.LittleEndian.PutUint16(buf[10:], dosDate())
		binary.LittleEndian.PutUint16(buf[12:], 0)
		binary.LittleEndian.PutUint16(buf[14:], 0x20)
		copy(buf[16:], name)
		fileBytes = append(fileBytes, buf...)
	}
	var dataBytes []byte
	for _, block := range blocks {
		sizes := []byte{byte(len(block)), byte(len(block) >> 8), byte(len(block)), byte(len(block) >> 8)}
		sum := cabChecksum(block, cabChecksum(sizes, 0))
		hdr := make([]byte, 8)
		binary.LittleEndian.PutUint32(hdr[0:], sum)
		binary.LittleEndian.PutUint16(hdr[4:], uint16(len(block)))
		binary.LittleEndian.PutUint16(hdr[6:], uint16(len(block)))
		dataBytes = append(dataBytes, hdr...)
		dataBytes = append(dataBytes, block...)
	}
	header := make([]byte, 36)
	copy(header[0:], "MSCF")
	folderRec := make([]byte, 8)
	coffFiles := 36 + 8
	coffData := coffFiles + len(fileBytes)
	binary.LittleEndian.PutUint32(folderRec[0:], uint32(coffData))
	binary.LittleEndian.PutUint16(folderRec[4:], uint16(len(blocks)))
	total := coffData + len(dataBytes)
	binary.LittleEndian.PutUint32(header[8:], uint32(total))
	binary.LittleEndian.PutUint32(header[16:], uint32(coffFiles))
	header[24] = 3
	header[25] = 1
	binary.LittleEndian.PutUint16(header[26:], 1)
	binary.LittleEndian.PutUint16(header[28:], uint16(len(files)))
	out := append(append(append([]byte{}, header...), folderRec...), fileBytes...)
	out = append(out, dataBytes...)
	return out, nil
}

type cabFile struct {
	name string
	off  int
	data []byte
}

func cabChecksum(data []byte, seed uint32) uint32 {
	sum := seed
	i := 0
	for ; i+4 <= len(data); i += 4 {
		sum ^= uint32(data[i]) | uint32(data[i+1])<<8 | uint32(data[i+2])<<16 | uint32(data[i+3])<<24
	}
	var rem uint32
	switch len(data) - i {
	case 3:
		rem |= uint32(data[i+2]) << 16
		fallthrough
	case 2:
		rem |= uint32(data[i+1]) << 8
		fallthrough
	case 1:
		rem |= uint32(data[i])
		sum ^= rem
	}
	return sum
}

func dosDate() uint16 {
	return uint16((2026-1980)<<9 | 1<<5 | 1)
}
