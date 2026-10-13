package render

import (
	"image"

	"golang.org/x/image/font"
	"golang.org/x/image/math/fixed"
)

// Style is one step of a typographic scale: a font at a pixel size.
type Style struct {
	Font FontID
	Size int
}

// Advance is the width the text occupies when set in st.
func (c *Canvas) Advance(s string, st Style) int {
	return font.MeasureString(c.face(st.Font, st.Size), s).Ceil()
}

// Ascent and descent of the style's font metrics, in pixels.
func (c *Canvas) Metrics(st Style) (ascent, descent int) {
	m := c.face(st.Font, st.Size).Metrics()
	return m.Ascent.Ceil(), m.Descent.Ceil()
}

// CapHeight is the inked height of "H" in st (digits share it in Noto).
func (c *Canvas) CapHeight(st Style) int {
	b, _ := font.BoundString(c.face(st.Font, st.Size), "H")
	return (b.Max.Y - b.Min.Y).Ceil()
}

// Text sets s with its baseline at y, starting at x, and returns the advance width.
func (c *Canvas) Text(x, y int, s string, st Style, ink Ink) int {
	face := c.face(st.Font, st.Size)
	imgs, col := c.layers(ink)
	for _, img := range imgs {
		d := font.Drawer{Dst: img, Src: image.NewUniform(col), Face: face, Dot: fixed.P(x, y)}
		d.DrawString(s)
	}
	return font.MeasureString(face, s).Ceil()
}

// TextRight sets s so that it ends at x.
func (c *Canvas) TextRight(x, y int, s string, st Style, ink Ink) {
	c.Text(x-c.Advance(s, st), y, s, st, ink)
}

// TextCenter sets s centred on x.
func (c *Canvas) TextCenter(x, y int, s string, st Style, ink Ink) {
	c.Text(x-c.Advance(s, st)/2, y, s, st, ink)
}

// Fit shortens s with "…" until it is at most maxW wide in st ("" if nothing fits).
func (c *Canvas) Fit(s string, st Style, maxW int) string {
	if c.Advance(s, st) <= maxW {
		return s
	}
	r := []rune(s)
	for n := len(r) - 1; n > 0; n-- {
		t := string(r[:n])
		for len(t) > 0 && t[len(t)-1] == ' ' {
			t = t[:len(t)-1]
		}
		if t += "…"; c.Advance(t, st) <= maxW {
			return t
		}
	}
	return ""
}

// FillRect paints the rectangle x..x+w-1, y..y+h-1.
func (c *Canvas) FillRect(x, y, w, h int, ink Ink) {
	for yy := y; yy < y+h; yy++ {
		c.HLine(x, x+w-1, yy, ink)
	}
}
