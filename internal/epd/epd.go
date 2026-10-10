// Package epd drives e-paper panels (and a PNG stand-in for dry runs).
package epd

import (
	"fmt"
	"image/png"
	"os"
	"path/filepath"

	"inkycal/internal/render"
)

// Display shows frames on a panel.
type Display interface {
	Size() (w, h int)          // native panel size (landscape)
	Show(f render.Frame) error // init, transfer, refresh, deep sleep
	Clear(cycles int) error    // calibration: black, colour, white per cycle
}

// Models lists supported panels and their native size.
var Models = map[string][2]int{
	"epd_7_in_5_v2_colour": {800, 480},
}

// Bus is the hardware interface a driver needs (SPI + 4 GPIOs).
type Bus interface {
	Open() error              // power the panel, open SPI
	Close() error             // close SPI, pull RST/DC/PWR low (panel draws no power)
	SetReset(high bool) error // RST pin
	Command(c byte) error     // DC low, write one byte
	Data(p ...byte) error     // DC high, write bytes
	BusyLevel() (int, error)  // raw level of the BUSY pin
}

type pngDisplay struct {
	path string
	w, h int
}

// NewPNG returns a Display that writes each frame as PNG (as the panel shows it, landscape).
func NewPNG(path string, w, h int) Display { return &pngDisplay{path: path, w: w, h: h} }

func (p *pngDisplay) Size() (int, int) { return p.w, p.h }

func (p *pngDisplay) Clear(int) error { return nil }

func (p *pngDisplay) Show(f render.Frame) error {
	if f.W != p.w || f.H != p.h {
		return fmt.Errorf("frame %dx%d, display %dx%d", f.W, f.H, p.w, p.h)
	}
	tmp, err := os.CreateTemp(filepath.Dir(p.path), ".preview-*.png")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if err := png.Encode(tmp, f.Preview()); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), p.path)
}
