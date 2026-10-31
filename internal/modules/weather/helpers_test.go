package weather

import (
	"fmt"
	"io"
	"net/http"
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
