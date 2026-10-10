package weather

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"inkycal/internal/config"
	"inkycal/internal/fetch"
	"inkycal/internal/modules"
	"inkycal/internal/render"
)

func mustCommon(t *testing.T, raw string) config.Common {
	t.Helper()
	c, err := config.ParseCommon(json.RawMessage(raw))
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func inkIn(c *render.Canvas, x0, y0, x1, y1 int) (black, colour bool) {
	for y := y0; y < y1; y++ {
		for x := x0; x < x1; x++ {
			b, col := c.InkAt(x, y)
			black = black || b
			colour = colour || col
		}
	}
	return black, colour
}

func TestNewValidation(t *testing.T) {
	common := mustCommon(t, `{"size":[480,100],"padding_x":10,"padding_y":10,"fontsize":14,"language":"en"}`)
	if _, err := New(common, json.RawMessage(`{"location":"Berlin,DE"}`)); err == nil {
		t.Error("missing api_key accepted")
	}
	if _, err := New(common, json.RawMessage(`{"api_key":"k"}`)); err == nil {
		t.Error("missing location accepted")
	}
	tiny := mustCommon(t, `{"size":[60,30],"padding_x":0,"padding_y":0,"fontsize":14}`)
	if _, err := New(tiny, json.RawMessage(`{"api_key":"k","location":"Berlin"}`)); err == nil {
		t.Error("60x30 accepted, would divide by zero in the grid")
	}

	m, err := New(common, json.RawMessage(`{"api_key":"k","location":4893171,"units":"imperial","use_beaufort":"True","hour_format":"12","forecast_interval":"daily","round_windspeed":true}`))
	if err != nil {
		t.Fatal(err)
	}
	mod := m.(*Module)
	if mod.location != "4893171" {
		t.Errorf("numeric location = %q", mod.location)
	}
	want := settings{Fahrenheit: true, WindUnit: windBeaufort, RoundWind: true, Daily: true, Hour12: true, Lang: "en"}
	if mod.set != want {
		t.Errorf("settings = %+v, want %+v", mod.set, want)
	}
}

func TestGenerate(t *testing.T) {
	loc := berlin(t)
	now := time.Date(2026, 10, 10, 9, 0, 0, 0, loc)
	forecast := fmt.Sprintf(`{"list":[{"dt":%d,"main":{"temp":3.0},"weather":[{"icon":"03d"}]},{"dt":%d,"main":{"temp":4.3},"weather":[{"icon":"10d"}]}]}`,
		now.Add(3*time.Hour).Unix(), now.Add(6*time.Hour).Unix())
	serveOWM(t, currentFixture("04d", "7.9", now.Add(-2*time.Hour), now.Add(9*time.Hour)), forecast)

	common := mustCommon(t, `{"size":[480,100],"padding_x":10,"padding_y":10,"fontsize":14,"language":"en"}`)
	m, err := New(common, json.RawMessage(`{"api_key":"k","location":4893171,"forecast_interval":"hourly","units":"metric","hour_format":24,"use_beaufort":false,"round_temperature":true,"round_windspeed":true}`))
	if err != nil {
		t.Fatal(err)
	}
	env := &modules.Env{Now: now, Loc: loc, Fetch: fetch.New("")}

	c, err := m.Generate(context.Background(), env)
	if err != nil {
		t.Fatal(err)
	}
	if c.W != 460 || c.H != 80 {
		t.Fatalf("canvas = %dx%d, want 460x80", c.W, c.H)
	}
	g := computeGrid(460, 80)
	if _, colour := inkIn(c, g.Cols[0], 0, g.Cols[0]+g.ColW, c.H); !colour {
		t.Error("current weather icon not drawn in colour")
	}
	if black, _ := inkIn(c, g.Cols[3], 0, g.Cols[3]+g.ColW, c.H); !black {
		t.Error("first forecast column has no black ink")
	}
	if env.Stale {
		t.Error("Stale set although nothing came from cache")
	}
}
