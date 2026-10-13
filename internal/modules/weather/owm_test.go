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
