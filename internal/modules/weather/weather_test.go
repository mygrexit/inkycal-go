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
	common := mustCommon(t, `{"size":[984,290],"padding_x":40,"padding_y":20,"fontsize":14,"language":"en"}`)
	if _, err := New(common, json.RawMessage(`{"location":"Berlin,DE"}`)); err == nil {
		t.Error("missing api_key accepted")
	}
	if _, err := New(common, json.RawMessage(`{"api_key":"k"}`)); err == nil {
		t.Error("missing location accepted")
	}
	tiny := mustCommon(t, `{"size":[60,30],"padding_x":0,"padding_y":0,"fontsize":14}`)
	if _, err := New(tiny, json.RawMessage(`{"api_key":"k","location":"Berlin"}`)); err == nil {
		t.Error("60x30 accepted, too small for the today band")
	}

	m, err := New(common, json.RawMessage(`{"api_key":"k","location":4893171,"units":"imperial","use_beaufort":"True","hour_format":"12","forecast_interval":"daily","round_windspeed":true}`))
	if err != nil {
		t.Fatal(err)
	}
	mod := m.(*Module)
	if mod.location != "4893171" {
		t.Errorf("numeric location = %q", mod.location)
	}
	want := settings{Fahrenheit: true, Hour12: true, Lang: "en"}
	if mod.set != want {
		t.Errorf("settings = %+v, want %+v", mod.set, want)
	}
}

func TestGenerate(t *testing.T) {
	loc := berlin(t)
	now := time.Date(2026, 10, 13, 6, 5, 0, 0, loc)
	forecast := fmt.Sprintf(`{"list":[{"dt":%d,"main":{"temp":3.0},"weather":[{"icon":"03d"}],"wind":{"speed":2}},{"dt":%d,"main":{"temp":4.3},"weather":[{"icon":"10d"}],"wind":{"speed":12}}]}`,
		now.Add(3*time.Hour).Unix(), now.Add(6*time.Hour).Unix())
	serveOWM(t, currentFixture("04d", "7.9", now.Add(2*time.Hour), now.Add(12*time.Hour)), forecast)

	common := mustCommon(t, `{"size":[984,290],"padding_x":40,"padding_y":20,"fontsize":20,"language":"de"}`)
	m, err := New(common, json.RawMessage(`{"api_key":"k","location":4893171,"units":"metric","hour_format":24}`))
	if err != nil {
		t.Fatal(err)
	}
	env := &modules.Env{Now: now, Loc: loc, Fetch: fetch.New("")}

	c, err := m.Generate(context.Background(), env)
	if err != nil {
		t.Fatal(err)
	}
	if c.W != 904 || c.H != 250 {
		t.Fatalf("canvas = %dx%d, want 904x250", c.W, c.H)
	}
	if _, colour := inkIn(c, 0, 0, 280, 125); !colour {
		t.Error("date numeral not drawn in colour")
	}
	if black, _ := inkIn(c, 300, 0, 424, 124); !black {
		t.Error("day icon missing")
	}
	// 12 m/s later today is strong wind: the gauge turns red.
	if _, colour := inkIn(c, 734, 0, c.W, 45); !colour {
		t.Error("strong-wind gauge not red")
	}
	if env.Stale {
		t.Error("Stale set although nothing came from cache")
	}
}

// Narrow or short bands scale the whole layout instead of overlapping or being rejected.
func TestDrawDayFitsOtherShapes(t *testing.T) {
	loc := berlin(t)
	now := time.Date(2026, 10, 13, 6, 5, 0, 0, loc)
	v := buildDay(settings{Lang: "de"}, current{Icon: "04d", Temp: 8, Sunrise: now, Sunset: now}, nil, now)

	narrow := render.NewCanvas(400, 200, render.FontDefault, 14) // width-limited: 400*250/904 ≈ 110 px tall
	drawDay(narrow, v, "de")
	if b, c := inkIn(narrow, 0, 0, 400, 40); b || c {
		t.Error("narrow band not centred vertically (ink in the top 40 px)")
	}
	if b, c := inkIn(narrow, 0, 160, 400, 200); b || c {
		t.Error("narrow band not centred vertically (ink in the bottom 40 px)")
	}

	common := mustCommon(t, `{"size":[480,110],"padding_x":10,"padding_y":10,"fontsize":14,"language":"de"}`)
	if _, err := New(common, json.RawMessage(`{"api_key":"k","location":"Berlin"}`)); err != nil {
		t.Errorf("7.5\" sized band rejected: %v", err)
	}
}
