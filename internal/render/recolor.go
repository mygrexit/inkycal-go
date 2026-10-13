package render

import "image/color"

// InkMap gives a module its own colours: what its paper, its ink and its accent
// (colour layer) become. Only the panel's inks are possible.
type InkMap struct {
	Paper, Ink, Accent Ink
}

// DefaultInks is the normal look: white paper, black ink, colour accent.
var DefaultInks = InkMap{Paper: InkWhite, Ink: InkBlack, Accent: InkColour}

// layerValues are the two layer bytes that show one ink exactly.
func layerValues(ink Ink) (black, colour uint8) {
	switch ink {
	case InkBlack:
		return 0, 255
	case InkColour:
		return 255, 0
	}
	return 255, 255
}

// Recolor maps every pixel to the ink map (colour wins over black, as on the panel).
// Antialiased edges become crisp, as the panel shows them anyway.
func (c *Canvas) Recolor(m InkMap) {
	if m == DefaultInks {
		return
	}
	for y := 0; y < c.H; y++ {
		for x := 0; x < c.W; x++ {
			ink := m.Paper
			switch black, colour := c.InkAt(x, y); {
			case colour:
				ink = m.Accent
			case black:
				ink = m.Ink
			}
			b, col := layerValues(ink)
			c.Black.SetGray(x, y, color.Gray{Y: b})
			c.Colour.SetGray(x, y, color.Gray{Y: col})
		}
	}
}

// PasteOpaque copies src including its paper, so white ink on a dark tile survives.
func (c *Canvas) PasteOpaque(src *Canvas, x, y int) {
	for sy := 0; sy < src.H; sy++ {
		for sx := 0; sx < src.W; sx++ {
			dx, dy := x+sx, y+sy
			if dx < 0 || dy < 0 || dx >= c.W || dy >= c.H {
				continue
			}
			c.Black.SetGray(dx, dy, src.Black.GrayAt(sx, sy))
			c.Colour.SetGray(dx, dy, src.Colour.GrayAt(sx, sy))
		}
	}
}

// fillSection paints a module's whole section (padding included) with its paper.
func (c *Canvas) fillSection(x, y, w, h int, m *InkMap) {
	if m == nil || m.Paper == InkWhite {
		return
	}
	b, col := layerValues(m.Paper)
	for yy := max(y, 0); yy < min(y+h, c.H); yy++ {
		for xx := max(x, 0); xx < min(x+w, c.W); xx++ {
			c.Black.SetGray(xx, yy, color.Gray{Y: b})
			c.Colour.SetGray(xx, yy, color.Gray{Y: col})
		}
	}
}
