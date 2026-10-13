package epd

import (
	"fmt"
	"image"
	"image/color"
	"image/png"
	"log/slog"
	"os"
	"path/filepath"

	"inkycal/internal/render"
)

// Upstream's image_file and image_file_12_in_48 "drivers" (drivers/image_file*.py) show
// nothing: getbuffer saves the image as getbuffer_image.png and display saves it again as
// display_image.png, both in the working directory. The image is main.py's merged
// black/white canvas with the colour band pasted in pure red (models without "colour"),
// in portrait as Inkycal renders it (getbuffer does not rotate here).

func init() {
	Models["image_file"] = [2]int{800, 480}
	Models["image_file_12_in_48"] = [2]int{1304, 984}
}

// ImageFile writes frames like upstream's image_file drivers.
type ImageFile struct {
	dir  string
	w, h int
}

// NewImageFile writes into dir ("" = the working directory, like upstream).
func NewImageFile(model, dir string) (*ImageFile, error) {
	size, ok := Models[model]
	if !ok || (model != "image_file" && model != "image_file_12_in_48") {
		return nil, fmt.Errorf("epd: %q is not an image_file model", model)
	}
	return &ImageFile{dir: dir, w: size[0], h: size[1]}, nil
}

func (d *ImageFile) Size() (int, int) { return d.w, d.h }

// Show writes getbuffer_image.png and display_image.png (the same portrait image).
func (d *ImageFile) Show(f render.Frame) error {
	if f.W != d.w || f.H != d.h {
		return fmt.Errorf("epd: frame %dx%d, image_file is %dx%d", f.W, f.H, d.w, d.h)
	}
	img := portraitImage(f)
	for _, name := range []string{"getbuffer_image.png", "display_image.png"} {
		if err := writePNGAtomic(filepath.Join(d.dir, name), img); err != nil {
			return err
		}
	}
	abs, _ := filepath.Abs(filepath.Join(d.dir, "display_image.png"))
	slog.Info("image written", "path", abs)
	return nil
}

// Clear mirrors calibrate() for a model without colour: black, white per cycle.
func (d *ImageFile) Clear(cycles int) error {
	stride := (d.w + 7) / 8
	black := render.Frame{W: d.w, H: d.h, Black: make([]byte, stride*d.h), Red: make([]byte, stride*d.h)}
	white := render.Frame{W: d.w, H: d.h, Black: make([]byte, stride*d.h), Red: black.Red}
	for i := range white.Black {
		white.Black[i] = 0xFF
	}
	for i := 0; i < cycles; i++ {
		for _, f := range []render.Frame{black, white} {
			if err := d.Show(f); err != nil {
				return err
			}
		}
	}
	return nil
}

// portraitImage returns the canvas Inkycal rendered (H x W): render.Pack maps canvas
// pixel (sx, sy) = (H-1-dy, dx) to frame pixel (dx, dy), so canvas (x, y) is frame
// (y, H-1-x). Red wins over black, as Images.merge pastes the red layer on top.
func portraitImage(f render.Frame) *image.NRGBA {
	img := image.NewNRGBA(image.Rect(0, 0, f.H, f.W))
	stride := (f.W + 7) / 8
	for y := 0; y < f.W; y++ {
		for x := 0; x < f.H; x++ {
			dx, dy := y, f.H-1-x
			idx, bit := dy*stride+dx/8, byte(0x80>>(dx%8))
			c := color.NRGBA{255, 255, 255, 255}
			if f.Black[idx]&bit == 0 {
				c = color.NRGBA{0, 0, 0, 255}
			}
			if f.Red[idx]&bit != 0 {
				c = color.NRGBA{255, 0, 0, 255}
			}
			img.SetNRGBA(x, y, c)
		}
	}
	return img
}

func writePNGAtomic(path string, img image.Image) error {
	tmp, err := os.CreateTemp(filepath.Dir(path), ".image-*.png")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if err := png.Encode(tmp, img); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), path)
}
