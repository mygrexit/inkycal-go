package forecast

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"
	_ "time/tzdata"

	"inkycal/internal/fetch"
	"inkycal/internal/fetch/fetchtest"
)

func berlin(t *testing.T) *time.Location {
	t.Helper()
	loc, err := time.LoadLocation("Europe/Berlin")
	if err != nil {
		t.Fatal(err)
	}
	return loc
}

// serveOWM answers OpenWeatherMap's /weather and /forecast.
func serveOWM(t *testing.T, currentBody, forecastBody string) *fetch.Client {
	t.Helper()
	return fetchtest.Client(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Query().Get("units"); got != "metric" {
			t.Errorf("units = %q, want metric", got)
		}
		switch r.URL.Path {
		case "/data/2.5/weather":
			io.WriteString(w, currentBody)
		case "/data/2.5/forecast":
			io.WriteString(w, forecastBody)
		default:
			http.NotFound(w, r)
		}
	}))
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

func TestOWMURL(t *testing.T) {
	u, err := url.Parse(owm{apiKey: "key", place: "4893171", lang: "de"}.url("weather"))
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
	u2, err := url.Parse(owm{apiKey: "key", place: " Berlin, DE ", lang: "en"}.url("forecast"))
	if err != nil {
		t.Fatal(err)
	}
	if u2.Query().Get("q") != "Berlin, DE" || u2.Query().Has("id") {
		t.Errorf("name location: query = %v", u2.Query())
	}
	// without a place: by coordinates (upstream Fullweather)
	u3, err := url.Parse(owm{apiKey: "key", lat: 50.94, lon: -6.5, lang: "de"}.url("forecast"))
	if err != nil {
		t.Fatal(err)
	}
	if q := u3.Query(); u3.Path != "/data/2.5/forecast" || q.Get("lat") != "50.94" || q.Get("lon") != "-6.5" || q.Has("q") || q.Has("id") {
		t.Errorf("coordinates: url = %s", u3)
	}
}

func TestFetchCurrentUsesGust(t *testing.T) {
	loc := berlin(t)
	sunrise := time.Date(2026, 10, 10, 7, 30, 0, 0, loc)
	sunset := time.Date(2026, 10, 10, 18, 45, 0, 0, loc)
	fc := serveOWM(t, currentFixture("04d", "7.9", sunrise, sunset), `{"list":[]}`)

	cur, stale, err := getCurrent(context.Background(), fc, owm{apiKey: "k", place: "Berlin"}.url("weather"), loc)
	if err != nil {
		t.Fatal(err)
	}
	if stale {
		t.Error("stale without cache")
	}
	if cur.WindMPS != 7.9 {
		t.Errorf("WindMPS = %v, want gust 7.9", cur.WindMPS)
	}
	if cur.Icon != "04d" || cur.Temp != -1.6 || cur.Humidity != 81 {
		t.Errorf("current = %+v", cur)
	}
	if !cur.Sunrise.Equal(sunrise) || cur.Sunrise.Location() != loc || cur.Sunset.Hour() != 18 {
		t.Errorf("sunrise/sunset = %v / %v", cur.Sunrise, cur.Sunset)
	}
}

func TestFetchCurrentFallsBackToSpeed(t *testing.T) {
	loc := berlin(t)
	now := time.Date(2026, 10, 10, 9, 0, 0, 0, loc)
	fc := serveOWM(t, currentFixture("01d", "", now, now), `{"list":[]}`)

	cur, _, err := getCurrent(context.Background(), fc, owm{apiKey: "k", place: "Berlin"}.url("weather"), loc)
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
	fc := serveOWM(t, `{}`, body)

	points, _, err := getForecast(context.Background(), fc, owm{apiKey: "k", place: "Berlin"}.url("forecast"), loc)
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

// The fields Fullweather needs: description, feels-like, gusts and rain + snow; and the
// UV index from One Call 3.0 only when the key may use it.
func TestOWMReport(t *testing.T) {
	loc := berlin(t)
	t1 := time.Date(2026, 10, 10, 12, 0, 0, 0, loc)
	fc := fetchtest.Client(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/data/2.5/weather":
			io.WriteString(w, `{"weather":[{"icon":"13d","description":"light snow"}],"main":{"temp":-1,"feels_like":-4.5},"sys":{"sunrise":0,"sunset":0}}`)
		case "/data/2.5/forecast":
			fmt.Fprintf(w, `{"list":[{"dt":%d,"main":{"temp":3},"weather":[{"icon":"13d"}],"wind":{"speed":2,"gust":5.5},"rain":{"3h":0.5},"snow":{"3h":1.25}},`+
				`{"dt":%d,"main":{"temp":4},"weather":[{"icon":"03d"}],"wind":{"speed":2}}]}`, t1.Unix(), t1.Add(3*time.Hour).Unix())
		case "/data/3.0/onecall":
			io.WriteString(w, `{"current":{"uvi":2.5}}`)
		}
	}))
	for _, oneCall := range []bool{false, true} {
		src, err := Settings{APIKey: "k", Latitude: ptr(50.9), Longitude: ptr(6.9)}.Source("en", Want{UV: true, OneCall: oneCall})
		if err != nil {
			t.Fatal(err)
		}
		r, err := src.Fetch(context.Background(), fc, loc)
		if err != nil {
			t.Fatal(err)
		}
		if r.Now.Description != "light snow" || r.Now.FeelsLike != -4.5 || !r.Now.Sunrise.IsZero() {
			t.Errorf("current = %+v (no sun times in polar night)", r.Now)
		}
		p := r.Points
		if len(p) != 2 || p[0].Gust != 5.5 || p[0].Precip != 1.75 || p[1].Gust != 2 || p[1].Precip != 0 {
			t.Errorf("points = %+v", p)
		}
		if (r.UV != nil) != oneCall || (oneCall && *r.UV != 2.5) {
			t.Errorf("one call %v: uv %v", oneCall, r.UV)
		}
	}
}
