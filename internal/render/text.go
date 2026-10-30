package render

import (
	"image"
	"strings"

	"golang.org/x/image/font"
	"golang.org/x/image/math/fixed"
)

// Align is the horizontal text alignment inside a box.
type Align int

const (
	AlignCenter Align = iota
	AlignLeft
	AlignRight
)

// TextOpts configures Write.
type TextOpts struct {
	Align      Align
	Ink        Ink
	Autofit    bool    // pick the size whose "Ag" height is ~FillHeight*h (does not change the canvas font)
	FillHeight float64 // 0 means 0.8
}

// LineHeight is ascent+descent of the current font.
func (c *Canvas) LineHeight() int { return lineHeight(c.face(c.fontID, c.fontSize)) }

// TextWidth is the advance width of s in the current font.
func (c *Canvas) TextWidth(s string) int {
	return font.MeasureString(c.face(c.fontID, c.fontSize), s).Ceil()
}

// Wrap splits text on spaces into lines no wider than maxWidth (a single over-long word stays on its own line).
func (c *Canvas) Wrap(text string, maxWidth int) []string {
	return wrap(c.face(c.fontID, c.fontSize), text, maxWidth)
}

func lineHeight(f font.Face) int {
	m := f.Metrics()
	return (m.Ascent + m.Descent).Ceil()
}

func wrap(f font.Face, text string, maxWidth int) []string {
	var lines []string
	cur := ""
	for word := range strings.SplitSeq(text, " ") {
		proposed := strings.TrimSpace(cur + " " + word)
		if font.MeasureString(f, proposed).Ceil() <= maxWidth {
			cur = proposed
			continue
		}
		if cur == "" {
			lines = append(lines, word)
		} else {
			lines = append(lines, cur)
			cur = word
		}
	}
	if cur != "" {
		lines = append(lines, cur)
	}
	return lines
}

// fitSize returns the largest size (>= 8) whose "Ag" ink height stays below target.
func (c *Canvas) fitSize(id FontID, target int) int {
	for size := 8; size < 400; size++ {
		b, _ := font.BoundString(c.face(id, size), "Ag")
		if (b.Max.Y - b.Min.Y).Ceil() >= target {
			if size > 8 {
				return size - 1
			}
			return 8
		}
	}
	return 400
}

// Write draws (multi-line, wrapped) text centred vertically in the box (x, y, w, h).
// Lines that do not fit are dropped from the bottom, but the first line is always drawn (clipped to the box).
func (c *Canvas) Write(x, y, w, h int, text string, o TextOpts) {
	if w <= 0 || h <= 0 {
		return
	}
	fill := o.FillHeight
	if fill == 0 {
		fill = 0.8
	}
	size := c.fontSize
	if o.Autofit {
		size = c.fitSize(c.fontID, int(float64(h)*fill))
	}
	face := c.face(c.fontID, size)

	var lines []string
	for logical := range strings.SplitSeq(text, "\n") {
		lines = append(lines, wrap(face, logical, w)...)
	}
	if len(lines) == 0 {
		return
	}
	lh := lineHeight(face)
	spacing := int(float64(size) * 0.2)
	n := len(lines)
	for n > 1 && n*lh+(n-1)*spacing > h {
		n--
	}
	lines = lines[:n]
	total := n*lh + (n-1)*spacing
	top := y + (h-total)/2
	ascent := face.Metrics().Ascent.Ceil()

	clip := image.Rect(x, y, x+w, y+h)
	imgs, col := c.layers(o.Ink)
	for i, line := range lines {
		lw := font.MeasureString(face, line).Ceil()
		px := x + (w-lw)/2
		switch o.Align {
		case AlignLeft:
			px = x
		case AlignRight:
			px = x + w - lw
		}
		baseline := top + i*(lh+spacing) + ascent
		for _, img := range imgs {
			d := font.Drawer{
				Dst:  img.SubImage(clip).(*image.Gray),
				Src:  image.NewUniform(col),
				Face: face,
				Dot:  fixed.P(px, baseline),
			}
			d.DrawString(line)
		}
	}
}

// Icon draws a single glyph scaled so its ink fills fillRatio of the box, centred by its ink bounds.
func (c *Canvas) Icon(x, y, w, h int, glyph string, id FontID, ink Ink, fillRatio float64) {
	if w <= 0 || h <= 0 {
		return
	}
	if fillRatio == 0 {
		fillRatio = 0.9
	}
	size := 8
	for ; size < 400; size++ {
		b, _ := font.BoundString(c.face(id, size), glyph)
		bw, bh := (b.Max.X - b.Min.X).Ceil(), (b.Max.Y - b.Min.Y).Ceil()
		if float64(bw) >= float64(w)*fillRatio || float64(bh) >= float64(h)*fillRatio {
			if size > 8 {
				size--
			}
			break
		}
	}
	face := c.face(id, size)
	b, _ := font.BoundString(face, glyph)
	bw, bh := (b.Max.X - b.Min.X).Ceil(), (b.Max.Y - b.Min.Y).Ceil()
	left := x + (w-bw)/2
	top := y + (h-bh)/2
	dot := fixed.Point26_6{X: fixed.I(left) - b.Min.X, Y: fixed.I(top) - b.Min.Y}
	imgs, col := c.layers(ink)
	for _, img := range imgs {
		d := font.Drawer{Dst: img, Src: image.NewUniform(col), Face: face, Dot: dot}
		d.DrawString(glyph)
	}
}
