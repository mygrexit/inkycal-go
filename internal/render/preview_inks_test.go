package render

import (
	"image/color"
	"testing"
)

func TestPreviewInksFollowThePanel(t *testing.T) {
	c := NewCanvas(3, 1, FontDefault, 14)
	c.Set(1, 0, InkBlack)
	c.Set(2, 0, InkColour)
	red, yellow := color.RGBA{196, 22, 28, 255}, color.RGBA{230, 190, 0, 255}
	white := color.RGBA{255, 255, 255, 255}
	cases := []struct {
		inks        string
		colour      color.RGBA
		want, paper color.RGBA
	}{
		{"bwr", red, red, white},
		{"bwr", yellow, yellow, white},
		{"bw", red, color.RGBA{0, 0, 0, 255}, white}, // the panel prints the colour layer black
		// IT8951 (it8951.greyLevels): white 0xF0, colour 0x40
		{"gray16", red, color.RGBA{64, 64, 64, 255}, color.RGBA{240, 240, 240, 255}},
	}
	for _, k := range cases {
		img := c.PreviewInks(k.inks, k.colour)
		if got := img.RGBAAt(2, 0); got != k.want {
			t.Errorf("%s: colour pixel %v, want %v", k.inks, got, k.want)
		}
		if got := img.RGBAAt(0, 0); got != k.paper {
			t.Errorf("%s: paper %v", k.inks, got)
		}
	}
}
