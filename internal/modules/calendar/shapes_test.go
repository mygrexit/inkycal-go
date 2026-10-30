package calendar

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"inkycal/internal/config"
	"inkycal/internal/fetch"
	"inkycal/internal/modules"
	"inkycal/internal/modules/modulestest"
	"inkycal/internal/render"
)

func TestShapes(t *testing.T) {
	loc := berlin(t)
	url, _ := serveICS(t)
	modulestest.Shapes(t, Meta, func(c config.Common) (*render.Canvas, error) {
		m, err := Def.New(c, json.RawMessage(`{"show_events":true,"weekday_prefix":true,"date_format":"D. MMM","ical_urls":"`+url+`"}`))
		if err != nil {
			return nil, err
		}
		return m.Generate(context.Background(), &modules.Env{Now: time.Date(2026, 10, 10, 9, 0, 0, 0, loc), Loc: loc, Fetch: fetch.New("")})
	})
}
