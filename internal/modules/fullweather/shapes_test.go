package fullweather

import (
	"context"
	"encoding/json"
	"sync/atomic"
	"testing"
	"time"

	"inkycal/internal/config"
	"inkycal/internal/modules"
	"inkycal/internal/modules/modulestest"
	"inkycal/internal/render"
)

func TestShapes(t *testing.T) {
	loc := berlin(t)
	now := time.Date(2026, 10, 13, 6, 5, 0, 0, loc)
	var hits atomic.Int32
	fc := serveOWM(t, now, &hits)
	modulestest.Shapes(t, Meta, func(c config.Common) (*render.Canvas, error) {
		m, err := Def.New(c, json.RawMessage(`{"api_key":"k","latitude":50.94,"longitude":6.96}`))
		if err != nil {
			return nil, err
		}
		return m.Generate(context.Background(), &modules.Env{Now: now, Loc: loc, Fetch: fc})
	})
}
