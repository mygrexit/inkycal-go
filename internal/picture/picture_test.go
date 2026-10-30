package picture

import (
	"bytes"
	"context"
	"image"
	"image/color"
	"image/png"
	"net/http"
	"net/http/httptest"
	"testing"

	"inkycal/internal/fetch"
	"inkycal/internal/render"
)

func solid(w, h int, c color.Color) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := range h {
		for x := range w {
			img.Set(x, y, c)
		}
	}
	return img
}

func count(c *render.Canvas) (black, colour int) {
	for y := 0; y < c.H; y++ {
		for x := 0; x < c.W; x++ {
			b, col := c.InkAt(x, y)
			if b {
				black++
			}
			if col {
				colour++
			}
		}
	}
	return
}

func TestDrawMapsToPalette(t *testing.T) {
	for _, tc := range []struct {
		name          string
		img           color.Color
		pal           Palette
		black, colour int
	}{
		{"white is paper", color.White, BWR, 0, 0},
		{"black", color.Black, BWR, 100, 0},
		{"red on bwr", color.RGBA{230, 20, 20, 255}, BWR, 0, 100},
		{"red on bw becomes ink or paper only", color.RGBA{230, 20, 20, 255}, BW, -1, 0},
		{"yellow on bwy", color.RGBA{250, 240, 10, 255}, BWY, 0, 100},
	} {
		c := render.NewCanvas(10, 10, render.FontDefault, 12)
		Draw(c, solid(10, 10, tc.img), 0, 0, tc.pal, false)
		b, col := count(c)
		if (tc.black >= 0 && b != tc.black) || col != tc.colour {
			t.Errorf("%s: black=%d colour=%d", tc.name, b, col)
		}
	}
}

func TestDitherKeepsMidGreyAtHalfInk(t *testing.T) {
	c := render.NewCanvas(40, 40, render.FontDefault, 12)
	Draw(c, solid(40, 40, color.Gray{Y: 128}), 0, 0, BW, true)
	b, _ := count(c)
	if b < 700 || b > 900 { // ~50 % of 1600
		t.Errorf("mid grey dithered to %d of 1600 black pixels", b)
	}
	c = render.NewCanvas(40, 40, render.FontDefault, 12)
	Draw(c, solid(40, 40, color.Gray{Y: 128}), 0, 0, BW, false)
	if b, _ := count(c); b != 1600 && b != 0 {
		t.Errorf("without dither a flat grey must map to one colour, got %d", b)
	}
}

func TestFitKeepsAspect(t *testing.T) {
	for _, tc := range []struct{ w, h, mw, mh, ww, wh int }{
		{400, 200, 100, 100, 100, 50},
		{200, 400, 100, 100, 50, 100},
		{10, 10, 300, 200, 200, 200}, // upscales like Python
	} {
		got := Fit(solid(tc.w, tc.h, color.Black), tc.mw, tc.mh).Bounds()
		if got.Dx() != tc.ww || got.Dy() != tc.wh {
			t.Errorf("%dx%d in %dx%d → %v", tc.w, tc.h, tc.mw, tc.mh, got)
		}
	}
}

func TestAutoflipAndRotate(t *testing.T) {
	tall := solid(10, 20, color.Black)
	tall.Set(0, 0, color.White) // top-left marker
	if b := Autoflip(tall, "horizontal").Bounds(); b.Dx() != 20 || b.Dy() != 10 {
		t.Errorf("horizontal: %v", b)
	}
	if b := Autoflip(tall, "vertical").Bounds(); b.Dx() != 10 {
		t.Errorf("vertical must keep a portrait image: %v", b)
	}
	r := Rotate90(tall)
	// Counter-clockwise: top-left goes to bottom-left.
	if r.At(0, 9) != (color.RGBA{255, 255, 255, 255}) {
		t.Errorf("marker not at bottom-left after rotation")
	}
}

func TestFlattenMakesTransparencyWhite(t *testing.T) {
	img := image.NewNRGBA(image.Rect(0, 0, 2, 1))
	img.Set(1, 0, color.NRGBA{0, 0, 0, 255})
	f := Flatten(img)
	if r, g, b, _ := f.At(0, 0).RGBA(); r>>8 != 255 || g>>8 != 255 || b>>8 != 255 {
		t.Errorf("transparent pixel is not white")
	}
}

func TestLoadFromURLAndFile(t *testing.T) {
	var buf bytes.Buffer
	png.Encode(&buf, solid(3, 2, color.Black))
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.Write(buf.Bytes()) }))
	defer srv.Close()
	img, _, err := Load(context.Background(), fetch.New(""), srv.URL+"/x.png")
	if err != nil || img.Bounds().Dx() != 3 {
		t.Fatalf("url: %v %v", img, err)
	}
	if _, _, err := Load(context.Background(), fetch.New(""), "/does/not/exist.png"); err == nil {
		t.Error("missing file must fail")
	}
	if _, err := Decode([]byte("nope")); err == nil {
		t.Error("garbage must fail")
	}
	if _, err := ParsePalette("rainbow"); err == nil {
		t.Error("unknown palette must fail")
	}
}
