package modulestest

import (
	"fmt"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"inkycal/internal/config"
	"inkycal/internal/modules"
	"inkycal/internal/render"
)

// Shape is an inner tile size a module must lay out in.
type Shape struct {
	Name string
	W, H int
}

// ShapesFor lists the tile shapes a builder user can make for a module on the 12.48"
// page (984 wide): its minimum, a tall column, a wide band, a square and a small tile.
func ShapesFor(m modules.Meta) []Shape {
	sq := max(m.MinW, m.MinH, 300)
	return []Shape{
		{"min", m.MinW, m.MinH},
		{"tall", max(m.MinW, 200), max(m.MinH, 640)},
		{"wide", 904, max(m.MinH, 130)},
		{"square", sq, sq},
		{"small", max(m.MinW, 160), max(m.MinH, 140)},
	}
}

// Shapes renders a module at every shape (gen builds and runs it for the given common
// keys) and fails on errors, panics or a wrong canvas size. With INKYCAL_SHAPES_DIR set it
// writes a PNG per shape for a look at the layouts.
func Shapes(t *testing.T, m modules.Meta, gen func(c config.Common) (*render.Canvas, error)) {
	t.Helper()
	dir := os.Getenv("INKYCAL_SHAPES_DIR")
	for _, s := range ShapesFor(m) {
		c := config.Common{Size: [2]int{s.W + 20, s.H + 20}, PaddingX: 10, PaddingY: 10,
			FontSize: config.FlexInt(modules.AutoFontSize(s.W, s.H)), Language: "de"}
		cv, err := func() (cv *render.Canvas, err error) {
			defer func() {
				if r := recover(); r != nil {
					err = fmt.Errorf("panic: %v", r)
				}
			}()
			return gen(c)
		}()
		if err != nil {
			t.Errorf("%s %s %dx%d: %v", m.Name, s.Name, s.W, s.H, err)
			continue
		}
		if cv.W != s.W || cv.H != s.H {
			t.Errorf("%s %s: canvas %dx%d, want %dx%d", m.Name, s.Name, cv.W, cv.H, s.W, s.H)
		}
		if dir != "" {
			f, err := os.Create(filepath.Join(dir, fmt.Sprintf("%s-%s.png", m.Name, s.Name)))
			if err == nil {
				png.Encode(f, cv.Preview())
				f.Close()
			}
		}
	}
}
