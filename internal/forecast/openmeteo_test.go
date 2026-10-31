package forecast

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"inkycal/internal/config"
	"inkycal/internal/fetch"
	"inkycal/internal/fetch/fetchtest"
	"inkycal/internal/forecast/forecasttest"
	"inkycal/internal/problem"
)

func serveOpenMeteo(t *testing.T, body string) (*fetch.Client, *http.Request) {
	t.Helper()
	var seen http.Request
	fc := fetchtest.Client(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen = *r
		w.Write([]byte(body))
	}))
	return fc, &seen
}

func TestOpenMeteoMapsToTheSameModel(t *testing.T) {
	loc := forecasttest.Berlin(t)
	start := time.Date(2026, 10, 13, 6, 0, 0, 0, loc)
	fc, req := serveOpenMeteo(t, forecasttest.OpenMeteo(start, 12))
	r, err := openMeteo{lat: 52.52, lon: 13.41, lang: "de"}.fetch(context.Background(), fc, loc, Want{})
	if err != nil || r.Stale || r.UV != nil {
		t.Fatalf("err %v stale %v uv %v", err, r.Stale, r.UV)
	}
	q := req.URL.Query()
	if req.URL.Path != "/v1/forecast" || q.Get("latitude") != "52.52" || q.Get("wind_speed_unit") != "ms" || q.Get("timeformat") != "unixtime" ||
		q.Get("forecast_days") != "3" || strings.Contains(q.Get("current"), "uv_index") {
		t.Errorf("query %v", q)
	}
	cur, points := r.Now, r.Points
	want := Current{Icon: "10d", Description: "Leichter Regen", Temp: 4.2, FeelsLike: 1.5, Humidity: 81,
		WindMPS: 7.9, WindMean: 3.1, Sunrise: time.Date(2026, 10, 13, 7, 40, 0, 0, loc), Sunset: time.Date(2026, 10, 13, 18, 30, 0, 0, loc)}
	if !cur.Sunrise.Equal(want.Sunrise) || !cur.Sunset.Equal(want.Sunset) {
		t.Errorf("sun %v %v", cur.Sunrise, cur.Sunset)
	}
	cur.Sunrise, cur.Sunset, want.Sunrise, want.Sunset = time.Time{}, time.Time{}, time.Time{}, time.Time{}
	if cur != want {
		t.Errorf("current\n got %+v\nwant %+v", cur, want)
	}
	// 3-hour slots on local hours 06, 09, 12, 15; rain summed over the three hours ending at the slot
	if len(points) != 4 || points[1].Time.Hour() != 9 || points[1].Precip != 1.5 || points[0].Precip != 0 {
		t.Fatalf("points %+v", points)
	}
	if points[1].Icon != "10d" || points[2].Icon != "04d" || points[1].Gust != 5.5 || points[1].Wind != 2.0 {
		t.Errorf("slot 09:00 %+v / 12:00 %+v", points[1], points[2])
	}
}

// Fullweather asks for five days and the UV index, which Open-Meteo has for free.
func TestOpenMeteoDaysAndUV(t *testing.T) {
	loc := forecasttest.Berlin(t)
	fc, req := serveOpenMeteo(t, forecasttest.OpenMeteo(time.Date(2026, 10, 13, 6, 0, 0, 0, loc), 5*24))
	r, err := openMeteo{lat: 52.52, lon: 13.41, lang: "en"}.fetch(context.Background(), fc, loc, Want{Days: 5, UV: true})
	if err != nil {
		t.Fatal(err)
	}
	if q := req.URL.Query(); q.Get("forecast_days") != "5" || !strings.HasSuffix(q.Get("current"), ",uv_index") {
		t.Errorf("query %v", q)
	}
	if r.UV == nil || *r.UV != 3.4 {
		t.Errorf("uv %v", r.UV)
	}
}

func TestWMOIcons(t *testing.T) {
	cases := map[int]string{0: "01", 1: "02", 2: "03", 3: "04", 45: "50", 53: "09", 63: "10", 66: "13", 73: "13", 81: "09", 86: "13", 95: "11", 99: "11"}
	for code, want := range cases {
		if got := wmoIcon(code, true); got != want+"d" {
			t.Errorf("wmo %d = %s, want %sd", code, got, want)
		}
	}
	if wmoIcon(0, false) != "01n" || wmoText(61, "en") != "Light rain" || wmoText(61, "de_DE") != "Leichter Regen" {
		t.Error("night icon or texts")
	}
}

func ptr(v float64) *config.FlexFloat { f := config.FlexFloat(v); return &f }

// Upstream settings keep their service; new ones without a key get Open-Meteo.
func TestSourceChoosesTheService(t *testing.T) {
	cases := []struct {
		s    Settings
		want string // provider type, or the error kind's key
	}{
		{Settings{APIKey: "k", Location: "Berlin"}, "owm"},
		{Settings{APIKey: "k", Latitude: ptr(50.9), Longitude: ptr(6.9)}, "owm"}, // upstream Fullweather
		{Settings{Latitude: ptr(52.5), Longitude: ptr(13.4), Location: "Berlin"}, "openMeteo"},
		{Settings{Provider: OpenMeteo, Location: "Berlin"}, "latitude and longitude are missing"},
		{Settings{Provider: OpenWeatherMap, Location: "Berlin"}, "api_key is missing"},
		{Settings{Provider: OpenWeatherMap, APIKey: "k"}, "location is missing"},
		{Settings{Provider: "met.no"}, `provider "met.no"`},
		{Settings{Latitude: ptr(91), Longitude: ptr(0)}, "out of range"},
		{Settings{APIKey: "k", Location: "Berlin", Latitude: ptr(91), Longitude: ptr(0)}, "owm"}, // by name: coordinates unused
	}
	for _, c := range cases {
		src, err := c.s.Source("en", Want{})
		got := fmt.Sprintf("%T", src.p)
		if err != nil {
			got = err.Error()
			if k := problem.Classify(err); k != problem.Missing && k != problem.BadSetting {
				t.Errorf("%+v: kind %v", c.s, k)
			}
		}
		if !strings.Contains(got, c.want) {
			t.Errorf("%+v: %s, want %s", c.s, got, c.want)
		}
	}
}

// Fullweather goes by coordinates (Want.ByCoords) and reports in its old order:
// api_key, then the coordinates, then their range.
func TestSourceByCoords(t *testing.T) {
	cases := []struct {
		s    Settings
		want string
	}{
		{Settings{Provider: OpenWeatherMap}, "api_key is missing"},
		{Settings{APIKey: "k", Location: "Köln"}, "latitude and longitude are missing"},
		{Settings{APIKey: "k", Latitude: ptr(95), Longitude: ptr(1)}, "out of range"},
		{Settings{Latitude: ptr(1)}, "latitude and longitude are missing"},
	}
	for _, c := range cases {
		if _, err := c.s.Source("en", Want{ByCoords: true}); err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("%+v: %v, want %q", c.s, err, c.want)
		}
	}
	src, err := Settings{APIKey: "k", Location: "Köln", Latitude: ptr(50.9), Longitude: ptr(6.9)}.Source("en", Want{ByCoords: true})
	if err != nil {
		t.Fatal(err)
	}
	if o := src.p.(owm); o.place != "" || !o.hasCoords {
		t.Errorf("by coordinates: %+v", o)
	}
}
