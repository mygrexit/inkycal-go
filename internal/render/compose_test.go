package render

import (
	"image/color"
	"testing"
)

func dot(w, h int) *Canvas {
	c := NewCanvas(w, h, FontDefault, 14)
	c.FillCircle(w/2, h/2, 3, InkColour)
	return c
}

func TestAssembleGridCentresAndDivides(t *testing.T) {
	parts := []Placed{
		{Img: dot(80, 180), X: 0, Y: 0, W: 100, H: 200},    // left column
		{Img: dot(80, 80), X: 100, Y: 0, W: 100, H: 100},   // top right
		{Img: dot(80, 80), X: 100, Y: 100, W: 100, H: 100}, // bottom right
	}
	out := AssembleGrid(200, 240, parts, Layout{Dividers: true, Info: "info", InfoHeight: 40})
	for _, p := range [][2]int{{50, 100}, {150, 50}, {150, 150}} {
		if _, col := out.InkAt(p[0], p[1]); !col {
			t.Errorf("no dot at %v", p)
		}
	}
	// Vertical divider left of the right modules, horizontal one above the bottom right.
	if b, _ := out.InkAt(100, 50); !b {
		t.Error("no vertical divider at x=100")
	}
	if b, _ := out.InkAt(150, 100); !b {
		t.Error("no horizontal divider at y=100")
	}
	// Modules touching the page edge get no divider there.
	if b, _ := out.InkAt(50, 0); b {
		t.Error("divider on the top page edge")
	}
	// The info line is in the bottom 40 px.
	inked := false
	for y := 200; y < 240; y++ {
		for x := range 200 {
			b, _ := out.InkAt(x, y)
			inked = inked || b
		}
	}
	if !inked {
		t.Error("info line missing")
	}
}

func TestCanvasPreviewColours(t *testing.T) {
	c := NewCanvas(3, 1, FontDefault, 14)
	c.Set(1, 0, InkBlack)
	c.Set(2, 0, InkColour)
	img := c.Preview()
	want := []color.RGBA{{255, 255, 255, 255}, {0, 0, 0, 255}, {255, 0, 0, 255}}
	for x, w := range want {
		if got := img.RGBAAt(x, 0); got != w {
			t.Errorf("x=%d: %v, want %v", x, got, w)
		}
	}
	if img.Bounds().Dx() != 3 || img.Bounds().Dy() != 1 {
		t.Errorf("bounds %v", img.Bounds())
	}
}
