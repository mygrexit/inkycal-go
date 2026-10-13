package render

// Part is one module image placed in its section (sections are stacked top to bottom).
type Part struct {
	Img                *Canvas
	SectionW, SectionH int
}

// Layout holds the page-level options of Assemble.
type Layout struct {
	Border     bool // rounded border around every module
	Dividers   bool // hairline between sections, as wide as the module below it
	Info       string
	InfoHeight int // bottom pixels for the info line; 0 = none
}

// Assemble stacks the parts vertically, centring each image in its section, and writes the
// info line into the bottom InfoHeight pixels.
func Assemble(w, h int, parts []Part, l Layout) *Canvas {
	out := NewCanvas(w, h, FontDefault, 14)
	cursor := 0
	for i, p := range parts {
		if l.Border {
			p.Img.RoundedRect(1, 1, p.Img.W-2, p.Img.H-2, 5, InkBlack)
		}
		x := (p.SectionW - p.Img.W) / 2
		y := cursor + (p.SectionH-p.Img.H)/2
		out.Paste(p.Img, x, y)
		if l.Dividers && i > 0 {
			out.HLine(x, x+p.Img.W-1, cursor, InkBlack)
		}
		cursor += p.SectionH
	}
	info, infoHeight := l.Info, l.InfoHeight
	if infoHeight > 0 && info != "" {
		out.Write(0, h-infoHeight, w, infoHeight, info, TextOpts{})
	}
	return out
}
