# Inkycal-Go Plan 2: Module Weather, Calendar, Todoist, Feeds – Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Die vier restlichen Module des Nutzers (Weather, Calendar, Todoist, Feeds) als Go-Pakete portieren, in der Registry eintragen und mit einem End-to-End-Test über alle fünf Module absichern.

**Architecture:** Jedes Modul ist ein eigenes Paket unter `internal/modules/<name>` mit drei Schichten: (1) Datenabruf über `fetch.Client` (Basis-URL als unexportierte Paketvariable, in Tests auf `httptest` umgebogen), (2) eine **reine** Funktion, die ein View-Model bzw. Zeilenmodell aus den Daten berechnet (voll unit-getestet), (3) eine dünne Zeichenfunktion auf `render.Canvas`. Netzwerkabrufe innerhalb eines Moduls laufen parallel.

**Tech Stack:** Go 1.26, `golang.org/x/image` (über `internal/render`), `github.com/mmcdole/gofeed v1.5.0` (nur Feeds), Plan-1-Pakete `config`, `fetch`, `ical`, `modules`, `render`, `timefmt`, `app`, `epd`.

**Spec:** `docs/superpowers/specs/2026-10-10-inkycal-go-design.md` (insbesondere §5 – verbindliche Schnittstellen)

**Voraussetzung:** Plan 1 (`docs/superpowers/plans/2026-10-10-inkycal-go-plan1-core.md`) ist vollständig umgesetzt; `go test ./...` ist grün.

## Global Constraints

- Go 1.26 (`go 1.26` in go.mod; `golang.org/x/image v0.47.0` verlangt ≥ 1.26), Modulpfad `inkycal`
- Lizenz GPL-3.0-only (abgeleitet von Inkycal)
- `CGO_ENABLED=0`; Ziele `linux/arm64` und `linux/arm` mit `GOARM=7`
- Erlaubte Abhängigkeiten: `golang.org/x/image v0.47.0`, `golang.org/x/sys v0.49.0`, `github.com/warthog618/go-gpiocdev v0.9.2`, `github.com/apognu/gocal v0.9.1`, `github.com/mmcdole/gofeed v1.5.0` (nur Feeds)
- Tests nur mit Standardbibliothek; kein Netzwerk in Tests (`httptest`)
- Logging über `log/slog`
- Ausgabe muss nicht pixelgleich zu Python sein – gleiches Layout, gleiche Inhalte
- Jedes Modul: `func New(c config.Common, raw json.RawMessage) (modules.Module, error)`; `Generate` liefert Canvas der Größe `c.InnerSize()`, erzeugt mit `render.NewCanvas(w, h, render.FontDefault, int(c.FontSize))`; bei Cache-Daten `env.Stale = true`
- Config-Schlüssel exakt wie Python: Weather `api_key`, `location`, `round_temperature`, `round_windspeed`, `forecast_interval`, `units`, `hour_format`, `use_beaufort`; Calendar `week_starts_on`, `show_events`, `ical_urls`, `ical_files`, `date_format`, `time_format`; Todoist `api_key`, `project_filter`, `show_priority`; Feeds `feed_urls`, `shuffle_feeds`

## Review Focus

1. **Unbekannter OWM-Icon-Code** (OWM führt neue Codes ein, z.B. `"77z"`): Python wirft `KeyError` und das ganze Modul fehlt. Erwartet: Fallback-Glyphe „n/a“, Modul wird gezeichnet. → Test `TestBuildViewUnknownIconAndNoForecast` (Task 2).
2. **Zu kleine Modulgröße in settings.json** (z.B. Weather 60×30 ohne Padding): darf nicht mit Division durch 0 panicken, sondern muss beim Laden einen klaren Fehler liefern. → `TestNewValidation` (Task 2), `TestNewRejectsTinySize` (Task 4).
3. **Ganztägige und monatsübergreifende Termine im Monatsraster**: Python markiert den Folgetag eines Ganztagstermins und Tage aus Vormonat/Folgemonat mit gleicher Tageszahl. Erwartet: Ende exklusiv, nur Tage des angezeigten Monats. → `TestEventDays` (Task 3).
4. **Todoist-Fälligkeiten mit Uhrzeit** (`"2026-10-10T15:00:00Z"`, `"2026-10-10T15:00:00"`) und kaputte Datumswerte: Erwartet: Datumsteil zählt („TODAY“), kaputte Aufgabe wird übersprungen statt Modulabbruch. → `TestBuildRows` (Task 6).
5. **Ein defekter Feed unter mehreren / HTML-Entities im Text**: Erwartet: übrige Feeds werden angezeigt, `&amp;` erscheint als `&`, Zeilenumbrüche beenden den Eintrag. → `TestFetchEntriesSkipsBrokenFeed`, `TestEntry` (Task 7).

---

## Dateistruktur

| Datei | Verantwortung |
|---|---|
| `internal/modules/weather/convert.go` | Glyphen-Tabellen, Einheiten, Beaufort, Mondphase |
| `internal/modules/weather/owm.go` | OWM-2.5-Abruf `/weather`, `/forecast` |
| `internal/modules/weather/view.go` | View-Model (formatierte Strings), Tagesaggregation |
| `internal/modules/weather/layout.go` | 7-Spalten-Raster + Zeichnen |
| `internal/modules/weather/weather.go` | Config, `New`, `Generate` |
| `internal/modules/calendar/grid.go` | Monatsraster, Wochenbeginn, Layout |
| `internal/modules/calendar/events.go` | Event-Tage, Event-Zeilen |
| `internal/modules/calendar/calendar.go` | Config, `New`, `Generate`, Zeichnen |
| `internal/modules/todoist/api.go` | Todoist-API v1 mit Cursor-Pagination |
| `internal/modules/todoist/view.go` | Zeilenmodell (Gruppierung, Filter, Priorität, Fälligkeit) |
| `internal/modules/todoist/todoist.go` | Config, `New`, `Generate`, Zeichnen |
| `internal/modules/feeds/feeds.go` | Abruf, Parsen, Bereinigen, Zeilenauswahl, Modul |
| `internal/app/registry.go` | Registry um vier Module erweitern |
| `internal/app/modules_e2e_test.go` | `RunCycle` mit allen fünf Modulen gegen `httptest` |

---

### Task 1: Weather – Umrechnungen, Glyphen und OWM-Abruf

**Files:**
- Create: `internal/modules/weather/convert.go`
- Create: `internal/modules/weather/owm.go`
- Test: `internal/modules/weather/convert_test.go`
- Test: `internal/modules/weather/owm_test.go`

**Interfaces:**
- Consumes: `fetch.Client.GetJSON(ctx context.Context, url string, header http.Header, v any) (stale bool, err error)` (Plan 1)
- Produces (paketintern, von Task 2 genutzt):
  - Konstanten `glyphNA, glyphThermometer, glyphHumidity, glyphWind, glyphSunrise, glyphSunset string`; `windMetersSec, windMilesHour, windBeaufort string`
  - `func iconGlyph(code string) string`
  - `func toFahrenheit(c float64) float64`
  - `func beaufort(mps float64) int`
  - `func convertWind(mps float64, unit string) float64`
  - `func moonPhaseGlyph(t time.Time) string`
  - `var owmBaseURL string`
  - `type current struct { Icon string; Temp, MaxTemp float64; Humidity int; WindMPS float64; Sunrise, Sunset time.Time }`
  - `type forecastPoint struct { Time time.Time; Temp float64; Icon string }`
  - `func queryURL(endpoint, apiKey, location, lang string) string`
  - `func fetchCurrent(ctx context.Context, fc *fetch.Client, apiKey, location, lang string, loc *time.Location) (current, bool, error)`
  - `func fetchForecast(ctx context.Context, fc *fetch.Client, apiKey, location, lang string, loc *time.Location) ([]forecastPoint, bool, error)`

- [ ] **Step 1: Failing Tests für Umrechnungen schreiben**

`internal/modules/weather/convert_test.go`:

```go
package weather

import (
	"math"
	"testing"
	"time"
)

func TestBeaufort(t *testing.T) {
	cases := []struct {
		mps  float64
		want int
	}{{0, 0}, {0.29, 0}, {0.3, 1}, {1.5, 1}, {5.5, 4}, {7.9, 4}, {32.6, 11}, {40, 12}}
	for _, c := range cases {
		if got := beaufort(c.mps); got != c.want {
			t.Errorf("beaufort(%v) = %d, want %d", c.mps, got, c.want)
		}
	}
}

func TestConvertWind(t *testing.T) {
	if got := convertWind(10, windMetersSec); got != 10 {
		t.Errorf("meters_sec = %v", got)
	}
	if got := convertWind(10, windMilesHour); math.Abs(got-22.3694) > 1e-9 {
		t.Errorf("miles_hour = %v", got)
	}
	if got := convertWind(10, windBeaufort); got != 5 {
		t.Errorf("beaufort = %v", got)
	}
}

func TestToFahrenheit(t *testing.T) {
	if toFahrenheit(0) != 32 || toFahrenheit(100) != 212 {
		t.Errorf("toFahrenheit wrong: %v %v", toFahrenheit(0), toFahrenheit(100))
	}
}

func TestMoonPhaseGlyph(t *testing.T) {
	// 2001-01-01: lunation position 0.204 -> index 2.
	if got := moonPhaseGlyph(time.Date(2001, 1, 1, 0, 0, 0, 0, time.UTC)); got != "" {
		t.Errorf("2001-01-01: got %q", got)
	}
	// 2001-01-11: 0.2044 + 10*0.03386 = 0.543 -> index 4.
	if got := moonPhaseGlyph(time.Date(2001, 1, 11, 0, 0, 0, 0, time.UTC)); got != "" {
		t.Errorf("2001-01-11: got %q", got)
	}
}

func TestIconGlyph(t *testing.T) {
	if got := iconGlyph("01d"); got != "" {
		t.Errorf("01d = %q", got)
	}
	if got := iconGlyph("13n"); got != "" {
		t.Errorf("13n = %q", got)
	}
	if got := iconGlyph("77z"); got != glyphNA {
		t.Errorf("unknown code = %q, want glyphNA", got)
	}
}
```

- [ ] **Step 2: Test laufen lassen – muss fehlschlagen**

Run: `go test ./internal/modules/weather/ -run 'TestBeaufort|TestConvertWind|TestToFahrenheit|TestMoonPhaseGlyph|TestIconGlyph' -v`
Expected: FAIL – `undefined: beaufort` (Build-Fehler)

- [ ] **Step 3: `convert.go` implementieren**

```go
package weather

import (
	"math"
	"time"
)

// Glyphs of the Weather Icons font (render.FontWeather).
const (
	glyphNA          = ""
	glyphThermometer = ""
	glyphHumidity    = ""
	glyphWind        = ""
	glyphSunrise     = ""
	glyphSunset      = ""
)

// Wind units, named like the Python OpenWeatherMap wrapper.
const (
	windMetersSec = "meters_sec"
	windMilesHour = "miles_hour"
	windBeaufort  = "beaufort"
)

// weatherIcons maps OpenWeatherMap icon codes to Weather Icons glyphs (table from inkycal_weather.py).
var weatherIcons = map[string]string{
	"01d": "", "02d": "", "03d": "", "04d": "", "09d": "",
	"10d": "", "11d": "", "13d": "", "50d": "",
	"01n": "", "02n": "", "03n": "", "04n": "", "09n": "",
	"10n": "", "11n": "", "13n": "", "50n": "",
}

var moonGlyphs = [8]string{"", "", "", "", "", "", "", ""}

// iconGlyph returns the glyph for an OWM icon code; unknown codes get "n/a"
// instead of failing the whole module like Python's KeyError.
func iconGlyph(code string) string {
	if g, ok := weatherIcons[code]; ok {
		return g
	}
	return glyphNA
}

func toFahrenheit(c float64) float64 { return c*9.0/5.0 + 32.0 }

// beaufort converts m/s to the Beaufort scale (thresholds from the Python wrapper).
func beaufort(mps float64) int {
	thresholds := []float64{0.3, 1.6, 3.4, 5.5, 8.0, 10.8, 13.9, 17.2, 20.8, 24.5, 28.5, 32.7}
	for i, t := range thresholds {
		if mps < t {
			return i
		}
	}
	return 12
}

func convertWind(mps float64, unit string) float64 {
	switch unit {
	case windBeaufort:
		return float64(beaufort(mps))
	case windMilesHour:
		return mps * 2.23694
	default:
		return mps
	}
}

// moonPhaseGlyph approximates the moon phase exactly like Python's get_moon_phase.
func moonPhaseGlyph(t time.Time) string {
	epoch := time.Date(2001, 1, 1, 0, 0, 0, 0, time.UTC)
	days := t.Sub(epoch).Hours() / 24
	lunations := 0.20439731 + days*0.03386319269
	position := lunations - math.Floor(lunations)
	return moonGlyphs[int(math.Floor(position*8+0.5))&7]
}
```

- [ ] **Step 4: Tests laufen lassen – müssen bestehen**

Run: `go test ./internal/modules/weather/ -run 'TestBeaufort|TestConvertWind|TestToFahrenheit|TestMoonPhaseGlyph|TestIconGlyph' -v`
Expected: PASS (5 Tests)

- [ ] **Step 5: Failing Tests für den OWM-Abruf schreiben**

`internal/modules/weather/owm_test.go`:

```go
package weather

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
	_ "time/tzdata"

	"inkycal/internal/fetch"
)

func berlin(t *testing.T) *time.Location {
	t.Helper()
	loc, err := time.LoadLocation("Europe/Berlin")
	if err != nil {
		t.Fatal(err)
	}
	return loc
}

// serveOWM points owmBaseURL at a test server answering /weather and /forecast.
func serveOWM(t *testing.T, currentBody, forecastBody string) {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Query().Get("units"); got != "metric" {
			t.Errorf("units = %q, want metric", got)
		}
		switch r.URL.Path {
		case "/weather":
			io.WriteString(w, currentBody)
		case "/forecast":
			io.WriteString(w, forecastBody)
		default:
			http.NotFound(w, r)
		}
	}))
	old := owmBaseURL
	owmBaseURL = srv.URL
	t.Cleanup(func() {
		owmBaseURL = old
		srv.Close()
	})
}

// currentFixture builds a /weather body; gust "" omits the gust field.
func currentFixture(icon, gust string, sunrise, sunset time.Time) string {
	wind := `{"speed":3.1}`
	if gust != "" {
		wind = `{"speed":3.1,"gust":` + gust + `}`
	}
	return fmt.Sprintf(`{"weather":[{"icon":%q}],"main":{"temp":-1.6,"temp_max":2.4,"humidity":81},"wind":%s,"sys":{"sunrise":%d,"sunset":%d}}`,
		icon, wind, sunrise.Unix(), sunset.Unix())
}

func TestQueryURL(t *testing.T) {
	u, err := url.Parse(queryURL("weather", "key", "4893171", "de"))
	if err != nil {
		t.Fatal(err)
	}
	q := u.Query()
	if q.Get("id") != "4893171" || q.Has("q") {
		t.Errorf("numeric location: query = %v", q)
	}
	if q.Get("appid") != "key" || q.Get("units") != "metric" || q.Get("lang") != "de" {
		t.Errorf("query = %v", q)
	}
	if !strings.HasSuffix(u.Path, "/weather") {
		t.Errorf("path = %q", u.Path)
	}
	u2, err := url.Parse(queryURL("forecast", "key", " Berlin, DE ", "en"))
	if err != nil {
		t.Fatal(err)
	}
	if u2.Query().Get("q") != "Berlin, DE" || u2.Query().Has("id") {
		t.Errorf("name location: query = %v", u2.Query())
	}
}

func TestFetchCurrentUsesGust(t *testing.T) {
	loc := berlin(t)
	sunrise := time.Date(2026, 10, 10, 7, 30, 0, 0, loc)
	sunset := time.Date(2026, 10, 10, 18, 45, 0, 0, loc)
	serveOWM(t, currentFixture("04d", "7.9", sunrise, sunset), `{"list":[]}`)

	cur, stale, err := fetchCurrent(context.Background(), fetch.New(""), "k", "Berlin", "en", loc)
	if err != nil {
		t.Fatal(err)
	}
	if stale {
		t.Error("stale without cache")
	}
	if cur.WindMPS != 7.9 {
		t.Errorf("WindMPS = %v, want gust 7.9", cur.WindMPS)
	}
	if cur.Icon != "04d" || cur.Temp != -1.6 || cur.MaxTemp != 2.4 || cur.Humidity != 81 {
		t.Errorf("current = %+v", cur)
	}
	if !cur.Sunrise.Equal(sunrise) || cur.Sunrise.Location() != loc || cur.Sunset.Hour() != 18 {
		t.Errorf("sunrise/sunset = %v / %v", cur.Sunrise, cur.Sunset)
	}
}

func TestFetchCurrentFallsBackToSpeed(t *testing.T) {
	loc := berlin(t)
	now := time.Date(2026, 10, 10, 9, 0, 0, 0, loc)
	serveOWM(t, currentFixture("01d", "", now, now), `{"list":[]}`)

	cur, _, err := fetchCurrent(context.Background(), fetch.New(""), "k", "Berlin", "en", loc)
	if err != nil {
		t.Fatal(err)
	}
	if cur.WindMPS != 3.1 {
		t.Errorf("WindMPS = %v, want speed 3.1", cur.WindMPS)
	}
}

func TestFetchForecastSkipsEntriesWithoutWeather(t *testing.T) {
	loc := berlin(t)
	t1 := time.Date(2026, 10, 10, 12, 0, 0, 0, loc)
	body := fmt.Sprintf(`{"list":[{"dt":%d,"main":{"temp":3.0},"weather":[{"icon":"03d"}]},{"dt":%d,"main":{"temp":4.0},"weather":[]}]}`,
		t1.Unix(), t1.Add(3*time.Hour).Unix())
	serveOWM(t, `{}`, body)

	points, _, err := fetchForecast(context.Background(), fetch.New(""), "k", "Berlin", "en", loc)
	if err != nil {
		t.Fatal(err)
	}
	if len(points) != 1 {
		t.Fatalf("len(points) = %d, want 1", len(points))
	}
	if !points[0].Time.Equal(t1) || points[0].Temp != 3.0 || points[0].Icon != "03d" {
		t.Errorf("point = %+v", points[0])
	}
}
```

- [ ] **Step 6: Test laufen lassen – muss fehlschlagen**

Run: `go test ./internal/modules/weather/ -run 'TestQueryURL|TestFetch' -v`
Expected: FAIL – `undefined: queryURL`

- [ ] **Step 7: `owm.go` implementieren**

```go
package weather

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"

	"inkycal/internal/fetch"
)

// owmBaseURL is a variable so tests can point it at an httptest server.
var owmBaseURL = "https://api.openweathermap.org/data/2.5"

// current holds the parts of /weather the module displays (metric units).
type current struct {
	Icon     string
	Temp     float64
	MaxTemp  float64
	Humidity int
	WindMPS  float64 // gust if reported, else mean speed (as in Python)
	Sunrise  time.Time
	Sunset   time.Time
}

// forecastPoint is one 3-hour slot of /forecast (metric units).
type forecastPoint struct {
	Time time.Time
	Temp float64
	Icon string
}

type owmWeather struct {
	Icon string `json:"icon"`
}

type owmCurrent struct {
	Weather []owmWeather `json:"weather"`
	Main    struct {
		Temp     float64 `json:"temp"`
		TempMax  float64 `json:"temp_max"`
		Humidity int     `json:"humidity"`
	} `json:"main"`
	Wind struct {
		Speed float64  `json:"speed"`
		Gust  *float64 `json:"gust"`
	} `json:"wind"`
	Sys struct {
		Sunrise int64 `json:"sunrise"`
		Sunset  int64 `json:"sunset"`
	} `json:"sys"`
}

type owmForecast struct {
	List []struct {
		Dt   int64 `json:"dt"`
		Main struct {
			Temp float64 `json:"temp"`
		} `json:"main"`
		Weather []owmWeather `json:"weather"`
	} `json:"list"`
}

// queryURL builds an OWM 2.5 request URL. Purely numeric locations are city IDs.
func queryURL(endpoint, apiKey, location, lang string) string {
	q := url.Values{}
	loc := strings.TrimSpace(location)
	if _, err := strconv.ParseUint(loc, 10, 64); err == nil {
		q.Set("id", loc)
	} else {
		q.Set("q", loc)
	}
	q.Set("appid", apiKey)
	q.Set("units", "metric")
	q.Set("lang", lang)
	return owmBaseURL + "/" + endpoint + "?" + q.Encode()
}

func fetchCurrent(ctx context.Context, fc *fetch.Client, apiKey, location, lang string, loc *time.Location) (current, bool, error) {
	var raw owmCurrent
	stale, err := fc.GetJSON(ctx, queryURL("weather", apiKey, location, lang), nil, &raw)
	if err != nil {
		return current{}, false, fmt.Errorf("weather: current: %w", err)
	}
	if len(raw.Weather) == 0 {
		return current{}, false, errors.New("weather: current: response has no weather entry")
	}
	wind := raw.Wind.Speed
	if raw.Wind.Gust != nil {
		wind = *raw.Wind.Gust
	}
	return current{
		Icon:     raw.Weather[0].Icon,
		Temp:     raw.Main.Temp,
		MaxTemp:  raw.Main.TempMax,
		Humidity: raw.Main.Humidity,
		WindMPS:  wind,
		Sunrise:  time.Unix(raw.Sys.Sunrise, 0).In(loc),
		Sunset:   time.Unix(raw.Sys.Sunset, 0).In(loc),
	}, stale, nil
}

func fetchForecast(ctx context.Context, fc *fetch.Client, apiKey, location, lang string, loc *time.Location) ([]forecastPoint, bool, error) {
	var raw owmForecast
	stale, err := fc.GetJSON(ctx, queryURL("forecast", apiKey, location, lang), nil, &raw)
	if err != nil {
		return nil, false, fmt.Errorf("weather: forecast: %w", err)
	}
	points := make([]forecastPoint, 0, len(raw.List))
	for _, f := range raw.List {
		if len(f.Weather) == 0 {
			continue
		}
		points = append(points, forecastPoint{
			Time: time.Unix(f.Dt, 0).In(loc),
			Temp: f.Main.Temp,
			Icon: f.Weather[0].Icon,
		})
	}
	return points, stale, nil
}
```

- [ ] **Step 8: Tests laufen lassen – müssen bestehen**

Run: `go test ./internal/modules/weather/ -v`
Expected: PASS (alle 9 Tests)

- [ ] **Step 9: Commit**

```bash
git add internal/modules/weather/convert.go internal/modules/weather/convert_test.go internal/modules/weather/owm.go internal/modules/weather/owm_test.go
git commit -m "feat(weather): OWM client, unit conversion, moon phase"
```

---

### Task 2: Weather – View-Model, Layout und Modul

**Files:**
- Create: `internal/modules/weather/view.go`
- Create: `internal/modules/weather/layout.go`
- Create: `internal/modules/weather/weather.go`
- Test: `internal/modules/weather/view_test.go`
- Test: `internal/modules/weather/weather_test.go`

**Interfaces:**
- Consumes: alles aus Task 1; `timefmt.Format(t time.Time, layout, lang string) string`; `render.NewCanvas`, `Canvas.Icon(x, y, w, h int, glyph string, font render.FontID, ink render.Ink, fillRatio float64)`, `Canvas.Write(x, y, w, h int, text string, o render.TextOpts)`, `Canvas.InkAt(x, y int) (black, colour bool)`, `render.Border(c *render.Canvas, x, y, w, h, r int, shrinkX, shrinkY float64, ink render.Ink)`; `config.ParseCommon`, `config.FlexBool`, `config.FlexInt`; `modules.Env`, `modules.Module`
- Produces:
  - `func New(c config.Common, raw json.RawMessage) (modules.Module, error)` (für Registry, Task 8)
  - `type Module struct` mit `Generate(ctx context.Context, env *modules.Env) (*render.Canvas, error)`
  - paketintern: `type settings`, `type view`, `type fcCell`, `type dayForecast`, `func dailyForecast(points []forecastPoint, now time.Time, offset int) dayForecast`, `func buildView(s settings, cur current, points []forecastPoint, now time.Time) (view, error)`, `type grid`, `func computeGrid(w, h int) grid`, `func draw(c *render.Canvas, v view, g grid)`

- [ ] **Step 1: Failing Tests für View-Model schreiben**

`internal/modules/weather/view_test.go`:

```go
package weather

import (
	"testing"
	"time"

	"inkycal/internal/timefmt"
)

func fixtureCurrent(loc *time.Location, icon string) current {
	return current{
		Icon: icon, Temp: -1.6, MaxTemp: 2.4, Humidity: 81, WindMPS: 7.9,
		Sunrise: time.Date(2026, 10, 10, 7, 30, 0, 0, loc),
		Sunset:  time.Date(2026, 10, 10, 18, 45, 0, 0, loc),
	}
}

func TestBuildViewMetricHourly(t *testing.T) {
	loc := berlin(t)
	now := time.Date(2026, 10, 10, 9, 0, 0, 0, loc)
	points := []forecastPoint{
		{time.Date(2026, 10, 10, 12, 0, 0, 0, loc), 3.0, "03d"},
		{time.Date(2026, 10, 10, 15, 0, 0, 0, loc), 4.3, "10d"},
		{time.Date(2026, 10, 10, 18, 0, 0, 0, loc), 2.0, "01n"},
		{time.Date(2026, 10, 10, 21, 0, 0, 0, loc), -0.5, "13n"},
		{time.Date(2026, 10, 11, 0, 0, 0, 0, loc), -1.0, "13n"},
	}
	s := settings{WindUnit: windMetersSec, Lang: "en"}

	v, err := buildView(s, fixtureCurrent(loc, "04d"), points, now)
	if err != nil {
		t.Fatal(err)
	}
	checks := map[string][2]string{
		"Temperature": {v.Temperature, "-1.6°/2.4°"},
		"Humidity":    {v.Humidity, "81 %"},
		"Wind":        {v.Wind, "7.9 m/s"},
		"Sunrise":     {v.Sunrise, "7:30"},
		"Sunset":      {v.Sunset, "18:45"},
		"CurrentIcon": {v.CurrentIcon, ""},
		"Moon":        {v.Moon, moonPhaseGlyph(now)},
	}
	for name, c := range checks {
		if c[0] != c[1] {
			t.Errorf("%s = %q, want %q", name, c[0], c[1])
		}
	}
	if !v.TempNegative {
		t.Error("TempNegative = false for -1.6°C")
	}
	if len(v.Forecasts) != 4 {
		t.Fatalf("len(Forecasts) = %d, want 4", len(v.Forecasts))
	}
	if want := (fcCell{"12:00", "", "3.0°"}); v.Forecasts[0] != want {
		t.Errorf("Forecasts[0] = %+v, want %+v", v.Forecasts[0], want)
	}
	if want := (fcCell{"15:00", "", "4.3°"}); v.Forecasts[1] != want {
		t.Errorf("Forecasts[1] = %+v, want %+v", v.Forecasts[1], want)
	}
	if want := (fcCell{"21:00", "", "-0.5°"}); v.Forecasts[3] != want {
		t.Errorf("Forecasts[3] = %+v, want %+v", v.Forecasts[3], want)
	}
}

func TestBuildViewImperialDailyBeaufort(t *testing.T) {
	loc := berlin(t)
	now := time.Date(2026, 10, 10, 9, 0, 0, 0, loc)
	points := []forecastPoint{
		{time.Date(2026, 10, 11, 9, 0, 0, 0, loc), 5, "10d"},
		{time.Date(2026, 10, 11, 12, 0, 0, 0, loc), 9, "10d"},
		{time.Date(2026, 10, 11, 15, 0, 0, 0, loc), 7, "04d"},
	}
	s := settings{Fahrenheit: true, WindUnit: windBeaufort, RoundTemp: true, Daily: true, Hour12: true, Lang: "de"}
	cur := fixtureCurrent(loc, "04d")

	v, err := buildView(s, cur, points, now)
	if err != nil {
		t.Fatal(err)
	}
	if v.Temperature != "29F/36F" {
		t.Errorf("Temperature = %q, want 29F/36F", v.Temperature)
	}
	if !v.TempNegative {
		t.Error("29°F must count as freezing")
	}
	if v.Wind != "4 bft" {
		t.Errorf("Wind = %q, want 4 bft", v.Wind)
	}
	if want := timefmt.Format(cur.Sunrise, "h:mm a", "en"); v.Sunrise != want {
		t.Errorf("Sunrise = %q, want %q (12h layout)", v.Sunrise, want)
	}
	if len(v.Forecasts) != 4 {
		t.Fatalf("len(Forecasts) = %d, want 4", len(v.Forecasts))
	}
	want0 := fcCell{
		Stamp: timefmt.Format(time.Date(2026, 10, 11, 0, 0, 0, 0, loc), "ddd", "de"),
		Icon:  "",
		Temp:  "41F/48F",
	}
	if v.Forecasts[0] != want0 {
		t.Errorf("Forecasts[0] = %+v, want %+v", v.Forecasts[0], want0)
	}
	// Days without slots fall back to the first slot (Python behaviour).
	if v.Forecasts[3].Temp != "41F/41F" || v.Forecasts[3].Icon != "" {
		t.Errorf("Forecasts[3] = %+v", v.Forecasts[3])
	}
}

func TestBuildViewHourly12h(t *testing.T) {
	loc := berlin(t)
	now := time.Date(2026, 10, 10, 9, 0, 0, 0, loc)
	slot := time.Date(2026, 10, 10, 12, 0, 0, 0, loc)
	s := settings{WindUnit: windMetersSec, Hour12: true, Lang: "en"}

	v, err := buildView(s, fixtureCurrent(loc, "01d"), []forecastPoint{{slot, 3, "01d"}}, now)
	if err != nil {
		t.Fatal(err)
	}
	if want := timefmt.Format(slot, "h a", "en"); v.Forecasts[0].Stamp != want {
		t.Errorf("stamp = %q, want %q", v.Forecasts[0].Stamp, want)
	}
}

func TestDailyForecastMajorityIcon(t *testing.T) {
	loc := berlin(t)
	now := time.Date(2026, 10, 10, 9, 0, 0, 0, loc)
	points := []forecastPoint{
		{time.Date(2026, 10, 11, 3, 0, 0, 0, loc), 4, "10d"},
		{time.Date(2026, 10, 11, 6, 0, 0, 0, loc), 2, "04d"},
		{time.Date(2026, 10, 11, 9, 0, 0, 0, loc), 8, "04d"},
		{time.Date(2026, 10, 12, 0, 0, 0, 0, loc), 20, "01d"},
	}
	df := dailyForecast(points, now, 1)
	if df.Icon != "04d" || df.Min != 2 || df.Max != 8 {
		t.Errorf("dailyForecast = %+v", df)
	}
	if !df.Date.Equal(time.Date(2026, 10, 11, 0, 0, 0, 0, loc)) {
		t.Errorf("Date = %v", df.Date)
	}
}

func TestBuildViewUnknownIconAndNoForecast(t *testing.T) {
	loc := berlin(t)
	now := time.Date(2026, 10, 10, 9, 0, 0, 0, loc)
	s := settings{WindUnit: windMetersSec, Lang: "en"}
	points := []forecastPoint{{now.Add(3 * time.Hour), 1, "88q"}}

	v, err := buildView(s, fixtureCurrent(loc, "77z"), points, now)
	if err != nil {
		t.Fatal(err)
	}
	if v.CurrentIcon != glyphNA || v.Forecasts[0].Icon != glyphNA {
		t.Errorf("unknown icons: current %q, forecast %q", v.CurrentIcon, v.Forecasts[0].Icon)
	}
	if _, err := buildView(s, fixtureCurrent(loc, "01d"), nil, now); err == nil {
		t.Error("empty forecast accepted")
	}
}

func TestComputeGrid(t *testing.T) {
	g := computeGrid(460, 80) // ratio 5.75 >= 4
	if g.ColW != 65 || g.RowH != 26 || g.IconSmall != 21 || g.LineGap != 0 {
		t.Errorf("grid = %+v", g)
	}
	if g.Cols[0] != 2 || g.Cols[6] != 392 || g.Rows != [3]int{0, 26, 52} {
		t.Errorf("cols/rows = %v / %v", g.Cols, g.Rows)
	}
	g2 := computeGrid(460, 200) // ratio 2.3 < 4
	if g2.RowH != 37 || g2.LineGap != 21 || g2.Rows != [3]int{21, 79, 137} {
		t.Errorf("grid2 = %+v", g2)
	}
}
```

- [ ] **Step 2: Test laufen lassen – muss fehlschlagen**

Run: `go test ./internal/modules/weather/ -run 'TestBuildView|TestDailyForecast|TestComputeGrid' -v`
Expected: FAIL – `undefined: settings`

- [ ] **Step 3: `view.go` implementieren**

```go
package weather

import (
	"errors"
	"fmt"
	"math"
	"strconv"
	"time"

	"inkycal/internal/timefmt"
)

// settings are the display options parsed from settings.json.
type settings struct {
	Fahrenheit bool
	WindUnit   string // windMetersSec, windMilesHour or windBeaufort
	RoundTemp  bool
	RoundWind  bool
	Daily      bool
	Hour12     bool
	Lang       string
}

type fcCell struct {
	Stamp string
	Icon  string // glyph
	Temp  string
}

// view holds every string the draw step writes, already formatted.
type view struct {
	CurrentIcon  string
	Temperature  string
	TempNegative bool
	Humidity     string
	Wind         string
	Moon         string
	Sunrise      string
	Sunset       string
	Forecasts    []fcCell
}

type dayForecast struct {
	Date     time.Time
	Icon     string
	Min, Max float64
}

func (s settings) temp(celsius float64) float64 {
	if s.Fahrenheit {
		return toFahrenheit(celsius)
	}
	return celsius
}

func (s settings) formatTemp(v float64) string {
	dec := 1
	if s.RoundTemp {
		dec = 0
	}
	unit := "°"
	if s.Fahrenheit {
		unit = "F"
	}
	return strconv.FormatFloat(v, 'f', dec, 64) + unit
}

func (s settings) windLabel() string {
	switch s.WindUnit {
	case windBeaufort:
		return "bft"
	case windMilesHour:
		return "mph"
	default:
		return "m/s"
	}
}

// dailyForecast aggregates the 3-hour slots of the day `offset` days after now.
// Without slots for that day it falls back to the first slot, like Python.
// points must not be empty. Ties for the icon go to the first-seen code.
func dailyForecast(points []forecastPoint, now time.Time, offset int) dayForecast {
	y, m, d := now.Date()
	start := time.Date(y, m, d+offset, 0, 0, 0, 0, now.Location())
	end := start.AddDate(0, 0, 1)
	var day []forecastPoint
	for _, p := range points {
		if !p.Time.Before(start) && p.Time.Before(end) {
			day = append(day, p)
		}
	}
	if len(day) == 0 {
		day = points[:1]
	}
	df := dayForecast{Date: start, Min: day[0].Temp, Max: day[0].Temp}
	counts := map[string]int{}
	best := 0
	for _, p := range day {
		df.Min = math.Min(df.Min, p.Temp)
		df.Max = math.Max(df.Max, p.Temp)
		counts[p.Icon]++
		if counts[p.Icon] > best {
			best = counts[p.Icon]
			df.Icon = p.Icon
		}
	}
	return df
}

// buildView formats current weather and four forecast cells.
func buildView(s settings, cur current, points []forecastPoint, now time.Time) (view, error) {
	if len(points) == 0 {
		return view{}, errors.New("weather: forecast contains no entries")
	}
	loc := now.Location()
	temp := s.temp(cur.Temp)
	freezing := 0.0
	if s.Fahrenheit {
		freezing = 32
	}
	windDec := 1
	if s.RoundWind || s.WindUnit == windBeaufort {
		windDec = 0
	}
	clock := "H:mm"
	if s.Hour12 {
		clock = "h:mm a"
	}
	v := view{
		CurrentIcon:  iconGlyph(cur.Icon),
		Temperature:  s.formatTemp(temp) + "/" + s.formatTemp(s.temp(cur.MaxTemp)),
		TempNegative: math.Round(temp) <= freezing,
		Humidity:     fmt.Sprintf("%d %%", cur.Humidity),
		Wind:         strconv.FormatFloat(convertWind(cur.WindMPS, s.WindUnit), 'f', windDec, 64) + " " + s.windLabel(),
		Moon:         moonPhaseGlyph(now),
		Sunrise:      timefmt.Format(cur.Sunrise.In(loc), clock, "en"),
		Sunset:       timefmt.Format(cur.Sunset.In(loc), clock, "en"),
	}
	if s.Daily {
		for d := 1; d <= 4; d++ {
			df := dailyForecast(points, now, d)
			v.Forecasts = append(v.Forecasts, fcCell{
				Stamp: timefmt.Format(df.Date, "ddd", s.Lang),
				Icon:  iconGlyph(df.Icon),
				Temp:  s.formatTemp(s.temp(df.Min)) + "/" + s.formatTemp(s.temp(df.Max)),
			})
		}
		return v, nil
	}
	stamp := "H:mm"
	if s.Hour12 {
		stamp = "h a"
	}
	for i, p := range points {
		if i == 4 {
			break
		}
		v.Forecasts = append(v.Forecasts, fcCell{
			Stamp: timefmt.Format(p.Time.In(loc), stamp, "en"),
			Icon:  iconGlyph(p.Icon),
			Temp:  s.formatTemp(s.temp(p.Temp)),
		})
	}
	return v, nil
}
```

Hinweis: Python formatiert Beaufort-Werte als `4.0 bft`; hier bewusst ohne Nachkommastelle (`4 bft`).

- [ ] **Step 4: `layout.go` implementieren**

```go
package weather

import "inkycal/internal/render"

// grid is the 7-column layout of inkycal_weather.py:
//
//	| current icon | temp/hum/wind | moon/sunrise/sunset | fc1 | fc2 | fc3 | fc4 |
type grid struct {
	ColW, RowH, IconSmall, LineGap int
	Cols                           [7]int
	Rows                           [3]int
}

func computeGrid(w, h int) grid {
	g := grid{ColW: w / 7}
	if float64(w)/float64(h) >= 4 {
		g.RowH = h / 3
	} else {
		g.RowH = int(float64(h) * (1 - float64(h)/float64(w)) / 3)
	}
	spacingTop := (w % g.ColW) / 2
	g.IconSmall = g.ColW / 3
	for i := range g.Cols {
		g.Cols[i] = spacingTop + i*g.ColW
	}
	g.LineGap = (h - spacingTop - 3*g.RowH) / 4
	g.Rows[0] = g.LineGap
	g.Rows[1] = g.Rows[0] + g.LineGap + g.RowH
	g.Rows[2] = g.Rows[1] + g.LineGap + g.RowH
	return g
}

func draw(c *render.Canvas, v view, g grid) {
	const fill = 0.9
	text := render.TextOpts{}
	tempText := render.TextOpts{}
	if v.TempNegative {
		tempText.Ink = render.InkColour
	}
	small := g.ColW - g.IconSmall

	// Column 1: current weather icon over the full height.
	c.Icon(g.Cols[0], 0, g.ColW, c.H, v.CurrentIcon, render.FontWeather, render.InkColour, fill)

	// Column 2: temperature, humidity, wind.
	c.Icon(g.Cols[1], g.Rows[0], g.IconSmall, g.RowH, glyphThermometer, render.FontWeather, render.InkColour, fill)
	c.Write(g.Cols[1]+g.IconSmall, g.Rows[0], small, g.RowH, v.Temperature, tempText)
	c.Icon(g.Cols[1], g.Rows[1], g.IconSmall, g.RowH, glyphHumidity, render.FontWeather, render.InkColour, fill)
	c.Write(g.Cols[1]+g.IconSmall, g.Rows[1], small, g.RowH, v.Humidity, text)
	c.Icon(g.Cols[1], g.Rows[2], g.IconSmall, g.IconSmall, glyphWind, render.FontWeather, render.InkColour, fill)
	c.Write(g.Cols[1]+g.IconSmall, g.Rows[2], small, g.RowH, v.Wind, text)

	// Column 3: moon phase, sunrise, sunset.
	c.Icon(g.Cols[2], g.Rows[0], g.ColW, g.RowH, v.Moon, render.FontWeather, render.InkColour, fill)
	c.Icon(g.Cols[2], g.Rows[1], g.IconSmall, g.IconSmall, glyphSunrise, render.FontWeather, render.InkColour, fill)
	c.Write(g.Cols[2]+g.IconSmall, g.Rows[1], small, g.RowH, v.Sunrise, text)
	c.Icon(g.Cols[2], g.Rows[2], g.IconSmall, g.IconSmall, glyphSunset, render.FontWeather, render.InkColour, fill)
	c.Write(g.Cols[2]+g.IconSmall, g.Rows[2], small, g.RowH, v.Sunset, text)

	// Columns 4-7: forecasts.
	for i, fc := range v.Forecasts {
		x := g.Cols[3+i]
		c.Write(x, g.Rows[0], g.ColW, g.RowH, fc.Stamp, text)
		c.Icon(x, g.Rows[0]+g.RowH, g.ColW, g.RowH+2*g.LineGap, fc.Icon, render.FontWeather, render.InkColour, fill)
		c.Write(x, g.Rows[2], g.ColW, g.RowH, fc.Temp, text)
	}

	borderH := g.Rows[2] + g.RowH
	render.Border(c, g.Cols[0], g.Rows[0], g.ColW*3-3, borderH, 5, 0, 0, render.InkBlack)
	for i := 3; i < 7; i++ {
		render.Border(c, g.Cols[i], g.Rows[0], g.ColW-3, borderH, 5, 0, 0, render.InkBlack)
	}
}
```

- [ ] **Step 5: View-Tests laufen lassen – müssen bestehen**

Run: `go test ./internal/modules/weather/ -run 'TestBuildView|TestDailyForecast|TestComputeGrid' -v`
Expected: PASS (6 Tests)

- [ ] **Step 6: Failing Tests für das Modul schreiben**

`internal/modules/weather/weather_test.go`:

```go
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
```

- [ ] **Step 7: Test laufen lassen – muss fehlschlagen**

Run: `go test ./internal/modules/weather/ -run 'TestNewValidation|TestGenerate' -v`
Expected: FAIL – `undefined: New`

- [ ] **Step 8: `weather.go` implementieren**

```go
// Package weather ports inkycal_weather.py: current weather plus four forecasts from OpenWeatherMap.
package weather

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"

	"inkycal/internal/config"
	"inkycal/internal/modules"
	"inkycal/internal/render"
)

type Module struct {
	common   config.Common
	apiKey   string
	location string
	set      settings
}

type rawConfig struct {
	APIKey           string          `json:"api_key"`
	Location         flexString      `json:"location"`
	RoundTemperature config.FlexBool `json:"round_temperature"`
	RoundWindspeed   config.FlexBool `json:"round_windspeed"`
	ForecastInterval string          `json:"forecast_interval"`
	Units            string          `json:"units"`
	HourFormat       config.FlexInt  `json:"hour_format"`
	UseBeaufort      config.FlexBool `json:"use_beaufort"`
}

// flexString accepts a JSON string or number; the web UI stores city IDs as numbers.
type flexString string

func (f *flexString) UnmarshalJSON(b []byte) error {
	var s string
	if err := json.Unmarshal(b, &s); err == nil {
		*f = flexString(s)
		return nil
	}
	var n json.Number
	if err := json.Unmarshal(b, &n); err != nil {
		return fmt.Errorf("want string or number, got %s", b)
	}
	*f = flexString(n.String())
	return nil
}

func New(c config.Common, raw json.RawMessage) (modules.Module, error) {
	var rc rawConfig
	if err := json.Unmarshal(raw, &rc); err != nil {
		return nil, fmt.Errorf("weather: config: %w", err)
	}
	if strings.TrimSpace(rc.APIKey) == "" {
		return nil, errors.New("weather: api_key is missing")
	}
	if strings.TrimSpace(string(rc.Location)) == "" {
		return nil, errors.New("weather: location is missing")
	}
	if w, h := c.InnerSize(); w < 70 || h < 30 {
		return nil, fmt.Errorf("weather: inner size %dx%d too small (min 70x30)", w, h)
	}
	s := settings{
		Fahrenheit: strings.EqualFold(rc.Units, "imperial"),
		WindUnit:   windMetersSec,
		RoundTemp:  bool(rc.RoundTemperature),
		RoundWind:  bool(rc.RoundWindspeed),
		Daily:      strings.EqualFold(rc.ForecastInterval, "daily"),
		Hour12:     int(rc.HourFormat) == 12,
		Lang:       c.Language,
	}
	switch {
	case bool(rc.UseBeaufort):
		s.WindUnit = windBeaufort
	case s.Fahrenheit:
		s.WindUnit = windMilesHour
	}
	return &Module{common: c, apiKey: rc.APIKey, location: string(rc.Location), set: s}, nil
}

func (m *Module) Generate(ctx context.Context, env *modules.Env) (*render.Canvas, error) {
	var (
		wg             sync.WaitGroup
		cur            current
		points         []forecastPoint
		staleC, staleF bool
		errC, errF     error
	)
	wg.Add(2)
	go func() {
		defer wg.Done()
		cur, staleC, errC = fetchCurrent(ctx, env.Fetch, m.apiKey, m.location, m.set.Lang, env.Loc)
	}()
	go func() {
		defer wg.Done()
		points, staleF, errF = fetchForecast(ctx, env.Fetch, m.apiKey, m.location, m.set.Lang, env.Loc)
	}()
	wg.Wait()
	if err := errors.Join(errC, errF); err != nil {
		return nil, err
	}
	env.Stale = env.Stale || staleC || staleF

	v, err := buildView(m.set, cur, points, env.Now)
	if err != nil {
		return nil, err
	}
	w, h := m.common.InnerSize()
	c := render.NewCanvas(w, h, render.FontDefault, int(m.common.FontSize))
	draw(c, v, computeGrid(w, h))
	return c, nil
}
```

- [ ] **Step 9: Alle Weather-Tests laufen lassen – müssen bestehen**

Run: `go test ./internal/modules/weather/ -v`
Expected: PASS (alle 17 Tests)

- [ ] **Step 10: Commit**

```bash
git add internal/modules/weather/
git commit -m "feat(weather): view model, 7-column layout and module"
```

---

### Task 3: Calendar – Monatsraster, Layout und Event-Logik

**Files:**
- Create: `internal/modules/calendar/grid.go`
- Create: `internal/modules/calendar/events.go`
- Test: `internal/modules/calendar/grid_test.go`
- Test: `internal/modules/calendar/events_test.go`

**Interfaces:**
- Consumes: `ical.Event{Title string; Start, End time.Time; AllDay bool}`; `timefmt.Format`, `timefmt.HumanizeDays(days int, lang string) string`
- Produces (paketintern, von Task 4 genutzt):
  - `func monthGrid(year int, month time.Month, weekStart time.Weekday) [][7]int`
  - `func weekStartDate(now time.Time, weekStart time.Weekday) time.Time`
  - `type layout struct { MonthH, WeekdaysH, CellW, CellH, GridX, GridY, EventsH, NumSize int }`, `func computeLayout(w, h, lineHeight int, showEvents bool, weeks int) layout`, `func (l layout) cell(week, col int) (x, y int)`
  - `type eventRow struct { Date, Time, Title string; AllDay bool }`
  - `func eventDays(events []ical.Event, year int, month time.Month, loc *time.Location) []int`
  - `func buildEventRows(events []ical.Event, now time.Time, limit int, dateFmt, timeFmt, lang string) []eventRow`

- [ ] **Step 1: Failing Tests für Raster und Layout schreiben**

`internal/modules/calendar/grid_test.go`:

```go
package calendar

import (
	"testing"
	"time"
	_ "time/tzdata"
)

func berlin(t *testing.T) *time.Location {
	t.Helper()
	loc, err := time.LoadLocation("Europe/Berlin")
	if err != nil {
		t.Fatal(err)
	}
	return loc
}

func TestMonthGridMondayStart(t *testing.T) {
	weeks := monthGrid(2026, time.October, time.Monday) // 1 Oct 2026 is a Thursday
	if len(weeks) != 5 {
		t.Fatalf("weeks = %d, want 5", len(weeks))
	}
	if weeks[0] != [7]int{0, 0, 0, 1, 2, 3, 4} {
		t.Errorf("week 0 = %v", weeks[0])
	}
	if weeks[4] != [7]int{26, 27, 28, 29, 30, 31, 0} {
		t.Errorf("week 4 = %v", weeks[4])
	}
}

func TestMonthGridSundayStartAndFourWeeks(t *testing.T) {
	weeks := monthGrid(2026, time.October, time.Sunday)
	if weeks[0] != [7]int{0, 0, 0, 0, 1, 2, 3} {
		t.Errorf("sunday week 0 = %v", weeks[0])
	}
	feb := monthGrid(2021, time.February, time.Monday) // starts Monday, 28 days
	if len(feb) != 4 || feb[0] != [7]int{1, 2, 3, 4, 5, 6, 7} {
		t.Errorf("feb 2021 = %v", feb)
	}
}

func TestWeekStartDate(t *testing.T) {
	loc := berlin(t)
	sat := time.Date(2026, 10, 10, 9, 0, 0, 0, loc)
	if got := weekStartDate(sat, time.Monday); !got.Equal(time.Date(2026, 10, 5, 0, 0, 0, 0, loc)) {
		t.Errorf("monday start = %v", got)
	}
	if got := weekStartDate(sat, time.Sunday); !got.Equal(time.Date(2026, 10, 4, 0, 0, 0, 0, loc)) {
		t.Errorf("sunday start = %v", got)
	}
}

func TestComputeLayout(t *testing.T) {
	l := computeLayout(460, 380, 20, true, 5)
	want := layout{MonthH: 38, WeekdaysH: 25, CellW: 65, CellH: 38, GridX: 2, GridY: 64, EventsH: 127, NumSize: 19}
	if l != want {
		t.Errorf("layout = %+v, want %+v", l, want)
	}
	x, y := l.cell(1, 5)
	if x != 2+65*5 || y != 64+38 {
		t.Errorf("cell(1,5) = %d,%d", x, y)
	}
	noEvents := computeLayout(460, 380, 20, false, 5)
	if noEvents.EventsH != 0 || noEvents.CellH != (380-38-25)/6 {
		t.Errorf("no events layout = %+v", noEvents)
	}
}
```

- [ ] **Step 2: Failing Tests für Event-Logik schreiben**

`internal/modules/calendar/events_test.go`:

```go
package calendar

import (
	"slices"
	"testing"
	"time"

	"inkycal/internal/ical"
	"inkycal/internal/timefmt"
)

func TestEventDays(t *testing.T) {
	loc := berlin(t)
	d := func(month time.Month, day, hour int) time.Time {
		return time.Date(2026, month, day, hour, 0, 0, 0, loc)
	}
	events := []ical.Event{
		{Title: "all-day 5th", Start: d(10, 5, 0), End: d(10, 6, 0), AllDay: true},
		{Title: "timed 7th", Start: d(10, 7, 10), End: d(10, 7, 11)},
		{Title: "from september", Start: d(9, 29, 0), End: d(10, 3, 0), AllDay: true},
		{Title: "november", Start: d(11, 3, 10), End: d(11, 3, 11)},
		{Title: "over midnight", Start: d(10, 20, 22), End: d(10, 21, 2)},
	}
	got := eventDays(events, 2026, time.October, loc)
	want := []int{1, 2, 5, 7, 20, 21}
	if !slices.Equal(got, want) {
		t.Errorf("eventDays = %v, want %v", got, want)
	}
}

func TestBuildEventRows(t *testing.T) {
	loc := berlin(t)
	now := time.Date(2026, 10, 10, 9, 0, 0, 0, loc)
	at := func(month time.Month, day, hour int) time.Time {
		return time.Date(2026, month, day, hour, 0, 0, 0, loc)
	}
	events := []ical.Event{
		{Title: "Zahnarzt", Start: at(10, 10, 10), End: at(10, 10, 11)},
		{Title: "Vergangen", Start: at(10, 9, 10), End: at(10, 9, 11)},
		{Title: "Urlaub", Start: at(10, 12, 0), End: at(10, 15, 0), AllDay: true},
		{Title: "Läuft", Start: at(10, 9, 8), End: at(10, 10, 12)},
		{Title: "Fern", Start: at(11, 20, 10), End: at(11, 20, 11)},
	}

	rows := buildEventRows(events, now, 10, "D MMM", "HH:mm", "en")
	if len(rows) != 3 {
		t.Fatalf("rows = %+v, want 3 rows", rows)
	}
	if rows[0].Title != "Läuft" || rows[1].Title != "Zahnarzt" {
		t.Errorf("order = %q, %q", rows[0].Title, rows[1].Title)
	}
	if rows[1].Date != "10 Oct" || rows[1].Time != "10:00" || rows[1].AllDay {
		t.Errorf("Zahnarzt row = %+v", rows[1])
	}
	wantTitle := "Urlaub (" + timefmt.HumanizeDays(3, "en") + ")"
	if rows[2].Title != wantTitle || !rows[2].AllDay {
		t.Errorf("Urlaub row = %+v, want title %q", rows[2], wantTitle)
	}

	if limited := buildEventRows(events, now, 2, "D MMM", "HH:mm", "en"); len(limited) != 2 {
		t.Errorf("limit 2 gave %d rows", len(limited))
	}
	if none := buildEventRows(events, now, -1, "D MMM", "HH:mm", "en"); len(none) != 0 {
		t.Errorf("negative limit gave %d rows", len(none))
	}
}
```

- [ ] **Step 3: Tests laufen lassen – müssen fehlschlagen**

Run: `go test ./internal/modules/calendar/ -v`
Expected: FAIL – `undefined: monthGrid`

- [ ] **Step 4: `grid.go` implementieren**

```go
package calendar

import "time"

// monthGrid mirrors Python's calendar.monthcalendar: weeks of 7 days, 0 = outside the month.
func monthGrid(year int, month time.Month, weekStart time.Weekday) [][7]int {
	first := time.Date(year, month, 1, 0, 0, 0, 0, time.UTC)
	days := first.AddDate(0, 1, -1).Day()
	col := (int(first.Weekday()) - int(weekStart) + 7) % 7
	var weeks [][7]int
	var week [7]int
	for d := 1; d <= days; d++ {
		week[col] = d
		col++
		if col == 7 {
			weeks = append(weeks, week)
			week = [7]int{}
			col = 0
		}
	}
	if col > 0 {
		weeks = append(weeks, week)
	}
	return weeks
}

// weekStartDate returns midnight of the first day of now's week.
func weekStartDate(now time.Time, weekStart time.Weekday) time.Time {
	offset := (int(now.Weekday()) - int(weekStart) + 7) % 7
	y, m, d := now.Date()
	return time.Date(y, m, d-offset, 0, 0, 0, 0, now.Location())
}

// layout holds the section sizes of inkycal_calendar.py.
type layout struct {
	MonthH, WeekdaysH int
	CellW, CellH      int
	GridX, GridY      int
	EventsH           int // includes the space of unused grid rows
	NumSize           int // font size of the day numbers
}

func computeLayout(w, h, lineHeight int, showEvents bool, weeks int) layout {
	l := layout{
		MonthH:    int(float64(h) * 0.10),
		WeekdaysH: int(float64(lineHeight) * 1.25),
	}
	calendarH := h - l.MonthH - l.WeekdaysH
	if showEvents {
		calendarH = int(float64(h) * 0.6)
		l.EventsH = h - l.MonthH - l.WeekdaysH - calendarH
	}
	l.CellW = w / 7
	l.CellH = calendarH / 6
	l.GridX = (w % 7) / 2
	l.GridY = l.MonthH + l.WeekdaysH + (h%6)/2
	l.NumSize = max(6, int(float64(min(l.CellW, l.CellH))*0.5))
	if showEvents {
		l.EventsH += (6 - weeks) * l.CellH
	}
	return l
}

func (l layout) cell(week, col int) (x, y int) {
	return l.GridX + l.CellW*col, l.GridY + l.CellH*week
}
```

- [ ] **Step 5: `events.go` implementieren**

```go
package calendar

import (
	"sort"
	"time"

	"inkycal/internal/ical"
	"inkycal/internal/timefmt"
)

type eventRow struct {
	Date   string
	Time   string
	Title  string
	AllDay bool
}

func dateOnly(t time.Time) time.Time {
	y, m, d := t.Date()
	return time.Date(y, m, d, 0, 0, 0, 0, t.Location())
}

// eventDays returns the sorted day numbers of (year, month) touched by an event.
// End is exclusive (an all-day event ending at the next midnight marks one day)
// and only days of the shown month count — Python marked both wrongly.
func eventDays(events []ical.Event, year int, month time.Month, loc *time.Location) []int {
	seen := map[int]bool{}
	for _, e := range events {
		start := e.Start.In(loc)
		last := start
		if e.End.After(e.Start) {
			last = e.End.In(loc).Add(-time.Nanosecond)
		}
		for d := dateOnly(start); !d.After(dateOnly(last)); d = d.AddDate(0, 0, 1) {
			if d.Year() == year && d.Month() == month {
				seen[d.Day()] = true
			}
		}
	}
	days := make([]int, 0, len(seen))
	for d := range seen {
		days = append(days, d)
	}
	sort.Ints(days)
	return days
}

// buildEventRows lists events that have not ended and start within four weeks,
// sorted by start, at most limit rows.
func buildEventRows(events []ical.Event, now time.Time, limit int, dateFmt, timeFmt, lang string) []eventRow {
	horizon := now.AddDate(0, 0, 28)
	var upcoming []ical.Event
	for _, e := range events {
		if e.End.After(now) && e.Start.Before(horizon) {
			upcoming = append(upcoming, e)
		}
	}
	sort.SliceStable(upcoming, func(i, j int) bool { return upcoming[i].Start.Before(upcoming[j].Start) })
	limit = max(0, limit)
	if len(upcoming) > limit {
		upcoming = upcoming[:limit]
	}
	rows := make([]eventRow, 0, len(upcoming))
	for _, e := range upcoming {
		start := e.Start.In(now.Location())
		title := e.Title
		if days := int(e.End.Sub(e.Start).Hours() / 24); days > 1 {
			title += " (" + timefmt.HumanizeDays(days, lang) + ")"
		}
		rows = append(rows, eventRow{
			Date:   timefmt.Format(start, dateFmt, lang),
			Time:   timefmt.Format(start, timeFmt, lang),
			Title:  title,
			AllDay: e.AllDay,
		})
	}
	return rows
}
```

- [ ] **Step 6: Tests laufen lassen – müssen bestehen**

Run: `go test ./internal/modules/calendar/ -v`
Expected: PASS (6 Tests)

- [ ] **Step 7: Commit**

```bash
git add internal/modules/calendar/
git commit -m "feat(calendar): month grid, layout and event logic"
```

---

### Task 4: Calendar – Modul und Zeichnen

**Files:**
- Create: `internal/modules/calendar/calendar.go`
- Test: `internal/modules/calendar/calendar_test.go`

**Interfaces:**
- Consumes: Task 3; `ical.Load(ctx context.Context, fc *fetch.Client, urls, files []string, from, to time.Time, loc *time.Location) ([]ical.Event, bool, error)`; `render.NewCanvas`, `Canvas.Write`, `Canvas.SetFont(id render.FontID, size int)`, `Canvas.Font() (render.FontID, int)`, `Canvas.LineHeight() int`, `Canvas.TextWidth(s string) int`, `Canvas.FillCircle(cx, cy, r int, ink render.Ink)`, `render.Border`, `render.InkWhite`, `render.FontRegular`; `config.StringList`, `config.FlexBool`
- Produces: `func New(c config.Common, raw json.RawMessage) (modules.Module, error)` (für Registry, Task 8)

- [ ] **Step 1: Failing Tests schreiben**

`internal/modules/calendar/calendar_test.go`:

```go
package calendar

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"inkycal/internal/config"
	"inkycal/internal/fetch"
	"inkycal/internal/modules"
	"inkycal/internal/render"
)

var testICS = strings.Join([]string{
	"BEGIN:VCALENDAR",
	"VERSION:2.0",
	"PRODID:-//inkycal//test//EN",
	"BEGIN:VEVENT",
	"UID:1@test",
	"DTSTAMP:20261001T000000Z",
	"DTSTART:20261010T080000Z",
	"DTEND:20261010T090000Z",
	"SUMMARY:Zahnarzt",
	"END:VEVENT",
	"BEGIN:VEVENT",
	"UID:2@test",
	"DTSTAMP:20261001T000000Z",
	"DTSTART;VALUE=DATE:20261012",
	"DTEND;VALUE=DATE:20261013",
	"SUMMARY:Urlaub",
	"END:VEVENT",
	"END:VCALENDAR",
	"",
}, "\r\n")

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

func serveICS(t *testing.T) (url string, hits *atomic.Int32) {
	t.Helper()
	hits = &atomic.Int32{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		io.WriteString(w, testICS)
	}))
	t.Cleanup(srv.Close)
	return srv.URL + "/cal.ics", hits
}

// cellRect returns the grid cell of a day as used by Generate.
func cellRect(t *testing.T, w, h, fontSize int, now time.Time, showEvents bool, day int) (x0, y0, x1, y1 int) {
	t.Helper()
	weeks := monthGrid(now.Year(), now.Month(), time.Monday)
	lh := render.NewCanvas(w, h, render.FontDefault, fontSize).LineHeight()
	l := computeLayout(w, h, lh, showEvents, len(weeks))
	for wi, week := range weeks {
		for col, d := range week {
			if d == day {
				x, y := l.cell(wi, col)
				return x, y, x + l.CellW, y + l.CellH
			}
		}
	}
	t.Fatalf("day %d not in grid", day)
	return
}

// cellInk reports ink inside the grid cell of a day (canvas size taken from c, font size 14).
func cellInk(t *testing.T, c *render.Canvas, now time.Time, showEvents bool, day int) (black, colour bool) {
	t.Helper()
	x0, y0, x1, y1 := cellRect(t, c.W, c.H, 14, now, showEvents, day)
	return inkIn(c, x0, y0, x1, y1)
}

func TestNewRejectsTinySize(t *testing.T) {
	tiny := mustCommon(t, `{"size":[60,60],"padding_x":0,"padding_y":0,"fontsize":14}`)
	if _, err := New(tiny, json.RawMessage(`{}`)); err == nil {
		t.Error("60x60 accepted")
	}
}

func TestGenerateWithEvents(t *testing.T) {
	loc := berlin(t)
	now := time.Date(2026, 10, 10, 9, 0, 0, 0, loc)
	icsURL, hits := serveICS(t)
	common := mustCommon(t, `{"size":[480,400],"padding_x":10,"padding_y":10,"fontsize":14,"language":"en"}`)
	m, err := New(common, json.RawMessage(fmt.Sprintf(
		`{"week_starts_on":"Monday","show_events":true,"ical_urls":%q,"ical_files":null,"date_format":"D MMM","time_format":"HH:mm"}`, icsURL)))
	if err != nil {
		t.Fatal(err)
	}
	env := &modules.Env{Now: now, Loc: loc, Fetch: fetch.New("")}

	c, err := m.Generate(context.Background(), env)
	if err != nil {
		t.Fatal(err)
	}
	if c.W != 460 || c.H != 380 {
		t.Fatalf("canvas = %dx%d", c.W, c.H)
	}
	if hits.Load() == 0 {
		t.Error("iCal URL was never fetched")
	}
	if _, colour := cellInk(t, c, now, true, 10); !colour {
		t.Error("today (10th) has no red circle")
	}
	if _, colour := cellInk(t, c, now, true, 12); !colour {
		t.Error("event day (12th) has no red border")
	}
	if _, colour := cellInk(t, c, now, true, 20); colour {
		t.Error("day without events (20th) has red ink")
	}
	weeks := monthGrid(2026, time.October, time.Monday)
	l := computeLayout(460, 380, render.NewCanvas(460, 380, render.FontDefault, 14).LineHeight(), true, len(weeks))
	// +2 skips the bottom edge of the last grid row.
	if black, _ := inkIn(c, 0, c.H-l.EventsH+2, c.W, c.H); !black {
		t.Error("events list area is empty")
	}
}

func TestGenerateWithoutEventsSkipsFetch(t *testing.T) {
	loc := berlin(t)
	now := time.Date(2026, 10, 10, 9, 0, 0, 0, loc)
	icsURL, hits := serveICS(t)
	common := mustCommon(t, `{"size":[480,400],"padding_x":10,"padding_y":10,"fontsize":14,"language":"de"}`)
	m, err := New(common, json.RawMessage(fmt.Sprintf(`{"show_events":"False","ical_urls":%q}`, icsURL)))
	if err != nil {
		t.Fatal(err)
	}
	c, err := m.Generate(context.Background(), &modules.Env{Now: now, Loc: loc, Fetch: fetch.New("")})
	if err != nil {
		t.Fatal(err)
	}
	if hits.Load() != 0 {
		t.Error("show_events=false still fetched the calendar")
	}
	if _, colour := cellInk(t, c, now, false, 10); !colour {
		t.Error("today circle missing")
	}
}
```

- [ ] **Step 2: Test laufen lassen – muss fehlschlagen**

Run: `go test ./internal/modules/calendar/ -run 'TestNewRejectsTinySize|TestGenerate' -v`
Expected: FAIL – `undefined: New`

- [ ] **Step 3: `calendar.go` implementieren**

```go
// Package calendar ports inkycal_calendar.py: month grid plus upcoming events.
package calendar

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"inkycal/internal/config"
	"inkycal/internal/ical"
	"inkycal/internal/modules"
	"inkycal/internal/render"
	"inkycal/internal/timefmt"
)

type Module struct {
	common     config.Common
	weekStart  time.Weekday
	showEvents bool
	urls       []string
	files      []string
	dateFormat string
	timeFormat string
}

type rawConfig struct {
	WeekStartsOn string            `json:"week_starts_on"`
	ShowEvents   *config.FlexBool  `json:"show_events"`
	ICalURLs     config.StringList `json:"ical_urls"`
	ICalFiles    config.StringList `json:"ical_files"`
	DateFormat   string            `json:"date_format"`
	TimeFormat   string            `json:"time_format"`
}

func New(c config.Common, raw json.RawMessage) (modules.Module, error) {
	var rc rawConfig
	if err := json.Unmarshal(raw, &rc); err != nil {
		return nil, fmt.Errorf("calendar: config: %w", err)
	}
	if w, h := c.InnerSize(); w < 70 || h < 100 {
		return nil, fmt.Errorf("calendar: inner size %dx%d too small (min 70x100)", w, h)
	}
	m := &Module{
		common:     c,
		weekStart:  time.Monday,
		showEvents: true,
		urls:       []string(rc.ICalURLs),
		files:      []string(rc.ICalFiles),
		dateFormat: "D MMM",
		timeFormat: "HH:mm",
	}
	if strings.EqualFold(rc.WeekStartsOn, "Sunday") {
		m.weekStart = time.Sunday
	}
	if rc.ShowEvents != nil {
		m.showEvents = bool(*rc.ShowEvents)
	}
	if rc.DateFormat != "" {
		m.dateFormat = rc.DateFormat
	}
	if rc.TimeFormat != "" {
		m.timeFormat = rc.TimeFormat
	}
	return m, nil
}

func (m *Module) Generate(ctx context.Context, env *modules.Env) (*render.Canvas, error) {
	now := env.Now
	w, h := m.common.InnerSize()
	c := render.NewCanvas(w, h, render.FontDefault, int(m.common.FontSize))
	weeks := monthGrid(now.Year(), now.Month(), m.weekStart)
	l := computeLayout(w, h, c.LineHeight(), m.showEvents, len(weeks))

	var events []ical.Event
	if m.showEvents && (len(m.urls) > 0 || len(m.files) > 0) {
		// One load covers both the month (event borders) and the next four weeks (list).
		from := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, env.Loc)
		to := from.AddDate(0, 1, 0)
		if horizon := now.AddDate(0, 0, 28); horizon.After(to) {
			to = horizon
		}
		evs, stale, err := ical.Load(ctx, env.Fetch, m.urls, m.files, from, to, env.Loc)
		if err != nil {
			return nil, fmt.Errorf("calendar: %w", err)
		}
		env.Stale = env.Stale || stale
		events = evs
	}

	m.drawGrid(c, l, weeks, now, events)
	if m.showEvents {
		m.drawEvents(c, l, now, events)
	}
	return c, nil
}

func (m *Module) drawGrid(c *render.Canvas, l layout, weeks [][7]int, now time.Time, events []ical.Event) {
	lang := m.common.Language
	c.Write(0, 0, c.W, l.MonthH, timefmt.Format(now, "MMMM", lang), render.TextOpts{Autofit: true})

	first := weekStartDate(now, m.weekStart)
	for i := 0; i < 7; i++ {
		name := timefmt.Format(first.AddDate(0, 0, i), "ddd", lang)
		c.Write(l.GridX+l.CellW*i, l.MonthH, l.CellW, l.WeekdaysH, name, render.TextOpts{Autofit: true, FillHeight: 0.9})
	}

	_, baseSize := c.Font()
	pos := map[int][2]int{}
	c.SetFont(render.FontDefault, l.NumSize)
	for wi, week := range weeks {
		for col, day := range week {
			if day == 0 {
				continue
			}
			x, y := l.cell(wi, col)
			pos[day] = [2]int{x, y}
			if day != now.Day() {
				c.Write(x, y, l.CellW, l.CellH, strconv.Itoa(day), render.TextOpts{})
			}
		}
	}

	// Today: filled circle (red on the panel) with a white number.
	today := pos[now.Day()]
	c.FillCircle(today[0]+l.CellW/2, today[1]+l.CellH/2, l.CellW/4, render.InkColour)
	c.SetFont(render.FontRegular, l.NumSize)
	c.Write(today[0], today[1], l.CellW, l.CellH, strconv.Itoa(now.Day()), render.TextOpts{Ink: render.InkWhite})
	c.SetFont(render.FontDefault, baseSize)

	for _, day := range eventDays(events, now.Year(), now.Month(), now.Location()) {
		p := pos[day]
		render.Border(c, p[0], p[1], l.CellW, l.CellH, 6, 0.1, 0.1, render.InkColour)
	}
}

func (m *Module) drawEvents(c *render.Canvas, l layout, now time.Time, events []ical.Event) {
	const lineSpacing = 2
	lh := c.LineHeight() + lineSpacing
	maxLines := l.EventsH / (lh + lineSpacing)
	if maxLines <= 0 {
		return
	}
	offset := c.H - l.EventsH
	lineY := func(i int) int { return offset + int(float64(l.EventsH)/float64(maxLines)*float64(i)) }
	left := render.TextOpts{Align: render.AlignLeft}

	rows := buildEventRows(events, now, maxLines, m.dateFormat, m.timeFormat, m.common.Language)
	if len(rows) == 0 {
		dash := "- "
		n := c.W / max(1, c.TextWidth(dash))
		c.Write(0, lineY(0), c.W, lh, strings.Repeat(dash, n), left)
		return
	}

	dateW, timeW := 0, 0
	for _, r := range rows {
		dateW = max(dateW, int(float64(c.TextWidth(r.Date))*1.1))
		timeW = max(timeW, int(float64(c.TextWidth(r.Time))*1.1))
	}
	for i, r := range rows {
		y := lineY(i)
		c.Write(0, y, dateW, lh, r.Date, left)
		if r.AllDay {
			c.Write(dateW, y, c.W-dateW, lh, r.Title, left)
			continue
		}
		c.Write(dateW, y, timeW, lh, r.Time, left)
		c.Write(dateW+timeW, y, c.W-dateW-timeW, lh, r.Title, left)
	}
}
```

- [ ] **Step 4: Tests laufen lassen – müssen bestehen**

Run: `go test ./internal/modules/calendar/ -v`
Expected: PASS (alle 9 Tests)

- [ ] **Step 5: Commit**

```bash
git add internal/modules/calendar/calendar.go internal/modules/calendar/calendar_test.go
git commit -m "feat(calendar): calendar module with today circle and event list"
```

---

### Task 5: Todoist – API-v1-Client mit Cursor-Pagination

Die offizielle `todoist-api-python` (4.x, von Inkycal genutzt) spricht `https://api.todoist.com/api/v1/`, Auth per `Authorization: Bearer <token>`, Listen-Endpunkte `projects` und `tasks` liefern `{"results": [...], "next_cursor": "..."|null}`; Folgeseiten mit Query-Parameter `cursor`, `limit` maximal 200. Relevante Task-Felder: `content`, `project_id`, `priority` (4 = höchste), `due.date` (`"2026-10-10"` oder Datum mit Uhrzeit).

**Files:**
- Create: `internal/modules/todoist/api.go`
- Test: `internal/modules/todoist/api_test.go`

**Interfaces:**
- Consumes: `fetch.Client.GetJSON`
- Produces (paketintern, von Task 6 genutzt):
  - `var todoistBaseURL string`
  - `type project struct { ID, Name string }`, `type due struct { Date string }`, `type task struct { Content, ProjectID string; Priority int; Due *due }`
  - `func fetchAll[T any](ctx context.Context, fc *fetch.Client, path, token string) ([]T, bool, error)`

- [ ] **Step 1: Failing Test schreiben**

`internal/modules/todoist/api_test.go`:

```go
package todoist

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"inkycal/internal/fetch"
)

// serveTodoist points todoistBaseURL at a test server with the given handler.
func serveTodoist(t *testing.T, h http.HandlerFunc) {
	t.Helper()
	srv := httptest.NewServer(h)
	old := todoistBaseURL
	todoistBaseURL = srv.URL
	t.Cleanup(func() {
		todoistBaseURL = old
		srv.Close()
	})
}

func TestFetchAllFollowsCursor(t *testing.T) {
	serveTodoist(t, func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer tok" {
			t.Errorf("Authorization = %q", got)
		}
		if r.URL.Path != "/tasks" || r.URL.Query().Get("limit") != "200" {
			t.Errorf("request = %s", r.URL)
		}
		switch r.URL.Query().Get("cursor") {
		case "":
			io.WriteString(w, `{"results":[{"content":"A","project_id":"p1","priority":4,"due":{"date":"2026-10-10"}}],"next_cursor":"c2"}`)
		case "c2":
			io.WriteString(w, `{"results":[{"content":"B","project_id":"p1","priority":1,"due":null}],"next_cursor":null}`)
		default:
			http.Error(w, "bad cursor", http.StatusBadRequest)
		}
	})

	tasks, stale, err := fetchAll[task](context.Background(), fetch.New(""), "tasks", "tok")
	if err != nil {
		t.Fatal(err)
	}
	if stale {
		t.Error("stale without cache")
	}
	if len(tasks) != 2 || tasks[0].Content != "A" || tasks[1].Content != "B" {
		t.Fatalf("tasks = %+v", tasks)
	}
	if tasks[0].Due == nil || tasks[0].Due.Date != "2026-10-10" || tasks[0].Priority != 4 || tasks[1].Due != nil {
		t.Errorf("fields = %+v / %+v", tasks[0], tasks[1])
	}
}

func TestFetchAllReportsHTTPError(t *testing.T) {
	serveTodoist(t, func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
	})
	if _, _, err := fetchAll[project](context.Background(), fetch.New(""), "projects", "bad"); err == nil {
		t.Error("401 not reported")
	}
}
```

- [ ] **Step 2: Test laufen lassen – muss fehlschlagen**

Run: `go test ./internal/modules/todoist/ -v`
Expected: FAIL – `undefined: todoistBaseURL`

- [ ] **Step 3: `api.go` implementieren**

```go
package todoist

import (
	"context"
	"fmt"
	"net/http"
	"net/url"

	"inkycal/internal/fetch"
)

// todoistBaseURL is a variable so tests can point it at an httptest server.
var todoistBaseURL = "https://api.todoist.com/api/v1"

// maxPages guards against a server that never stops returning cursors.
const maxPages = 50

type project struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type due struct {
	Date string `json:"date"`
}

type task struct {
	Content   string `json:"content"`
	ProjectID string `json:"project_id"`
	Priority  int    `json:"priority"`
	Due       *due   `json:"due"`
}

type page[T any] struct {
	Results    []T     `json:"results"`
	NextCursor *string `json:"next_cursor"`
}

// fetchAll reads every page of a list endpoint ("projects" or "tasks").
func fetchAll[T any](ctx context.Context, fc *fetch.Client, path, token string) ([]T, bool, error) {
	header := http.Header{}
	header.Set("Authorization", "Bearer "+token)
	var all []T
	stale := false
	cursor := ""
	for range maxPages {
		q := url.Values{}
		q.Set("limit", "200")
		if cursor != "" {
			q.Set("cursor", cursor)
		}
		var p page[T]
		s, err := fc.GetJSON(ctx, todoistBaseURL+"/"+path+"?"+q.Encode(), header, &p)
		if err != nil {
			return nil, false, fmt.Errorf("todoist: %s: %w", path, err)
		}
		stale = stale || s
		all = append(all, p.Results...)
		if p.NextCursor == nil || *p.NextCursor == "" {
			return all, stale, nil
		}
		cursor = *p.NextCursor
	}
	return nil, false, fmt.Errorf("todoist: %s: more than %d pages", path, maxPages)
}
```

- [ ] **Step 4: Tests laufen lassen – müssen bestehen**

Run: `go test ./internal/modules/todoist/ -v`
Expected: PASS (2 Tests)

- [ ] **Step 5: Commit**

```bash
git add internal/modules/todoist/
git commit -m "feat(todoist): API v1 client with cursor pagination"
```

---

### Task 6: Todoist – Zeilenmodell, Zeichnen und Modul

**Files:**
- Create: `internal/modules/todoist/view.go`
- Create: `internal/modules/todoist/todoist.go`
- Test: `internal/modules/todoist/view_test.go`
- Test: `internal/modules/todoist/todoist_test.go`

**Interfaces:**
- Consumes: Task 5; `timefmt.Format`; `render.NewCanvas`, `Canvas.Write`, `Canvas.LineHeight`, `Canvas.TextWidth`, `Canvas.InkAt`; `config.ParseCommon`, `config.StringList`, `config.FlexBool`; `modules.Env`
- Produces:
  - `func New(c config.Common, raw json.RawMessage) (modules.Module, error)` (für Registry, Task 8)
  - paketintern: `type row struct { Project, Due string; Overdue bool; Priority string; PriorityColour bool; Name string }`, `func buildRows(projects []project, tasks []task, filter []string, showPriority bool, now time.Time) ([]row, error)`, `func drawRows(c *render.Canvas, rows []row)`

- [ ] **Step 1: Failing Tests für das Zeilenmodell schreiben**

`internal/modules/todoist/view_test.go`:

```go
package todoist

import (
	"slices"
	"testing"
	"time"
	_ "time/tzdata"
)

func berlin(t *testing.T) *time.Location {
	t.Helper()
	loc, err := time.LoadLocation("Europe/Berlin")
	if err != nil {
		t.Fatal(err)
	}
	return loc
}

var (
	testProjects = []project{{ID: "p1", Name: "Inbox"}, {ID: "p2", Name: "Work"}}
	testTasks    = []task{
		{Content: "Steuer", ProjectID: "p2", Priority: 4, Due: &due{Date: "2026-10-08"}},
		{Content: "Meeting vorbereiten", ProjectID: "p2", Priority: 1, Due: &due{Date: "2026-10-10T15:00:00Z"}},
		{Content: "Milch", ProjectID: "p1", Priority: 2},
		{Content: "Altes Projekt", ProjectID: "p9", Priority: 3},
		{Content: "Kaputt", ProjectID: "p1", Priority: 1, Due: &due{Date: "not-a-date"}},
		{Content: "Brief", ProjectID: "p1", Priority: 3, Due: &due{Date: "2026-10-12T09:00:00"}},
	}
)

func TestBuildRows(t *testing.T) {
	now := time.Date(2026, 10, 10, 9, 0, 0, 0, berlin(t))
	rows, err := buildRows(testProjects, testTasks, nil, true, now)
	if err != nil {
		t.Fatal(err)
	}
	want := []row{
		{Project: "Inbox", Due: "12-Oct-26", Priority: "● ", Name: "Brief"},
		{Project: "Inbox", Priority: "○ ", Name: "Milch"},
		{Project: "Work", Due: "8-Oct-26", Overdue: true, Priority: "● ", PriorityColour: true, Name: "Steuer"},
		{Project: "Work", Due: "TODAY", Name: "Meeting vorbereiten"},
	}
	if !slices.Equal(rows, want) {
		t.Errorf("rows =\n%+v\nwant\n%+v", rows, want)
	}
}

func TestBuildRowsWithoutPriority(t *testing.T) {
	now := time.Date(2026, 10, 10, 9, 0, 0, 0, berlin(t))
	rows, err := buildRows(testProjects, testTasks, nil, false, now)
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range rows {
		if r.Priority != "" || r.PriorityColour {
			t.Errorf("priority shown although disabled: %+v", r)
		}
	}
}

func TestBuildRowsFilter(t *testing.T) {
	now := time.Date(2026, 10, 10, 9, 0, 0, 0, berlin(t))
	rows, err := buildRows(testProjects, testTasks, []string{"Work"}, true, now)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 2 || rows[0].Project != "Work" || rows[1].Project != "Work" {
		t.Errorf("filtered rows = %+v", rows)
	}
	if _, err := buildRows(testProjects, testTasks, []string{"Nope"}, true, now); err == nil {
		t.Error("filter without matching project accepted")
	}
}
```

- [ ] **Step 2: Test laufen lassen – muss fehlschlagen**

Run: `go test ./internal/modules/todoist/ -run TestBuildRows -v`
Expected: FAIL – `undefined: buildRows`

- [ ] **Step 3: `view.go` implementieren**

```go
package todoist

import (
	"fmt"
	"log/slog"
	"slices"
	"sort"
	"strings"
	"time"

	"inkycal/internal/render"
	"inkycal/internal/timefmt"
)

const (
	bulletFilled = "● "
	bulletEmpty  = "○ "
)

// row is one rendered todo line.
type row struct {
	Project        string
	Due            string
	Overdue        bool
	Priority       string
	PriorityColour bool
	Name           string
}

// parseDue reads the date part of a Todoist due value ("2026-10-10", "2026-10-10T15:00:00Z", ...).
func parseDue(s string, loc *time.Location) (time.Time, error) {
	if len(s) < 10 {
		return time.Time{}, fmt.Errorf("due date %q too short", s)
	}
	return time.ParseInLocation("2006-01-02", s[:10], loc)
}

// buildRows groups tasks by project (API order), sorts each group by due date
// (tasks with a date first) and priority, and formats every line.
func buildRows(projects []project, tasks []task, filter []string, showPriority bool, now time.Time) ([]row, error) {
	names := make(map[string]string, len(projects))
	var order []string
	for _, p := range projects {
		if len(filter) > 0 && !slices.Contains(filter, p.Name) {
			continue
		}
		names[p.ID] = p.Name
		order = append(order, p.ID)
	}
	if len(filter) > 0 && len(order) == 0 {
		return nil, fmt.Errorf("todoist: no project matches project_filter %q", strings.Join(filter, ","))
	}

	y, m, d := now.Date()
	today := time.Date(y, m, d, 0, 0, 0, 0, now.Location())

	type item struct {
		row
		due      time.Time
		hasDue   bool
		priority int
	}
	groups := map[string][]item{}
	for _, t := range tasks {
		name, ok := names[t.ProjectID]
		if !ok {
			continue
		}
		it := item{row: row{Project: name, Name: t.Content}, priority: t.Priority}
		if t.Due != nil && t.Due.Date != "" {
			dueDate, err := parseDue(t.Due.Date, now.Location())
			if err != nil {
				slog.Warn("todoist: skipping task with unparsable due date", "task", t.Content, "due", t.Due.Date)
				continue
			}
			it.due, it.hasDue = dueDate, true
			it.Overdue = dueDate.Before(today)
			if dueDate.Equal(today) {
				it.Due = "TODAY"
			} else {
				it.Due = timefmt.Format(dueDate, "D-MMM-YY", "en")
			}
		}
		if showPriority {
			switch t.Priority {
			case 4:
				it.Priority, it.PriorityColour = bulletFilled, true
			case 3:
				it.Priority = bulletFilled
			case 2:
				it.Priority = bulletEmpty
			}
		}
		groups[t.ProjectID] = append(groups[t.ProjectID], it)
	}

	var rows []row
	for _, id := range order {
		items := groups[id]
		sort.SliceStable(items, func(i, j int) bool {
			a, b := items[i], items[j]
			if a.hasDue != b.hasDue {
				return a.hasDue
			}
			if !a.due.Equal(b.due) {
				return a.due.Before(b.due)
			}
			return a.priority > b.priority
		})
		for _, it := range items {
			rows = append(rows, it.row)
		}
	}
	return rows, nil
}

// columnWidth is the widest text plus 10 %, like Python's offsets.
func columnWidth(c *render.Canvas, rows []row, pick func(row) string) int {
	w := 0
	for _, r := range rows {
		if s := pick(r); s != "" {
			w = max(w, int(float64(c.TextWidth(s))*1.1))
		}
	}
	return w
}

func drawRows(c *render.Canvas, rows []row) {
	lh := c.LineHeight() + 1
	maxLines := c.H / lh
	top := (c.H % lh) / 2
	projW := columnWidth(c, rows, func(r row) string { return r.Project })
	dueW := columnWidth(c, rows, func(r row) string { return r.Due })
	prioW := columnWidth(c, rows, func(r row) string { return r.Priority })

	for i, r := range rows {
		if i >= maxLines {
			slog.Warn("todoist: more todos than available lines", "shown", maxLines, "total", len(rows))
			break
		}
		y := top + i*lh
		if r.Project != "" {
			c.Write(0, y, projW, lh, r.Project, render.TextOpts{Align: render.AlignLeft, Ink: render.InkColour})
		}
		if r.Due != "" {
			ink := render.InkBlack
			if r.Overdue {
				ink = render.InkColour
			}
			c.Write(projW, y, dueW, lh, r.Due, render.TextOpts{Align: render.AlignLeft, Ink: ink})
		}
		if r.Priority != "" {
			ink := render.InkBlack
			if r.PriorityColour {
				ink = render.InkColour
			}
			c.Write(projW+dueW, y, prioW, lh, r.Priority, render.TextOpts{Align: render.AlignLeft, Ink: ink})
		}
		if r.Name != "" {
			x := projW + dueW + prioW
			c.Write(x, y, c.W-x, lh, r.Name, render.TextOpts{Align: render.AlignLeft})
		}
	}
}
```

- [ ] **Step 4: View-Tests laufen lassen – müssen bestehen**

Run: `go test ./internal/modules/todoist/ -run TestBuildRows -v`
Expected: PASS (3 Tests)

- [ ] **Step 5: Failing Tests für das Modul schreiben**

`internal/modules/todoist/todoist_test.go`:

```go
package todoist

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"testing"
	"time"

	"inkycal/internal/config"
	"inkycal/internal/fetch"
	"inkycal/internal/modules"
)

func mustCommon(t *testing.T, raw string) config.Common {
	t.Helper()
	c, err := config.ParseCommon(json.RawMessage(raw))
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func TestNewRequiresAPIKey(t *testing.T) {
	common := mustCommon(t, `{"size":[480,200],"padding_x":10,"padding_y":10,"fontsize":14}`)
	if _, err := New(common, json.RawMessage(`{"project_filter":""}`)); err == nil {
		t.Error("missing api_key accepted")
	}
}

func TestGenerate(t *testing.T) {
	loc := berlin(t)
	now := time.Date(2026, 10, 10, 9, 0, 0, 0, loc)
	serveTodoist(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/projects":
			io.WriteString(w, `{"results":[{"id":"p1","name":"Inbox"}],"next_cursor":null}`)
		case "/tasks":
			io.WriteString(w, `{"results":[{"content":"Milch kaufen","project_id":"p1","priority":4,"due":{"date":"2026-10-09"}}],"next_cursor":null}`)
		default:
			http.NotFound(w, r)
		}
	})
	common := mustCommon(t, `{"size":[480,200],"padding_x":10,"padding_y":10,"fontsize":14,"language":"en"}`)
	m, err := New(common, json.RawMessage(`{"api_key":"tok","project_filter":"Inbox","show_priority":true}`))
	if err != nil {
		t.Fatal(err)
	}
	env := &modules.Env{Now: now, Loc: loc, Fetch: fetch.New("")}

	c, err := m.Generate(context.Background(), env)
	if err != nil {
		t.Fatal(err)
	}
	if c.W != 460 || c.H != 180 {
		t.Fatalf("canvas = %dx%d", c.W, c.H)
	}
	colour, black := false, false
	for y := 0; y < c.H; y++ {
		for x := 0; x < c.W; x++ {
			b, col := c.InkAt(x, y)
			black, colour = black || b, colour || col
		}
	}
	if !colour || !black {
		t.Errorf("ink black=%v colour=%v, want both (project name red, task black)", black, colour)
	}
}
```

- [ ] **Step 6: Test laufen lassen – muss fehlschlagen**

Run: `go test ./internal/modules/todoist/ -run 'TestNewRequiresAPIKey|TestGenerate' -v`
Expected: FAIL – `undefined: New`

- [ ] **Step 7: `todoist.go` implementieren**

```go
// Package todoist ports inkycal_todoist.py: open tasks grouped by project.
package todoist

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"

	"inkycal/internal/config"
	"inkycal/internal/modules"
	"inkycal/internal/render"
)

type Module struct {
	common       config.Common
	token        string
	filter       []string
	showPriority bool
}

type rawConfig struct {
	APIKey        string            `json:"api_key"`
	ProjectFilter config.StringList `json:"project_filter"`
	ShowPriority  *config.FlexBool  `json:"show_priority"`
}

func New(c config.Common, raw json.RawMessage) (modules.Module, error) {
	var rc rawConfig
	if err := json.Unmarshal(raw, &rc); err != nil {
		return nil, fmt.Errorf("todoist: config: %w", err)
	}
	if strings.TrimSpace(rc.APIKey) == "" {
		return nil, errors.New("todoist: api_key is missing")
	}
	if w, h := c.InnerSize(); w < 50 || h < 20 {
		return nil, fmt.Errorf("todoist: inner size %dx%d too small (min 50x20)", w, h)
	}
	m := &Module{common: c, token: rc.APIKey, filter: []string(rc.ProjectFilter), showPriority: true}
	if rc.ShowPriority != nil {
		m.showPriority = bool(*rc.ShowPriority)
	}
	return m, nil
}

func (m *Module) Generate(ctx context.Context, env *modules.Env) (*render.Canvas, error) {
	var (
		wg             sync.WaitGroup
		projects       []project
		tasks          []task
		staleP, staleT bool
		errP, errT     error
	)
	wg.Add(2)
	go func() {
		defer wg.Done()
		projects, staleP, errP = fetchAll[project](ctx, env.Fetch, "projects", m.token)
	}()
	go func() {
		defer wg.Done()
		tasks, staleT, errT = fetchAll[task](ctx, env.Fetch, "tasks", m.token)
	}()
	wg.Wait()
	if err := errors.Join(errP, errT); err != nil {
		return nil, err
	}
	env.Stale = env.Stale || staleP || staleT

	rows, err := buildRows(projects, tasks, m.filter, m.showPriority, env.Now)
	if err != nil {
		return nil, err
	}
	w, h := m.common.InnerSize()
	c := render.NewCanvas(w, h, render.FontDefault, int(m.common.FontSize))
	drawRows(c, rows)
	return c, nil
}
```

- [ ] **Step 8: Alle Todoist-Tests laufen lassen – müssen bestehen**

Run: `gofmt -l internal/modules/todoist/ && go test ./internal/modules/todoist/ -v`
Expected: `gofmt` gibt keine Dateien aus (sonst `gofmt -w` ausführen); PASS (7 Tests)

- [ ] **Step 9: Commit**

```bash
git add internal/modules/todoist/
git commit -m "feat(todoist): rows grouped by project, priority and overdue markers"
```

---

### Task 7: Feeds – Abruf, Bereinigung, Zeilenauswahl und Modul

**Files:**
- Modify: `go.mod`, `go.sum` (Abhängigkeit `github.com/mmcdole/gofeed v1.5.0`)
- Create: `internal/modules/feeds/feeds.go`
- Test: `internal/modules/feeds/feeds_test.go`

**Interfaces:**
- Consumes: `fetch.Client.Get(ctx context.Context, url string, header http.Header) (fetch.Result, error)` mit `Result{Body []byte; Stale bool; FetchedAt time.Time}`; `render.NewCanvas`, `Canvas.Wrap(text string, maxWidth int) []string`, `Canvas.Write`, `Canvas.LineHeight`, `Canvas.InkAt`; `config.ParseCommon`, `config.StringList`, `config.FlexBool`
- Produces:
  - `func New(c config.Common, raw json.RawMessage) (modules.Module, error)` (für Registry, Task 8)
  - paketintern: `func entry(title, description string) string`, `func entriesFromFeed(f *gofeed.Feed) []string`, `func fetchEntries(ctx context.Context, fc *fetch.Client, urls []string) ([]string, bool, error)`, `func selectLines(c *render.Canvas, entries []string, width, maxLines int) []string`, `func (m *Module) arrange(entries []string) []string`

- [ ] **Step 1: Abhängigkeit hinzufügen**

Run: `go get github.com/mmcdole/gofeed@v1.5.0`
Expected: `go.mod` enthält `github.com/mmcdole/gofeed v1.5.0`

- [ ] **Step 2: Failing Tests schreiben**

`internal/modules/feeds/feeds_test.go`:

```go
package feeds

import (
	"context"
	"encoding/json"
	"io"
	"math/rand/v2"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/mmcdole/gofeed"

	"inkycal/internal/config"
	"inkycal/internal/fetch"
	"inkycal/internal/modules"
	"inkycal/internal/render"
)

const testRSS = `<?xml version="1.0" encoding="UTF-8"?>
<rss version="2.0"><channel><title>Test</title>
<item><title>Eins</title><description>&lt;p&gt;Erster &amp;amp; bester&lt;/p&gt;</description></item>
<item><title>Zwei</title><description>Zweiter Text
mit Umbruch</description></item>
<item><title>Ohne Beschreibung</title></item>
</channel></rss>`

const testAtom = `<?xml version="1.0" encoding="UTF-8"?>
<feed xmlns="http://www.w3.org/2005/Atom"><title>A</title>
<entry><title>Atom-Titel</title><summary>Kurzfassung</summary><id>1</id><updated>2026-10-10T00:00:00Z</updated></entry>
</feed>`

func mustCommon(t *testing.T, raw string) config.Common {
	t.Helper()
	c, err := config.ParseCommon(json.RawMessage(raw))
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func TestEntry(t *testing.T) {
	cases := []struct{ title, desc, want string }{
		{"Eins", "<p>Erster &amp; bester</p>", "•Eins: Erster & bester"},
		{"Zwei", "Zweiter Text\nmit Umbruch", "•Zwei: Zweiter Text"},
		{" Drei ", "&lt;b&gt; bleibt Text", "•Drei: <b> bleibt Text"},
	}
	for _, c := range cases {
		if got := entry(c.title, c.desc); got != c.want {
			t.Errorf("entry(%q, %q) = %q, want %q", c.title, c.desc, got, c.want)
		}
	}
}

func TestEntriesFromRSSAndAtom(t *testing.T) {
	p := gofeed.NewParser()
	rss, err := p.ParseString(testRSS)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"•Eins: Erster & bester", "•Zwei: Zweiter Text"}
	if got := entriesFromFeed(rss); !slices.Equal(got, want) {
		t.Errorf("rss entries = %q, want %q", got, want)
	}
	atom, err := p.ParseString(testAtom)
	if err != nil {
		t.Fatal(err)
	}
	if got := entriesFromFeed(atom); !slices.Equal(got, []string{"•Atom-Titel: Kurzfassung"}) {
		t.Errorf("atom entries = %q", got)
	}
}

func TestFetchEntriesSkipsBrokenFeed(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/ok.xml":
			io.WriteString(w, testRSS)
		case "/garbage.xml":
			io.WriteString(w, "this is not xml")
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()
	fc := fetch.New("")

	entries, stale, err := fetchEntries(context.Background(), fc, []string{srv.URL + "/missing.xml", srv.URL + "/garbage.xml", srv.URL + "/ok.xml"})
	if err != nil {
		t.Fatal(err)
	}
	if stale || len(entries) != 2 {
		t.Errorf("entries = %q, stale = %v", entries, stale)
	}
	if _, _, err := fetchEntries(context.Background(), fc, []string{srv.URL + "/missing.xml"}); err == nil {
		t.Error("all feeds failing not reported")
	}
}

func TestSelectLines(t *testing.T) {
	c := render.NewCanvas(460, 100, render.FontDefault, 14)
	short := []string{"•A: a", "•B: b", "•C: c", "•D: d"}
	if got := selectLines(c, short, 460, 3); !slices.Equal(got, short[:3]) {
		t.Errorf("short = %q", got)
	}
	long := strings.Repeat("wort ", 200)
	if got := selectLines(c, []string{long, "•B: b"}, 460, 3); len(got) != 0 {
		t.Errorf("entry longer than the box must stop selection, got %q", got)
	}
}

func TestArrange(t *testing.T) {
	common := mustCommon(t, `{"size":[480,120],"padding_x":10,"padding_y":10,"fontsize":14}`)
	in := []string{"1", "2", "3", "4", "5", "6"}

	m, err := New(common, json.RawMessage(`{"feed_urls":"http://x/feed","shuffle_feeds":false}`))
	if err != nil {
		t.Fatal(err)
	}
	if got := m.(*Module).arrange(slices.Clone(in)); !slices.Equal(got, in) {
		t.Errorf("shuffle off changed order: %q", got)
	}

	m2, err := New(common, json.RawMessage(`{"feed_urls":"http://x/feed"}`)) // default: shuffle on
	if err != nil {
		t.Fatal(err)
	}
	mod := m2.(*Module)
	mod.rnd = rand.New(rand.NewPCG(1, 2))
	got := mod.arrange(slices.Clone(in))
	sorted := slices.Clone(got)
	slices.Sort(sorted)
	if !slices.Equal(sorted, in) {
		t.Errorf("shuffle lost or duplicated entries: %q", got)
	}
}

func TestGenerate(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, testRSS)
	}))
	defer srv.Close()
	common := mustCommon(t, `{"size":[480,120],"padding_x":10,"padding_y":10,"fontsize":14,"language":"en"}`)
	m, err := New(common, json.RawMessage(`{"feed_urls":"`+srv.URL+`/a.xml, `+srv.URL+`/b.xml","shuffle_feeds":false}`))
	if err != nil {
		t.Fatal(err)
	}
	env := &modules.Env{Now: time.Now(), Loc: time.UTC, Fetch: fetch.New("")}
	c, err := m.Generate(context.Background(), env)
	if err != nil {
		t.Fatal(err)
	}
	if c.W != 460 || c.H != 100 {
		t.Fatalf("canvas = %dx%d", c.W, c.H)
	}
	black := false
	for y := 0; y < c.H && !black; y++ {
		for x := 0; x < c.W; x++ {
			if b, _ := c.InkAt(x, y); b {
				black = true
				break
			}
		}
	}
	if !black {
		t.Error("no feed text drawn")
	}
}

func TestNewRequiresURLs(t *testing.T) {
	common := mustCommon(t, `{"size":[480,120],"padding_x":10,"padding_y":10,"fontsize":14}`)
	if _, err := New(common, json.RawMessage(`{"feed_urls":""}`)); err == nil {
		t.Error("empty feed_urls accepted")
	}
}
```

- [ ] **Step 3: Test laufen lassen – muss fehlschlagen**

Run: `go test ./internal/modules/feeds/ -v`
Expected: FAIL – `undefined: entry`

- [ ] **Step 4: `feeds.go` implementieren**

```go
// Package feeds ports inkycal_feeds.py: headlines and summaries from RSS/Atom feeds.
package feeds

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"log/slog"
	"math/rand/v2"
	"regexp"
	"strings"
	"time"

	"github.com/mmcdole/gofeed"

	"inkycal/internal/config"
	"inkycal/internal/fetch"
	"inkycal/internal/modules"
	"inkycal/internal/render"
)

type Module struct {
	common  config.Common
	urls    []string
	shuffle bool
	rnd     *rand.Rand
}

type rawConfig struct {
	FeedURLs     config.StringList `json:"feed_urls"`
	ShuffleFeeds *config.FlexBool  `json:"shuffle_feeds"`
}

var tagRe = regexp.MustCompile(`<[^<]+?>`)

func New(c config.Common, raw json.RawMessage) (modules.Module, error) {
	var rc rawConfig
	if err := json.Unmarshal(raw, &rc); err != nil {
		return nil, fmt.Errorf("feeds: config: %w", err)
	}
	if len(rc.FeedURLs) == 0 {
		return nil, errors.New("feeds: feed_urls is missing")
	}
	if w, h := c.InnerSize(); w < 50 || h < 20 {
		return nil, fmt.Errorf("feeds: inner size %dx%d too small (min 50x20)", w, h)
	}
	m := &Module{
		common:  c,
		urls:    []string(rc.FeedURLs),
		shuffle: true,
		rnd:     rand.New(rand.NewPCG(uint64(time.Now().UnixNano()), 0x9e3779b97f4a7c15)),
	}
	if rc.ShuffleFeeds != nil {
		m.shuffle = bool(*rc.ShuffleFeeds)
	}
	return m, nil
}

// entry formats one item as "•Title: summary": tags stripped before entities
// are decoded (escaped markup stays text), cut at the first line break.
func entry(title, description string) string {
	text := strings.TrimSpace(html.UnescapeString(tagRe.ReplaceAllString(description, "")))
	e := "•" + strings.TrimSpace(title) + ": " + text
	if i := strings.IndexByte(e, '\n'); i >= 0 {
		e = e[:i]
	}
	return strings.TrimRight(e, " \t\r")
}

// entriesFromFeed keeps only items with a summary, like Python's `"summary" in posts`.
func entriesFromFeed(f *gofeed.Feed) []string {
	var out []string
	for _, it := range f.Items {
		if strings.TrimSpace(it.Description) == "" {
			continue
		}
		out = append(out, entry(it.Title, it.Description))
	}
	return out
}

// fetchEntries loads all feeds; broken feeds are logged and skipped,
// an error is returned only if none could be loaded.
func fetchEntries(ctx context.Context, fc *fetch.Client, urls []string) ([]string, bool, error) {
	parser := gofeed.NewParser()
	var (
		out   []string
		stale bool
		errs  []error
		ok    int
	)
	for _, u := range urls {
		res, err := fc.Get(ctx, u, nil)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		feed, err := parser.Parse(bytes.NewReader(res.Body))
		if err != nil {
			errs = append(errs, fmt.Errorf("parse %s: %w", u, err))
			continue
		}
		ok++
		stale = stale || res.Stale
		out = append(out, entriesFromFeed(feed)...)
	}
	if ok == 0 {
		return nil, false, fmt.Errorf("feeds: no feed could be loaded: %w", errors.Join(errs...))
	}
	for _, err := range errs {
		slog.Warn("feeds: skipping feed", "err", err)
	}
	return out, stale, nil
}

// selectLines wraps entries until the next one would not fit completely.
func selectLines(c *render.Canvas, entries []string, width, maxLines int) []string {
	if len(entries) > maxLines {
		entries = entries[:maxLines]
	}
	var lines []string
	for _, e := range entries {
		wrapped := c.Wrap(e, width)
		if len(lines)+len(wrapped) > maxLines {
			break
		}
		lines = append(lines, wrapped...)
	}
	return lines
}

// arrange shuffles the entries in place when shuffle_feeds is on.
func (m *Module) arrange(entries []string) []string {
	if m.shuffle {
		m.rnd.Shuffle(len(entries), func(i, j int) { entries[i], entries[j] = entries[j], entries[i] })
	}
	return entries
}

func (m *Module) Generate(ctx context.Context, env *modules.Env) (*render.Canvas, error) {
	entries, stale, err := fetchEntries(ctx, env.Fetch, m.urls)
	if err != nil {
		return nil, err
	}
	env.Stale = env.Stale || stale
	entries = m.arrange(entries)

	w, h := m.common.InnerSize()
	c := render.NewCanvas(w, h, render.FontDefault, int(m.common.FontSize))
	const lineSpacing = 1
	lh := c.LineHeight() + lineSpacing
	maxLines := h / (lh + lineSpacing)
	top := (h % lh) / 2
	lines := selectLines(c, entries, w, maxLines)
	if len(lines) == 0 {
		slog.Info("feeds: nothing fits into the module", "entries", len(entries))
	}
	for i, line := range lines {
		c.Write(0, top+i*lh, w, lh, line, render.TextOpts{Align: render.AlignLeft})
	}
	return c, nil
}
```

- [ ] **Step 5: Tests laufen lassen – müssen bestehen**

Run: `go mod tidy && go test ./internal/modules/feeds/ -v`
Expected: PASS (7 Tests)

- [ ] **Step 6: Commit**

```bash
git add go.mod go.sum internal/modules/feeds/
git commit -m "feat(feeds): RSS/Atom module with HTML cleanup and optional shuffle"
```

---

### Task 8: Registry erweitern und End-to-End-Test über alle fünf Module

**Files:**
- Modify: `internal/app/registry.go` (komplett ersetzen, Inhalt unten)
- Create: `internal/app/modules_e2e_test.go`

**Interfaces:**
- Consumes: `weather.New`, `calendar.New`, `todoist.New`, `feeds.New` (Tasks 2, 4, 6, 7); `agenda.New` (Plan 1); `app.RunCycle(ctx context.Context, o *app.Options) (app.Result, error)`, `app.Options`, `app.Result{Refreshed bool; ModuleErrors []string; ...}`, `app.System`-Interface; `config.Load(path string) (*config.Settings, error)`; `epd.NewPNG(path string, w, h int) epd.Display`; `fetch.New(cacheDir string) *fetch.Client` mit überschreibbarem Feld `HTTP *http.Client`
- Produces: `var Registry = map[string]modules.Factory` mit allen fünf Modulen (vom CLI in Plan 1 bereits verwendet)

- [ ] **Step 1: Failing End-to-End-Test schreiben**

`internal/app/modules_e2e_test.go` (alle Hilfstypen mit Präfix `p2`, damit nichts mit Plan-1-Tests im selben Paket kollidiert):

```go
package app

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
	_ "time/tzdata"

	"inkycal/internal/config"
	"inkycal/internal/epd"
	"inkycal/internal/fetch"
)

// p2Rewrite sends every request (api.openweathermap.org, api.todoist.com, ...) to the test server.
type p2Rewrite struct{ target *url.URL }

func (t p2Rewrite) RoundTrip(r *http.Request) (*http.Response, error) {
	r2 := r.Clone(r.Context())
	r2.URL.Scheme = t.target.Scheme
	r2.URL.Host = t.target.Host
	r2.Host = t.target.Host
	return http.DefaultTransport.RoundTrip(r2)
}

type p2System struct {
	mu         sync.Mutex
	poweredOff bool
}

func (s *p2System) NTPSynced() bool       { return true }
func (s *p2System) Uptime() time.Duration { return 10 * time.Second }
func (s *p2System) Maintenance() bool     { return false }
func (s *p2System) PowerOff() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.poweredOff = true
	return nil
}

const p2ICS = "BEGIN:VCALENDAR\r\nVERSION:2.0\r\nPRODID:-//inkycal//test//EN\r\n" +
	"BEGIN:VEVENT\r\nUID:1@test\r\nDTSTAMP:20261001T000000Z\r\nDTSTART:20261010T080000Z\r\nDTEND:20261010T090000Z\r\nSUMMARY:Zahnarzt\r\nEND:VEVENT\r\n" +
	"BEGIN:VEVENT\r\nUID:2@test\r\nDTSTAMP:20261001T000000Z\r\nDTSTART;VALUE=DATE:20261012\r\nDTEND;VALUE=DATE:20261013\r\nSUMMARY:Urlaub\r\nEND:VEVENT\r\n" +
	"END:VCALENDAR\r\n"

const p2RSS = `<?xml version="1.0"?><rss version="2.0"><channel><title>T</title>
<item><title>Schlagzeile</title><description>Kurzer Text zur Meldung</description></item>
</channel></rss>`

const p2Settings = `{
  "model": "epd_7_in_5_v2_colour",
  "update_interval": 60,
  "orientation": 0,
  "info_section": true,
  "info_section_height": 70,
  "calibration_hours": [],
  "border_around_modules": true,
  "image_hash": false,
  "use_pi_sugar": false,
  "shutdown_after_run": false,
  "modules": [
    {"position": 1, "name": "Weather", "config": {"size": [480, 100], "padding_x": 10, "padding_y": 10, "fontsize": 12, "language": "de",
      "api_key": "test", "location": "Berlin,DE", "forecast_interval": "daily", "units": "metric", "hour_format": 24,
      "use_beaufort": false, "round_temperature": true, "round_windspeed": true}},
    {"position": 2, "name": "Agenda", "config": {"size": [480, 150], "padding_x": 10, "padding_y": 10, "fontsize": 12, "language": "de",
      "ical_urls": "{{BASE}}/cal.ics", "ical_files": null, "date_format": "ddd D MMM", "time_format": "HH:mm", "columns": 1}},
    {"position": 3, "name": "Calendar", "config": {"size": [480, 260], "padding_x": 10, "padding_y": 10, "fontsize": 12, "language": "de",
      "week_starts_on": "Monday", "show_events": true, "ical_urls": "{{BASE}}/cal.ics", "ical_files": null,
      "date_format": "D MMM", "time_format": "HH:mm"}},
    {"position": 4, "name": "Todoist", "config": {"size": [480, 110], "padding_x": 10, "padding_y": 10, "fontsize": 12, "language": "de",
      "api_key": "test", "project_filter": "", "show_priority": true}},
    {"position": 5, "name": "Feeds", "config": {"size": [480, 110], "padding_x": 10, "padding_y": 10, "fontsize": 12, "language": "de",
      "feed_urls": "{{BASE}}/feed.xml", "shuffle_feeds": false}}
  ]
}`

func TestRunCycleAllModules(t *testing.T) {
	loc, err := time.LoadLocation("Europe/Berlin")
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 10, 10, 9, 0, 0, 0, loc)

	var mu sync.Mutex
	hits := map[string]int{}
	mux := http.NewServeMux()
	handle := func(path, body string) {
		mux.HandleFunc(path, func(w http.ResponseWriter, r *http.Request) {
			mu.Lock()
			hits[path]++
			mu.Unlock()
			io.WriteString(w, body)
		})
	}
	handle("/data/2.5/weather", fmt.Sprintf(
		`{"weather":[{"icon":"04d"}],"main":{"temp":11.2,"temp_max":14.0,"humidity":70},"wind":{"speed":3.0},"sys":{"sunrise":%d,"sunset":%d}}`,
		now.Add(-90*time.Minute).Unix(), now.Add(9*time.Hour).Unix()))
	handle("/data/2.5/forecast", fmt.Sprintf(
		`{"list":[{"dt":%d,"main":{"temp":12},"weather":[{"icon":"10d"}]},{"dt":%d,"main":{"temp":9},"weather":[{"icon":"04d"}]}]}`,
		now.Add(24*time.Hour).Unix(), now.Add(27*time.Hour).Unix()))
	handle("/api/v1/projects", `{"results":[{"id":"p1","name":"Inbox"}],"next_cursor":null}`)
	handle("/api/v1/tasks", `{"results":[{"content":"Milch kaufen","project_id":"p1","priority":4,"due":{"date":"2026-10-10"}}],"next_cursor":null}`)
	handle("/cal.ics", p2ICS)
	handle("/feed.xml", p2RSS)
	srv := httptest.NewServer(mux)
	defer srv.Close()
	target, err := url.Parse(srv.URL)
	if err != nil {
		t.Fatal(err)
	}

	dir := t.TempDir()
	settingsPath := filepath.Join(dir, "settings.json")
	if err := os.WriteFile(settingsPath, []byte(strings.ReplaceAll(p2Settings, "{{BASE}}", srv.URL)), 0o600); err != nil {
		t.Fatal(err)
	}
	settings, err := config.Load(settingsPath)
	if err != nil {
		t.Fatal(err)
	}
	fc := fetch.New(filepath.Join(dir, "http"))
	fc.HTTP = &http.Client{Transport: p2Rewrite{target: target}, Timeout: 10 * time.Second}
	preview := filepath.Join(dir, "preview.png")
	sys := &p2System{}

	res, err := RunCycle(context.Background(), &Options{
		Settings: settings,
		StateDir: dir,
		Display:  epd.NewPNG(preview, 800, 480),
		System:   sys,
		Fetch:    fc,
		Loc:      loc,
		Now:      func() time.Time { return now },
		Registry: Registry,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.ModuleErrors) != 0 {
		t.Fatalf("module errors: %v", res.ModuleErrors)
	}
	if !res.Refreshed {
		t.Error("first cycle did not refresh the display")
	}
	if info, err := os.Stat(preview); err != nil || info.Size() == 0 {
		t.Errorf("preview PNG missing or empty: %v", err)
	}
	for _, path := range []string{"/data/2.5/weather", "/data/2.5/forecast", "/api/v1/projects", "/api/v1/tasks", "/cal.ics", "/feed.xml"} {
		if hits[path] == 0 {
			t.Errorf("%s was never requested", path)
		}
	}
	if sys.poweredOff {
		t.Error("PowerOff called although use_pi_sugar is false")
	}
}
```

- [ ] **Step 2: Test laufen lassen – muss fehlschlagen**

Run: `go test ./internal/app/ -run TestRunCycleAllModules -v`
Expected: FAIL – `module errors: [...unknown module "Weather"...]` (bzw. die Fehlermeldung, die Plan 1 für unbekannte Module erzeugt)

- [ ] **Step 3: `internal/app/registry.go` komplett ersetzen**

```go
package app

import (
	"inkycal/internal/modules"
	"inkycal/internal/modules/agenda"
	"inkycal/internal/modules/calendar"
	"inkycal/internal/modules/feeds"
	"inkycal/internal/modules/todoist"
	"inkycal/internal/modules/weather"
)

// Registry maps the module names used in settings.json to their factories.
var Registry = map[string]modules.Factory{
	"Agenda":   agenda.New,
	"Calendar": calendar.New,
	"Feeds":    feeds.New,
	"Todoist":  todoist.New,
	"Weather":  weather.New,
}
```

- [ ] **Step 4: Test laufen lassen – muss bestehen**

Run: `go test ./internal/app/ -run TestRunCycleAllModules -v`
Expected: PASS

- [ ] **Step 5: Gesamte Suite, Vet und Cross-Compile prüfen**

Run: `gofmt -l . && go vet ./... && go test ./... && CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -o /dev/null ./cmd/inkycal && CGO_ENABLED=0 GOOS=linux GOARCH=arm GOARM=7 go build -o /dev/null ./cmd/inkycal`
Expected: `gofmt` ohne Ausgabe, `go vet` ohne Befunde, alle Tests PASS, beide Builds ohne Fehler

- [ ] **Step 6: Dry-Run-Vorschau manuell prüfen**

Run: `go run ./cmd/inkycal --mode dry-run --settings <eigene settings.json mit echten Keys> --out /tmp/inkycal-preview.png`
Expected: PNG zeigt Weather oben, darunter Agenda, Calendar (heutiger Tag als roter Kreis), Todoist (Projektname rot), Feeds; Info-Zeile unten. Abweichungen zum Python-Layout notieren, nicht pixelgenau angleichen.

- [ ] **Step 7: Commit**

```bash
git add internal/app/registry.go internal/app/modules_e2e_test.go
git commit -m "feat(app): register weather, calendar, todoist and feeds modules"
```

---

## Bewusste Abweichungen von Python (für Review)

| Stelle | Python | Go |
|---|---|---|
| Unbekannter OWM-Icon-Code | `KeyError`, Modul fehlt | Glyphe „n/a“ |
| Beaufort-Anzeige | `4.0 bft` | `4 bft` |
| Monatsraster: Ganztagstermin | Folgetag mitmarkiert | Ende exklusiv |
| Monatsraster: Termin aus Vormonat | gleiche Tageszahl im aktuellen Monat markiert | nur Tage des angezeigten Monats |
| Calendar iCal-Abruf | zwei Parsedurchläufe | ein `ical.Load` über Monat ∪ 4 Wochen |
| Todoist `project_filter` | Split ohne Trim (`" Work"` ≠ `"Work"`) | `StringList` trimmt |
| Todoist Offline | eigene JSON-Cache-Datei + Fehlerbild | `fetch.Client`-Cache, `env.Stale`; ohne Cache Modulfehler (Info-Zeile) |
| Feeds Entities | `&amp;` bleibt stehen | wird zu `&` |
| Feeds defekte URL | feedparser liefert still leer | wird übersprungen und geloggt; Fehler nur wenn alle scheitern |
| Netzwerkabrufe | sequentiell, vorher `internet_available()` | parallel, kein Vorab-Check |
