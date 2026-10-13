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
	writeInfo(out, w, h, l)
	return out
}

// Placed is one module image and its section rectangle on the page (grid layouts).
type Placed struct {
	Img        *Canvas
	X, Y, W, H int
}

// AssembleGrid centres each image in its rectangle. Dividers run along the top and left
// edge of every module not touching the page edge, as long as the module image.
func AssembleGrid(w, h int, parts []Placed, l Layout) *Canvas {
	out := NewCanvas(w, h, FontDefault, 14)
	for _, p := range parts {
		if l.Border {
			p.Img.RoundedRect(1, 1, p.Img.W-2, p.Img.H-2, 5, InkBlack)
		}
		x := p.X + (p.W-p.Img.W)/2
		y := p.Y + (p.H-p.Img.H)/2
		out.Paste(p.Img, x, y)
		if l.Dividers {
			if p.Y > 0 {
				out.HLine(x, x+p.Img.W-1, p.Y, InkBlack)
			}
			if p.X > 0 {
				out.VLine(p.X, y, y+p.Img.H-1, InkBlack)
			}
		}
	}
	writeInfo(out, w, h, l)
	return out
}

// writeInfo sets the info line into the bottom InfoHeight pixels.
func writeInfo(out *Canvas, w, h int, l Layout) {
	if l.InfoHeight > 0 && l.Info != "" {
		out.Write(0, h-l.InfoHeight, w, l.InfoHeight, l.Info, TextOpts{})
	}
}
