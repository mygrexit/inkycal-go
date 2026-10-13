package textfile

import (
	"testing"

	"inkycal/internal/config"
	"inkycal/internal/modules/modulestest"
	"inkycal/internal/render"
)

func TestShapes(t *testing.T) {
	modulestest.Shapes(t, Meta, func(c config.Common) (*render.Canvas, error) {
		w, h := c.InnerSize()
		cv := render.NewCanvas(w, h, render.FontRegular, int(c.FontSize))
		draw(cv, parse(sample), int(c.FontSize), c.Language)
		return cv, nil
	})
}
