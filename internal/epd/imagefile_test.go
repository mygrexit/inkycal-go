package epd

import (
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"inkycal/internal/render"
)

func TestImageFileWritesPortraitCanvas(t *testing.T) {
	for _, tc := range []struct {
		model string
		w, h  int
	}{{"image_file", 800, 480}, {"image_file_12_in_48", 1304, 984}} {
		t.Run(tc.model, func(t *testing.T) {
			if Models[tc.model] != [2]int{tc.w, tc.h} {
				t.Fatalf("size %v", Models[tc.model])
			}
			dir := t.TempDir()
			d, err := NewImageFile(tc.model, dir)
			if err != nil {
				t.Fatal(err)
			}
			c := render.NewCanvas(tc.h, tc.w, render.FontDefault, 12) // portrait, as Inkycal renders
			c.Set(10, 20, render.InkBlack)
			c.Set(30, 40, render.InkColour)
			f, err := render.Pack(c, tc.w, tc.h, 0)
			if err != nil {
				t.Fatal(err)
			}
			if err := d.Show(f); err != nil {
				t.Fatal(err)
			}
			for _, name := range []string{"getbuffer_image.png", "display_image.png"} {
				r, err := os.Open(filepath.Join(dir, name))
				if err != nil {
					t.Fatal(err)
				}
				img, err := png.Decode(r)
				r.Close()
				if err != nil {
					t.Fatal(err)
				}
				if b := img.Bounds(); b.Dx() != tc.h || b.Dy() != tc.w {
					t.Fatalf("%s: %v, want portrait %dx%d", name, b, tc.h, tc.w)
				}
				for _, p := range []struct {
					x, y int
					want color.NRGBA
				}{{10, 20, color.NRGBA{0, 0, 0, 255}}, {30, 40, color.NRGBA{255, 0, 0, 255}}, {0, 0, color.NRGBA{255, 255, 255, 255}}} {
					if got := color.NRGBAModel.Convert(img.At(p.x, p.y)).(color.NRGBA); got != p.want {
						t.Errorf("%s (%d,%d) = %v, want %v", name, p.x, p.y, got, p.want)
					}
				}
			}
			if err := d.Show(patternFrame(10, 10)); err == nil {
				t.Error("wrong frame size accepted")
			}
		})
	}
}

func TestImageFileClearEndsWhite(t *testing.T) {
	dir := t.TempDir()
	d, _ := NewImageFile("image_file", dir)
	if err := d.Clear(1); err != nil {
		t.Fatal(err)
	}
	r, err := os.Open(filepath.Join(dir, "display_image.png"))
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	img, err := png.Decode(r)
	if err != nil {
		t.Fatal(err)
	}
	if got := color.NRGBAModel.Convert(img.At(5, 5)).(color.NRGBA); got != (color.NRGBA{255, 255, 255, 255}) {
		t.Errorf("after clear: %v", got)
	}
	if _, err := NewImageFile("9_in_7", dir); err == nil {
		t.Error("non image_file model accepted")
	}
}
