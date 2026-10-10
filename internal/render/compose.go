package render

// Part is one module image placed in its section (sections are stacked top to bottom).
type Part struct {
	Img                *Canvas
	SectionW, SectionH int
}

// Assemble stacks the parts vertically, centring each image in its section, and writes the
// info line into the bottom infoHeight pixels (skipped when infoHeight is 0).
func Assemble(w, h int, parts []Part, border bool, info string, infoHeight int) *Canvas {
	out := NewCanvas(w, h, FontDefault, 14)
	cursor := 0
	for _, p := range parts {
		if border {
			p.Img.RoundedRect(1, 1, p.Img.W-2, p.Img.H-2, 5, InkBlack)
		}
		x := (p.SectionW - p.Img.W) / 2
		y := cursor + (p.SectionH-p.Img.H)/2
		out.Paste(p.Img, x, y)
		cursor += p.SectionH
	}
	if infoHeight > 0 && info != "" {
		out.Write(0, h-infoHeight, w, infoHeight, info, TextOpts{})
	}
	return out
}
