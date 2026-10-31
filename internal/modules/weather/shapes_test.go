package weather

import (
	"context"
	"encoding/json"
	"strconv"
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
	list := `{"list":[` +
		point(now.Add(3*time.Hour), 3.0, "03d", 2) + `,` + point(now.Add(6*time.Hour), 4.3, "10d", 12) + `,` +
		point(now.Add(9*time.Hour), 5.1, "04d", 4) + `,` + point(now.Add(12*time.Hour), 2.2, "01n", 3) + `]}`
	fc := serveOWM(t, currentFixture("04d", "7.9", now.Add(2*time.Hour), now.Add(12*time.Hour)), list)
	modulestest.Shapes(t, Meta, func(c config.Common) (*render.Canvas, error) {
		m, err := Def.New(c, json.RawMessage(`{"api_key":"k","location":"Berlin","units":"metric"}`))
		if err != nil {
			return nil, err
		}
		return m.Generate(context.Background(), &modules.Env{Now: now, Loc: loc, Fetch: fc})
	})
}

func point(at time.Time, temp float64, icon string, wind float64) string {
	return `{"dt":` + strconv.FormatInt(at.Unix(), 10) + `,"main":{"temp":` + strconv.FormatFloat(temp, 'f', 1, 64) + `},"weather":[{"icon":"` + icon + `"}],"wind":{"speed":` + strconv.FormatFloat(wind, 'f', 1, 64) + `}}`
}
