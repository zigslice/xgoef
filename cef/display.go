package cef

import (
	"fmt"
	"strconv"
	"strings"
)

// Display is one monitor. x, y, width, height and the work* fields are
// density-independent pixels, the same space window-state stores. The pixel*
// fields are physical pixels from CEF's ConvertPointToPixels. scaleFactor is
// the display's device scale. rotation is 0, 90, 180, or 270.
type Display struct {
	ID              int64   `json:"id"`
	ScaleFactor     float64 `json:"scaleFactor"`
	X               int     `json:"x"`
	Y               int     `json:"y"`
	Width           int     `json:"width"`
	Height          int     `json:"height"`
	WorkX           int     `json:"workX"`
	WorkY           int     `json:"workY"`
	WorkWidth       int     `json:"workWidth"`
	WorkHeight      int     `json:"workHeight"`
	PixelX          int     `json:"pixelX"`
	PixelY          int     `json:"pixelY"`
	PixelWidth      int     `json:"pixelWidth"`
	PixelHeight     int     `json:"pixelHeight"`
	WorkPixelX      int     `json:"workPixelX"`
	WorkPixelY      int     `json:"workPixelY"`
	WorkPixelWidth  int     `json:"workPixelWidth"`
	WorkPixelHeight int     `json:"workPixelHeight"`
	Rotation        int     `json:"rotation"`
	Primary         bool    `json:"primary"`
}

// DisplayTable is the monitor list plus the display under one window.
// HasWindow is false when that label has no Views window.
type DisplayTable struct {
	HasWindow   bool
	WindowID    int64
	WindowScale float64
	Displays    []Display
}

// DefaultDisplayTable is the stub used when CEF is not linked.
// One primary 1920×1080 display at scale 1, with the window on it.
func DefaultDisplayTable() string {
	table := DisplayTable{
		HasWindow:   true,
		WindowID:    1,
		WindowScale: 1,
		Displays: []Display{{
			ID:              1,
			ScaleFactor:     1,
			X:               0,
			Y:               0,
			Width:           1920,
			Height:          1080,
			WorkX:           0,
			WorkY:           0,
			WorkWidth:       1920,
			WorkHeight:      1040,
			PixelX:          0,
			PixelY:          0,
			PixelWidth:      1920,
			PixelHeight:     1080,
			WorkPixelX:      0,
			WorkPixelY:      0,
			WorkPixelWidth:  1920,
			WorkPixelHeight: 1040,
			Rotation:        0,
			Primary:         true,
		}},
	}
	text, err := FormatDisplayTable(table)
	if err != nil {
		return ""
	}
	return text
}

// FormatDisplayTable writes the text the CEF shim also writes.
func FormatDisplayTable(table DisplayTable) (string, error) {
	if err := validateDisplayTable(table); err != nil {
		return "", err
	}
	var b strings.Builder
	b.WriteString("window ")
	if table.HasWindow {
		b.WriteString(strconv.FormatInt(table.WindowID, 10))
	} else {
		b.WriteByte('-')
	}
	b.WriteByte(' ')
	scale := table.WindowScale
	if !table.HasWindow {
		scale = 0
	}
	b.WriteString(formatScale(scale))
	b.WriteByte('\n')
	for _, d := range table.Displays {
		fmt.Fprintf(&b, "%d %s %d %d %d %d %d %d %d %d %d %d %d %d %d %d %d %d %d %d\n",
			d.ID, formatScale(d.ScaleFactor),
			d.X, d.Y, d.Width, d.Height,
			d.WorkX, d.WorkY, d.WorkWidth, d.WorkHeight,
			d.PixelX, d.PixelY, d.PixelWidth, d.PixelHeight,
			d.WorkPixelX, d.WorkPixelY, d.WorkPixelWidth, d.WorkPixelHeight,
			d.Rotation, boolBit(d.Primary))
	}
	return b.String(), nil
}

// ParseDisplayTable reads FormatDisplayTable output.
func ParseDisplayTable(text string) (DisplayTable, error) {
	lines := strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n")
	var kept []string
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" {
			kept = append(kept, line)
		}
	}
	if len(kept) == 0 {
		return DisplayTable{}, fmt.Errorf("cef: display table is empty")
	}
	header := strings.Fields(kept[0])
	if len(header) != 3 || header[0] != "window" {
		return DisplayTable{}, fmt.Errorf("cef: display window header")
	}
	var table DisplayTable
	if header[1] == "-" {
		table.HasWindow = false
	} else {
		id, err := strconv.ParseInt(header[1], 10, 64)
		if err != nil {
			return DisplayTable{}, fmt.Errorf("cef: display window id")
		}
		table.HasWindow = true
		table.WindowID = id
	}
	scale, err := parseScale(header[2], !table.HasWindow)
	if err != nil {
		return DisplayTable{}, fmt.Errorf("cef: display window scale")
	}
	table.WindowScale = scale
	seen := map[int64]bool{}
	primaries := 0
	for _, line := range kept[1:] {
		d, err := parseDisplayLine(line)
		if err != nil {
			return DisplayTable{}, err
		}
		if seen[d.ID] {
			return DisplayTable{}, fmt.Errorf("cef: display id %d repeated", d.ID)
		}
		seen[d.ID] = true
		if d.Primary {
			primaries++
		}
		table.Displays = append(table.Displays, d)
	}
	if len(table.Displays) > 0 && primaries != 1 {
		return DisplayTable{}, fmt.Errorf("cef: display primary count %d", primaries)
	}
	return table, nil
}

func parseDisplayLine(line string) (Display, error) {
	fields := strings.Fields(line)
	if len(fields) != 20 {
		return Display{}, fmt.Errorf("cef: display fields")
	}
	id, err := strconv.ParseInt(fields[0], 10, 64)
	if err != nil {
		return Display{}, fmt.Errorf("cef: display id")
	}
	scale, err := parseScale(fields[1], false)
	if err != nil {
		return Display{}, fmt.Errorf("cef: display scale")
	}
	nums := make([]int, 18)
	for i := 0; i < 18; i++ {
		n, err := strconv.ParseInt(fields[i+2], 10, 32)
		if err != nil {
			return Display{}, fmt.Errorf("cef: display number")
		}
		nums[i] = int(n)
	}
	d := Display{
		ID:              id,
		ScaleFactor:     scale,
		X:               nums[0],
		Y:               nums[1],
		Width:           nums[2],
		Height:          nums[3],
		WorkX:           nums[4],
		WorkY:           nums[5],
		WorkWidth:       nums[6],
		WorkHeight:      nums[7],
		PixelX:          nums[8],
		PixelY:          nums[9],
		PixelWidth:      nums[10],
		PixelHeight:     nums[11],
		WorkPixelX:      nums[12],
		WorkPixelY:      nums[13],
		WorkPixelWidth:  nums[14],
		WorkPixelHeight: nums[15],
		Rotation:        nums[16],
		Primary:         nums[17] == 1,
	}
	if nums[17] != 0 && nums[17] != 1 {
		return Display{}, fmt.Errorf("cef: display primary")
	}
	if d.Width <= 0 || d.Height <= 0 || d.WorkWidth <= 0 || d.WorkHeight <= 0 || d.PixelWidth <= 0 || d.PixelHeight <= 0 || d.WorkPixelWidth <= 0 || d.WorkPixelHeight <= 0 {
		return Display{}, fmt.Errorf("cef: display size")
	}
	switch d.Rotation {
	case 0, 90, 180, 270:
	default:
		return Display{}, fmt.Errorf("cef: display rotation")
	}
	return d, nil
}

func validateDisplayTable(table DisplayTable) error {
	if table.HasWindow {
		if _, err := parseScale(formatScale(table.WindowScale), false); err != nil {
			return fmt.Errorf("cef: display window scale")
		}
	}
	seen := map[int64]bool{}
	primaries := 0
	for _, d := range table.Displays {
		if seen[d.ID] {
			return fmt.Errorf("cef: display id %d repeated", d.ID)
		}
		seen[d.ID] = true
		if _, err := parseScale(formatScale(d.ScaleFactor), false); err != nil {
			return fmt.Errorf("cef: display scale")
		}
		if d.Width <= 0 || d.Height <= 0 || d.WorkWidth <= 0 || d.WorkHeight <= 0 || d.PixelWidth <= 0 || d.PixelHeight <= 0 || d.WorkPixelWidth <= 0 || d.WorkPixelHeight <= 0 {
			return fmt.Errorf("cef: display size")
		}
		switch d.Rotation {
		case 0, 90, 180, 270:
		default:
			return fmt.Errorf("cef: display rotation")
		}
		if d.Primary {
			primaries++
		}
	}
	if len(table.Displays) > 0 && primaries != 1 {
		return fmt.Errorf("cef: display primary count %d", primaries)
	}
	return nil
}

func formatScale(scale float64) string {
	return strconv.FormatFloat(scale, 'f', 6, 64)
}

func parseScale(raw string, allowZero bool) (float64, error) {
	scale, err := strconv.ParseFloat(raw, 64)
	if err != nil {
		return 0, err
	}
	if scale == 0 && allowZero {
		return 0, nil
	}
	if scale <= 0 || scale >= 100 {
		return 0, fmt.Errorf("cef: display scale")
	}
	return scale, nil
}

func boolBit(v bool) int {
	if v {
		return 1
	}
	return 0
}
