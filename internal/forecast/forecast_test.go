package forecast

import (
	"context"
	"io"
	"net/http"
	"testing"
	"time"

	"inkycal/internal/fetch/fetchtest"
	"inkycal/internal/forecast/forecasttest"
)

// A UV index from the cache can be yesterday noon's: offline, the report keeps the
// cached weather but drops the UV index, from either service.
func TestStaleReportHasNoUVIndex(t *testing.T) {
	loc := forecasttest.Berlin(t)
	now := time.Date(2026, 10, 13, 6, 0, 0, 0, loc)
	down := false
	fc := fetchtest.Client(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if down {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		switch r.URL.Path {
		case "/data/2.5/weather":
			io.WriteString(w, forecasttest.OWMCurrent("01d", "", now, now.Add(10*time.Hour)))
		case "/data/2.5/forecast":
			io.WriteString(w, `{"list":[{"dt":1791000000,"main":{"temp":3},"weather":[{"icon":"01d"}]}]}`)
		case "/data/3.0/onecall":
			io.WriteString(w, `{"current":{"uvi":2.5}}`)
		case "/v1/forecast":
			io.WriteString(w, forecasttest.OpenMeteo(now, 12))
		}
	}))
	sources := map[string]Settings{
		"OpenWeatherMap": {APIKey: "k", Latitude: ptr(50.9), Longitude: ptr(6.9)},
		"Open-Meteo":     {Latitude: ptr(50.9), Longitude: ptr(6.9)},
	}
	for name, s := range sources {
		src, err := s.Source("en", Want{UV: true, OneCall: true})
		if err != nil {
			t.Fatal(err)
		}
		down = false
		if r, err := src.Fetch(context.Background(), fc, loc); err != nil || r.Stale || r.UV == nil {
			t.Fatalf("%s online: err %v stale %v uv %v", name, err, r.Stale, r.UV)
		}
		down = true
		r, err := src.Fetch(context.Background(), fc, loc)
		if err != nil || !r.Stale || len(r.Points) == 0 {
			t.Fatalf("%s offline: err %v stale %v points %d, want the cached weather", name, err, r.Stale, len(r.Points))
		}
		if r.UV != nil {
			t.Errorf("%s offline: UV %v from the cache, want none", name, *r.UV)
		}
	}
}
