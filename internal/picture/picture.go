// Package picture turns photos and drawings into panel ink: load from a file or URL,
// flatten transparency, rotate to the module's orientation, scale, and map to the
// panel's colours with Floyd–Steinberg dithering (Python: utils/inky_image.py).
package picture

import (
	"bytes"
	"context"
	"fmt"
	"image"
	"image/draw"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"os"
	"strings"

	_ "golang.org/x/image/bmp"
	xdraw "golang.org/x/image/draw"
	_ "golang.org/x/image/webp"

	"inkycal/internal/fetch"
	"inkycal/internal/render"
)

// Load reads an image from an http(s) URL or a local path.
func Load(ctx context.Context, f *fetch.Client, src string) (img image.Image, stale bool, err error) {
	var data []byte
	if strings.HasPrefix(src, "http://") || strings.HasPrefix(src, "https://") {
		res, err := f.Get(ctx, src, nil)
		if err != nil {
			return nil, false, err
		}
		data, stale = res.Body, res.Stale
	} else if data, err = os.ReadFile(src); err != nil {
		return nil, false, fmt.Errorf("picture: %w", err)
	}
	img, err = Decode(data)
	return img, stale, err
}

// Decode understands PNG, JPEG, GIF, BMP and WebP.
func Decode(data []byte) (image.Image, error) {
	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("picture: not an image: %w", err)
	}
	return img, nil
}

// Flatten draws img onto white, so transparent areas become paper.
func Flatten(img image.Image) *image.RGBA {
	b := img.Bounds()
	out := image.NewRGBA(image.Rect(0, 0, b.Dx(), b.Dy()))
	draw.Draw(out, out.Bounds(), image.White, image.Point{}, draw.Src)
	draw.Draw(out, out.Bounds(), img, b.Min, draw.Over)
	return out
}

// Autoflip turns img by 90° when its shape does not match orientation
// ("horizontal" wants landscape, "vertical" portrait). Anything else leaves it as is.
func Autoflip(img image.Image, orientation string) image.Image {
	if NeedsFlip(img.Bounds().Dx(), img.Bounds().Dy(), orientation) {
		return Rotate90(img)
	}
	return img
}

// NeedsFlip reports whether Autoflip would turn a w×h image.
func NeedsFlip(w, h int, orientation string) bool {
	return (orientation == "horizontal" && h > w) || (orientation == "vertical" && w > h)
}

// Rotate90 turns img counter-clockwise by 90°, like PIL's rotate(90, expand=True).
func Rotate90(img image.Image) *image.RGBA {
	b := img.Bounds()
	out := image.NewRGBA(image.Rect(0, 0, b.Dy(), b.Dx()))
	for y := 0; y < b.Dy(); y++ {
		for x := 0; x < b.Dx(); x++ {
			out.Set(y, b.Dx()-1-x, img.At(b.Min.X+x, b.Min.Y+y))
		}
	}
	return out
}

// Fit scales img to the largest size that fits in maxW×maxH, keeping its aspect ratio.
func Fit(img image.Image, maxW, maxH int) *image.RGBA {
	b := img.Bounds()
	w, h := maxW, b.Dy()*maxW/max(1, b.Dx())
	if h > maxH {
		w, h = b.Dx()*maxH/max(1, b.Dy()), maxH
	}
	out := image.NewRGBA(image.Rect(0, 0, max(1, w), max(1, h)))
	xdraw.CatmullRom.Scale(out, out.Bounds(), img, b, xdraw.Src, nil)
	return out
}

// Palette is the set of colours an image is mapped to.
type Palette string

const (
	BW  Palette = "bw"
	BWR Palette = "bwr"
	BWY Palette = "bwy"
)

// ParsePalette accepts the Python tokens. Grey levels can't be shown by a 1-bit
// panel, so "16gray" dithers to black and white.
func ParsePalette(s string) (Palette, error) {
	switch s {
	case "", "bw", "16gray", "gray16":
		return BW, nil
	case "bwr":
		return BWR, nil
	case "bwy":
		return BWY, nil
	}
	return "", fmt.Errorf("palette %q: want bw, bwr, bwy or 16gray", s)
}

type entry struct {
	r, g, b float64
	ink     render.Ink
	paper   bool
}

func (p Palette) entries() []entry {
	es := []entry{{255, 255, 255, 0, true}, {0, 0, 0, render.InkBlack, false}}
	switch p {
	case BWR:
		es = append(es, entry{255, 0, 0, render.InkColour, false})
	case BWY:
		es = append(es, entry{255, 255, 0, render.InkColour, false})
	}
	return es
}

// Draw maps img to the palette and sets it on c with its top-left corner at (x, y).
// With dither, quantisation errors are spread to the neighbours (Floyd–Steinberg).
func Draw(c *render.Canvas, img image.Image, x, y int, p Palette, dither bool) {
	b := img.Bounds()
	w, h := b.Dx(), b.Dy()
	pal := p.entries()
	// Two rows of RGB error, swapped per line.
	cur, next := make([][3]float64, w+2), make([][3]float64, w+2)
	for py := 0; py < h; py++ {
		for px := 0; px < w; px++ {
			r16, g16, b16, _ := img.At(b.Min.X+px, b.Min.Y+py).RGBA()
			v := [3]float64{float64(r16>>8) + cur[px+1][0], float64(g16>>8) + cur[px+1][1], float64(b16>>8) + cur[px+1][2]}
			best, bestD := pal[0], -1.0
			for _, e := range pal {
				dr, dg, db := v[0]-e.r, v[1]-e.g, v[2]-e.b
				if d := dr*dr + dg*dg + db*db; bestD < 0 || d < bestD {
					best, bestD = e, d
				}
			}
			if !best.paper {
				c.Set(x+px, y+py, best.ink)
			}
			if !dither {
				continue
			}
			er := [3]float64{v[0] - best.r, v[1] - best.g, v[2] - best.b}
			for i := 0; i < 3; i++ {
				cur[px+2][i] += er[i] * 7 / 16
				next[px][i] += er[i] * 3 / 16
				next[px+1][i] += er[i] * 5 / 16
				next[px+2][i] += er[i] * 1 / 16
			}
		}
		cur, next = next, cur
		for i := range next {
			next[i] = [3]float64{}
		}
	}
}
