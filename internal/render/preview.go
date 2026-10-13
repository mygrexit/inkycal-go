package render

import (
	"image"
	"image/color"
)

// Preview renders the canvas as laid out (portrait for stacked dashboards) in the
// panel's colours, for the builder: white paper, black ink, red colour ink.
func (c *Canvas) Preview() *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, c.W, c.H))
	for y := 0; y < c.H; y++ {
		for x := 0; x < c.W; x++ {
			col := color.RGBA{255, 255, 255, 255}
			switch black, colour := c.InkAt(x, y); {
			case colour:
				col = color.RGBA{255, 0, 0, 255}
			case black:
				col = color.RGBA{0, 0, 0, 255}
			}
			img.SetRGBA(x, y, col)
		}
	}
	return img
}
