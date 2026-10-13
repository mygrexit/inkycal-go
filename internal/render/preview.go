package render

import (
	"image"
	"image/color"
)

// Preview renders the canvas as laid out (portrait for stacked dashboards) in the
// panel's colours, for the builder: white paper, black ink, red colour ink.
func (c *Canvas) Preview() *image.RGBA {
	return c.PreviewInks("bwr", color.RGBA{255, 0, 0, 255})
}

// PreviewInks renders the canvas the way a panel with the given ink set shows it:
// "bw" prints the colour layer black, "gray16" (IT8951) shows paper 0xF0 and the colour
// layer as 0x40 grey, "bwr" uses colour for the colour layer (red or yellow).
func (c *Canvas) PreviewInks(inks string, colour color.RGBA) *image.RGBA {
	paper, ink := color.RGBA{255, 255, 255, 255}, color.RGBA{0, 0, 0, 255}
	switch inks {
	case "bw":
		colour = ink
	case "gray16":
		paper, colour = color.RGBA{240, 240, 240, 255}, color.RGBA{64, 64, 64, 255}
	}
	img := image.NewRGBA(image.Rect(0, 0, c.W, c.H))
	for y := 0; y < c.H; y++ {
		for x := 0; x < c.W; x++ {
			col := paper
			switch black, col2 := c.InkAt(x, y); {
			case col2:
				col = colour
			case black:
				col = ink
			}
			img.SetRGBA(x, y, col)
		}
	}
	return img
}
