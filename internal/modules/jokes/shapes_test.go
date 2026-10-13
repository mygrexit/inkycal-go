package jokes

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
		setup, punch := split("What do you call a fish wearing a bowtie? Sofishticated.")
		draw(cv, setup, punch, int(c.FontSize))
		return cv, nil
	})
}
