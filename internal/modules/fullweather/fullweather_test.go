package fullweather

import (
	"context"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"io"
	"math"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"
	"time"
	_ "time/tzdata"

	"inkycal/internal/config"
	"inkycal/internal/fetch"
	"inkycal/internal/fetch/fetchtest"
	"inkycal/internal/forecast"
	"inkycal/internal/modules"
	"inkycal/internal/modules/modulestest"
	"inkycal/internal/render"
)

func berlin(t *testing.T) *time.Location {
	t.Helper()
	loc, err := time.LoadLocation("Europe/Berlin")
	if err != nil {
		t.Fatal(err)
	}
	return loc
}

const fullCommon = `{"size":[984,1304],"padding_x":40,"padding_y":20,"fontsize":22,"language":"en"}`

func TestNewUpstreamKeys(t *testing.T) {
	common := modulestest.Common(t, fullCommon)
	for name, raw := range map[string]string{
		"no api_key":   `{"latitude":51.5,"longitude":-0.13}`,
		"no latitude":  `{"api_key":"k","longitude":-0.13}`,
		"no longitude": `{"api_key":"k","latitude":51.5}`,
		"bad latitude": `{"api_key":"k","latitude":"north","longitude":1}`,
		"out of range": `{"api_key":"k","latitude":95,"longitude":1}`,
	} {
		if _, err := Def.New(common, json.RawMessage(raw)); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
	tiny := modulestest.Common(t, `{"size":[300,60],"padding_x":0,"padding_y":0}`)
	if _, err := Def.New(tiny, json.RawMessage(`{"api_key":"k","latitude":1,"longitude":1}`)); err == nil {
		t.Error("300x60 accepted")
	}

	// Every upstream key, with the loose types the web UI writes.
	m, err := Def.New(common, json.RawMessage(`{"api_key":"k","latitude":"51.51","longitude":"-0.13","api_version":"3.0",
		"orientation":"vertical","temp_unit":"fahrenheit","wind_unit":"km_hour","wind_gusts":"False","keep_history":true,
		"min_max_annotations":"True","locale":"de_DE.UTF-8","font":"Roboto","chart_title":"Temperatur und Niederschlag",
		"weekly_title":"Tageswerte","icon_outline":true}`))
	if err != nil {
		t.Fatal(err)
	}
	mod := m.(*Module)
	if want := source(t, 51.51, -0.13, "de", true); !reflect.DeepEqual(mod.src, want) {
		t.Errorf("source %+v, want %+v", mod.src, want)
	}
	want := settings{Fahrenheit: true, Wind: windKMH, Gusts: false, MinMax: true, Lang: "de"}
	if mod.set != want {
		t.Errorf("settings = %+v, want %+v", mod.set, want)
	}

	// Defaults: celsius, m/s, gusts on, 2.5 API, language from the common keys.
	m, err = Def.New(common, json.RawMessage(`{"api_key":"k","latitude":51.5,"longitude":-0.13,"api_version":2.5}`))
	if err != nil {
		t.Fatal(err)
	}
	mod = m.(*Module)
	if want := (settings{Wind: windMPS, Gusts: true, Lang: "en"}); mod.set != want || !reflect.DeepEqual(mod.src, source(t, 51.5, -0.13, "en", false)) {
		t.Errorf("defaults = %+v source %+v", mod.set, mod.src)
	}
	if m, _ := Def.New(common, json.RawMessage(`{"api_key":"k","latitude":1,"longitude":1,"api_version":3}`)); !reflect.DeepEqual(m.(*Module).src, source(t, 1, 1, "en", true)) {
		t.Error("numeric api_version 3 not recognised")
	}
}

// source is the OpenWeatherMap source an upstream Fullweather tile resolves to.
func source(t *testing.T, lat, lon float64, lang string, oneCall bool) forecast.Source {
	t.Helper()
	la, lo := config.FlexFloat(lat), config.FlexFloat(lon)
	src, err := forecast.Settings{Provider: "openweathermap", APIKey: "k", Latitude: &la, Longitude: &lo}.Source(lang, forecast.Want{Days: 5, UV: true, OneCall: oneCall})
	if err != nil {
		t.Fatal(err)
	}
	return src
}

func TestWindUnits(t *testing.T) {
	cases := []struct {
		unit string
		want string
	}{
		{"meters_sec", "4–9 m/s"}, {"km_hour", "14–32 km/h"}, {"miles_hour", "9–20 mph"},
		{"knots", "8–17 kn"}, {"beaufort", "3–5 Bft"}, {"furlongs", "4–9 m/s"},
	}
	for _, c := range cases {
		s := settings{Wind: parseWindUnit(c.unit), Gusts: true}
		if got := s.windText(4, 9); got != c.want {
			t.Errorf("%s: %q, want %q", c.unit, got, c.want)
		}
	}
	if got := (settings{Wind: windMPS}).windText(4, 9); got != "4 m/s" {
		t.Errorf("without gusts: %q", got)
	}
	if got := (settings{Wind: windMPS, Gusts: true}).windText(4, 4.2); got != "4 m/s" {
		t.Errorf("gust equal to mean: %q", got)
	}
}

func TestFormatting(t *testing.T) {
	for in, want := range map[float64]string{0: "0 mm", 0.44: "0,4 mm", 9.94: "9,9 mm", 12.6: "13 mm"} {
		if got := mm(in, "de"); got != want {
			t.Errorf("mm(%v) = %q, want %q", in, got, want)
		}
	}
	if got := mm(1.25, "en"); got != "1.2 mm" && got != "1.3 mm" {
		t.Errorf("mm en = %q", got)
	}
	if capitalise("leichter Regen") != "Leichter Regen" || capitalise("") != "" {
		t.Error("capitalise")
	}
}

func TestTempStepAndPrecipScale(t *testing.T) {
	if got := tempStep(6.2, 14.3, false, 6); got != 3 {
		t.Errorf("celsius step = %v, want upstream 3", got)
	}
	if got := tempStep(40, 60, true, 6); got != 5 {
		t.Errorf("fahrenheit step = %v, want upstream 5", got)
	}
	if got := tempStep(-5, 30, false, 5); got != 10 {
		t.Errorf("wide range step = %v, want 10", got)
	}
	if got := tempStep(10.2, 11.4, false, 6); got > 1 {
		t.Errorf("narrow range step = %v, want 1", got)
	}
	for in, want := range map[float64]float64{0: 5, 3: 5, 7: 10, 18: 20, 140: 200} {
		if got := precipScale(in); got != want {
			t.Errorf("precipScale(%v) = %v, want %v", in, got, want)
		}
	}
}

// sampleForecast is five days of plausible October weather in 3-hour slots.
func sampleForecast(now time.Time) []forecast.Point {
	start := now.UTC().Truncate(3 * time.Hour).Add(3 * time.Hour)
	var pts []forecast.Point
	for i := range 40 {
		t := start.Add(time.Duration(i*3) * time.Hour).In(now.Location())
		hr := float64(t.Hour()) + float64(t.Minute())/60
		day := float64(i) / 8
		temp := 9 + 1.2*day - 4.5*math.Cos((hr-3)/24*2*math.Pi) // coldest around 3 h
		icon, rain := "04d", 0.0
		switch {
		case i >= 2 && i <= 4:
			icon, rain = "10d", []float64{0.4, 1.8, 0.9}[i-2]
		case i >= 9 && i <= 11:
			icon = "03d"
		case i >= 14 && i <= 16:
			icon = "01d"
		case i == 22 || i == 23:
			icon, rain = "09d", []float64{3.2, 6.1}[i-22]
		case i >= 28:
			icon = "02d"
		}
		if hr < 6 || hr > 20 {
			icon = icon[:2] + "n"
		}
		pts = append(pts, forecast.Point{Time: t, Temp: temp, Icon: icon, Wind: 3.5 + day, Gust: 7 + day, Precip: rain})
	}
	return pts
}

func TestBuildView(t *testing.T) {
	loc := berlin(t)
	now := time.Date(2026, 10, 13, 6, 5, 0, 0, loc)
	cur := forecast.Current{Icon: "10d", Description: "leichter Regen", Temp: 7.6, FeelsLike: 5.2, Humidity: 88,
		WindMean: 3, WindMPS: 6, Sunrise: now.Add(2 * time.Hour), Sunset: now.Add(12 * time.Hour)}
	pts := sampleForecast(now)
	uv := 6.4
	v := buildView(settings{Wind: windMPS, Gusts: true, Lang: "de"}, cur, pts, &uv, now)

	if v.Temp != "8°" || v.FeelsLike != "gefühlt 5°" || v.Description != "Leichter Regen" || v.Humidity != "88 %" {
		t.Errorf("current = %q %q %q %q", v.Temp, v.FeelsLike, v.Description, v.Humidity)
	}
	if v.Wind != "4–7 m/s" || v.WindStrong || v.Precip != "0 mm" {
		t.Errorf("next 3 h = %q strong=%v precip=%q (from the first slot after now)", v.Wind, v.WindStrong, v.Precip)
	}
	if v.UV != "UV 6,4" || !v.UVHigh {
		t.Errorf("uv = %q high=%v", v.UV, v.UVHigh)
	}
	if v.Sunrise != "8:05" || v.Sunset != "18:05" {
		t.Errorf("sun = %s / %s", v.Sunrise, v.Sunset)
	}
	for _, s := range v.Samples {
		if !s.Time.After(now) || s.Time.After(now.Add(chartHours*time.Hour)) {
			t.Errorf("sample %v outside the chart window", s.Time)
		}
	}
	if n := len(v.Samples); n < 20 || n > 22 {
		t.Errorf("%d samples, want about 22 three-hour slots", n)
	}
	if len(v.Days) != maxDays {
		t.Fatalf("%d day columns, want %d", len(v.Days), maxDays)
	}
	today := v.Days[0]
	if !today.Today || today.Label != "Heute" || today.Icon != "10d" || today.Precip != "3,1 mm" {
		t.Errorf("today = %+v", today)
	}
	if d := v.Days[1]; d.Today || d.Label != "Mi" || d.Date != "14. Okt" || d.Icon != "03d" {
		t.Errorf("tomorrow = %+v", d)
	}

	// Fahrenheit, English, no UV.
	v = buildView(settings{Fahrenheit: true, Wind: windBft, Lang: "en"}, cur, pts, nil, now)
	if v.Temp != "46°" || v.FeelsLike != "feels like 41°" || v.UV != "" || v.Days[0].Label != "Today" || v.Days[1].Date != "Oct 14" {
		t.Errorf("en/F view = %q %q %q %+v", v.Temp, v.FeelsLike, v.UV, v.Days[1])
	}
	if v.Samples[0].Temp < 32 {
		t.Errorf("chart samples not in °F: %v", v.Samples[0].Temp)
	}
}

// serveOWM answers /weather, /forecast and the One Call endpoint like OpenWeatherMap.
func serveOWM(t *testing.T, now time.Time, oneCallHits *atomic.Int32) *fetch.Client {
	t.Helper()
	var list []string
	for _, p := range sampleForecast(now) {
		list = append(list, fmt.Sprintf(`{"dt":%d,"main":{"temp":%.2f,"temp_min":0,"temp_max":0,"pressure":1012,"humidity":80},`+
			`"weather":[{"icon":%q,"description":"x"}],"wind":{"speed":%.1f,"gust":%.1f},"rain":{"3h":%.1f},"pop":0.4}`,
			p.Time.Unix(), p.Temp, p.Icon, p.Wind, p.Gust, p.Precip))
	}
	forecastBody := `{"list":[` + strings.Join(list, ",") + `]}`
	current := fmt.Sprintf(`{"weather":[{"icon":"10d","description":"leichter Regen"}],"main":{"temp":7.6,"feels_like":5.2,"temp_min":6,"temp_max":9,"humidity":88},`+
		`"wind":{"speed":3.2,"gust":6.4},"sys":{"sunrise":%d,"sunset":%d}}`,
		time.Date(2026, 10, 13, 7, 54, 0, 0, now.Location()).Unix(), time.Date(2026, 10, 13, 18, 48, 0, 0, now.Location()).Unix())

	return fetchtest.Client(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		if q.Get("lat") != "50.94" || q.Get("lon") != "6.96" || q.Get("appid") != "k" {
			t.Errorf("query = %v", q)
		}
		switch r.URL.Path {
		case "/data/2.5/weather":
			io.WriteString(w, current)
		case "/data/2.5/forecast":
			io.WriteString(w, forecastBody)
		case "/data/3.0/onecall":
			oneCallHits.Add(1)
			io.WriteString(w, `{"current":{"uvi":2.3}}`)
		default:
			http.NotFound(w, r)
		}
	}))
}

func generate(t *testing.T, fc *fetch.Client, common, raw string, now time.Time) *render.Canvas {
	t.Helper()
	m, err := Def.New(modulestest.Common(t, common), json.RawMessage(raw))
	if err != nil {
		t.Fatal(err)
	}
	env := &modules.Env{Now: now, Loc: now.Location(), Fetch: fc}
	c, err := m.Generate(context.Background(), env)
	if err != nil {
		t.Fatal(err)
	}
	if env.Stale {
		t.Error("Stale set although nothing came from cache")
	}
	return c
}

func TestGenerate(t *testing.T) {
	loc := berlin(t)
	now := time.Date(2026, 10, 13, 6, 5, 0, 0, loc)
	var hits atomic.Int32
	fc := serveOWM(t, now, &hits)

	c := generate(t, fc, fullCommon, `{"api_key":"k","latitude":50.94,"longitude":6.96,"api_version":"3.0","locale":"de_DE.UTF-8"}`, now)
	if c.W != 904 || c.H != 1264 {
		t.Fatalf("canvas = %dx%d, want 904x1264", c.W, c.H)
	}
	if hits.Load() != 1 {
		t.Errorf("One Call fetched %d times, want 1 with api_version 3.0", hits.Load())
	}
	if _, colour := modulestest.InkIn(c, 0, 0, 280, 125); !colour {
		t.Error("date numeral not drawn in colour")
	}
	if black, _ := modulestest.InkIn(c, 300, 0, 424, 124); !black {
		t.Error("current icon missing")
	}
	// The chart's red dot sits at its left edge, between the header and the day row.
	if _, colour := modulestest.InkIn(c, 0, 280, 200, c.H-300); !colour {
		t.Error("no red observation marker in the chart")
	}
	if black, _ := modulestest.InkIn(c, 0, c.H-200, c.W, c.H); !black {
		t.Error("day columns missing at the bottom")
	}

	hits.Store(0)
	half := generate(t, fc, `{"size":[984,640],"padding_x":40,"padding_y":20,"fontsize":22,"language":"en"}`,
		`{"api_key":"k","latitude":"50.94","longitude":"6.96"}`, now)
	if half.W != 904 || half.H != 600 {
		t.Fatalf("half canvas = %dx%d", half.W, half.H)
	}
	if hits.Load() != 0 {
		t.Error("One Call fetched without api_version 3.0")
	}
}

// Every supported shape draws something sensible and never panics.
func TestDrawShapes(t *testing.T) {
	loc := berlin(t)
	now := time.Date(2026, 10, 13, 21, 40, 0, 0, loc) // late: today has few slots left
	cur := forecast.Current{Icon: "01n", Description: "clear sky", Temp: 4, FeelsLike: 1, Humidity: 70, WindMean: 15, WindMPS: 22}
	v := buildView(settings{Wind: windMPS, Gusts: true, Lang: "en", MinMax: true}, cur, sampleForecast(now), nil, now)
	for _, sz := range [][2]int{{904, 1264}, {904, 600}, {904, 400}, {904, 140}, {400, 600}, {640, 384}} {
		c := render.NewCanvas(sz[0], sz[1], render.FontDefault, 14)
		draw(c, v)
		if b, _ := modulestest.InkIn(c, 0, 0, c.W, c.H); !b {
			t.Errorf("%v: nothing drawn", sz)
		}
	}
}

// TestPreview writes PNGs for a visual check when INKYCAL_PREVIEW_DIR is set.
func TestPreview(t *testing.T) {
	dir := os.Getenv("INKYCAL_PREVIEW_DIR")
	if dir == "" {
		t.Skip("INKYCAL_PREVIEW_DIR not set")
	}
	loc := berlin(t)
	now := time.Date(2026, 10, 13, 6, 5, 0, 0, loc)
	var hits atomic.Int32
	fc := serveOWM(t, now, &hits)
	shots := []struct{ name, common, raw string }{
		{"full", fullCommon, `{"api_key":"k","latitude":50.94,"longitude":6.96,"api_version":"3.0","locale":"de_DE.UTF-8","min_max_annotations":true}`},
		{"half", `{"size":[984,640],"padding_x":40,"padding_y":20,"fontsize":22,"language":"de"}`, `{"api_key":"k","latitude":50.94,"longitude":6.96}`},
		{"band", `{"size":[984,180],"padding_x":40,"padding_y":20,"fontsize":22,"language":"de"}`, `{"api_key":"k","latitude":50.94,"longitude":6.96}`},
		{"narrow", `{"size":[440,640],"padding_x":20,"padding_y":20,"fontsize":16,"language":"en"}`, `{"api_key":"k","latitude":50.94,"longitude":6.96,"hour_format":12,"min_max_annotations":true}`},
		{"mid", `{"size":[984,440],"padding_x":40,"padding_y":20,"fontsize":22,"language":"en"}`, `{"api_key":"k","latitude":50.94,"longitude":6.96,"temp_unit":"fahrenheit","wind_unit":"beaufort"}`},
	}
	for _, s := range shots {
		c := generate(t, fc, s.common, s.raw, now)
		writePNG(t, filepath.Join(dir, "fullweather-"+s.name+".png"), c)
	}
}

func writePNG(t *testing.T, path string, c *render.Canvas) {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, c.W, c.H))
	for y := 0; y < c.H; y++ {
		for x := 0; x < c.W; x++ {
			col := color.RGBA{255, 255, 255, 255}
			if b, r := c.InkAt(x, y); r {
				col = color.RGBA{230, 0, 0, 255}
			} else if b {
				col = color.RGBA{0, 0, 0, 255}
			}
			img.Set(x, y, col)
		}
	}
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if err := png.Encode(f, img); err != nil {
		t.Fatal(err)
	}
}
