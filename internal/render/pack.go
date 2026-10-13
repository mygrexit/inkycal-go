package render

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"image"
	"image/color"
)

// Frame is the display-native bit-plane representation of a canvas.
type Frame struct {
	W, H  int    // native panel size (landscape)
	Black []byte // 1 bit/px, MSB first, 1 = white, 0 = black
	Red   []byte // 1 bit/px, MSB first, 1 = red, 0 = no red
}

// Pack converts the canvas to panel bit planes. A portrait canvas (panelH x panelW) is rotated
// 90° counter-clockwise like Python's getbuffer; orientation 180 flips the canvas first.
func Pack(c *Canvas, panelW, panelH, orientation int) (Frame, error) {
	var rotate bool
	switch {
	case c.W == panelW && c.H == panelH:
	case c.W == panelH && c.H == panelW:
		rotate = true
	default:
		return Frame{}, fmt.Errorf("canvas %dx%d does not fit panel %dx%d", c.W, c.H, panelW, panelH)
	}
	stride := (panelW + 7) / 8
	f := Frame{W: panelW, H: panelH, Black: make([]byte, stride*panelH), Red: make([]byte, stride*panelH)}
	for i := range f.Black {
		f.Black[i] = 0xFF
	}
	for dy := 0; dy < panelH; dy++ {
		for dx := 0; dx < panelW; dx++ {
			sx, sy := dx, dy
			if rotate {
				sx, sy = c.W-1-dy, dx
			}
			if orientation == 180 {
				sx, sy = c.W-1-sx, c.H-1-sy
			}
			black, colour := c.InkAt(sx, sy)
			idx, bit := dy*stride+dx/8, byte(0x80>>(dx%8))
			if black {
				f.Black[idx] &^= bit
			}
			if colour {
				f.Red[idx] |= bit
			}
		}
	}
	return f, nil
}

// Hash identifies the frame content; equal hashes mean no refresh is needed.
func (f Frame) Hash() string {
	h := sha256.New()
	h.Write(f.Black)
	h.Write(f.Red)
	return hex.EncodeToString(h.Sum(nil))
}

// Preview renders the frame as the panel shows it (landscape): white, black and red.
func (f Frame) Preview() *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, f.W, f.H))
	stride := (f.W + 7) / 8
	for y := 0; y < f.H; y++ {
		for x := 0; x < f.W; x++ {
			idx, bit := y*stride+x/8, byte(0x80>>(x%8))
			col := color.RGBA{255, 255, 255, 255}
			if f.Black[idx]&bit == 0 {
				col = color.RGBA{0, 0, 0, 255}
			}
			if f.Red[idx]&bit != 0 {
				col = color.RGBA{255, 0, 0, 255}
			}
			img.SetRGBA(x, y, col)
		}
	}
	return img
}
