package greek

import (
	"context"
	"testing"
	"time"

	"inkycal/internal/config"
	"inkycal/internal/modules"
	"inkycal/internal/modules/modulestest"
	"inkycal/internal/render"
)

func TestShapes(t *testing.T) {
	modulestest.Shapes(t, Meta, func(c config.Common) (*render.Canvas, error) {
		m, err := New(c, nil)
		if err != nil {
			return nil, err
		}
		return m.Generate(context.Background(), &modules.Env{Now: time.Date(2026, 10, 13, 6, 0, 0, 0, time.UTC)})
	})
}
