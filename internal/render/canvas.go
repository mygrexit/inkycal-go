// Package render draws Inkycal module images on a two-layer (black + colour) canvas.
package render

import (
	"image"
	"image/color"

	"golang.org/x/image/font"
	"golang.org/x/image/font/opentype"
)

// InkThreshold: a pixel value <= InkThreshold counts as ink (same as Python's _optimize_im).
const InkThreshold = 220

// Ink selects the layer(s) a drawing operation affects.
type Ink int

const (
	InkBlack  Ink = iota // black layer only
	InkColour            // colour layer only (shown red)
	InkWhite             // white on both layers
)

type faceKey struct {
	id   FontID
	size int
}

// Canvas holds two greyscale layers; 255 is white (no ink).
type Canvas struct {
	W, H   int
	Black  *image.Gray
	Colour *image.Gray

	fontID   FontID
	fontSize int
	faces    map[faceKey]font.Face
}

// NewCanvas returns a white canvas using the given font as current font.
func NewCanvas(w, h int, id FontID, size int) *Canvas {
	c := &Canvas{
		W: w, H: h,
		Black:  image.NewGray(image.Rect(0, 0, w, h)),
		Colour: image.NewGray(image.Rect(0, 0, w, h)),
		fontID: id, fontSize: size,
		faces: map[faceKey]font.Face{},
	}
	for i := range c.Black.Pix {
		c.Black.Pix[i] = 255
		c.Colour.Pix[i] = 255
	}
	return c
}

// SetFont changes the current font.
func (c *Canvas) SetFont(id FontID, size int) { c.fontID, c.fontSize = id, size }

// Font returns the current font.
func (c *Canvas) Font() (FontID, int) { return c.fontID, c.fontSize }

// face returns a cached face; faces are per canvas because they are not goroutine-safe.
func (c *Canvas) face(id FontID, size int) font.Face {
	k := faceKey{id, size}
	if f, ok := c.faces[k]; ok {
		return f
	}
	f, err := opentype.NewFace(loadFont(id), &opentype.FaceOptions{Size: float64(size), DPI: 72, Hinting: font.HintingNone})
	if err != nil {
		panic(err) // only fails for invalid options
	}
	c.faces[k] = f
	return f
}

// layers returns the images an ink draws on and the colour to draw with.
func (c *Canvas) layers(ink Ink) ([]*image.Gray, color.Gray) {
	switch ink {
	case InkColour:
		return []*image.Gray{c.Colour}, color.Gray{Y: 0}
	case InkWhite:
		return []*image.Gray{c.Black, c.Colour}, color.Gray{Y: 255}
	default:
		return []*image.Gray{c.Black}, color.Gray{Y: 0}
	}
}

func (c *Canvas) set(x, y int, ink Ink) {
	if x < 0 || y < 0 || x >= c.W || y >= c.H {
		return
	}
	imgs, col := c.layers(ink)
	for _, img := range imgs {
		img.SetGray(x, y, col)
	}
}

// InkAt reports whether the pixel carries ink on the black and/or colour layer.
func (c *Canvas) InkAt(x, y int) (black, colour bool) {
	return c.Black.GrayAt(x, y).Y <= InkThreshold, c.Colour.GrayAt(x, y).Y <= InkThreshold
}

// Paste copies the ink (not the white background) of src to (x, y).
func (c *Canvas) Paste(src *Canvas, x, y int) {
	for sy := 0; sy < src.H; sy++ {
		for sx := 0; sx < src.W; sx++ {
			dx, dy := x+sx, y+sy
			if dx < 0 || dy < 0 || dx >= c.W || dy >= c.H {
				continue
			}
			if v := src.Black.GrayAt(sx, sy).Y; v <= InkThreshold {
				c.Black.SetGray(dx, dy, color.Gray{Y: v})
			}
			if v := src.Colour.GrayAt(sx, sy).Y; v <= InkThreshold {
				c.Colour.SetGray(dx, dy, color.Gray{Y: v})
			}
		}
	}
}
