package weather

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"inkycal/internal/fetch"
	"inkycal/internal/modules"
	"inkycal/internal/modules/modulestest"
)

// omFixture is an Open-Meteo answer (timeformat=unixtime) starting at 06:00 local time,
// 12 hourly values; rain 0.5 mm in the hours ending 07:00, 08:00 and 09:00.
func omFixture(start time.Time) string {
	var times, temps, codes, precip, wind, gust, isDay []string
	for h := range 12 {
		at := start.Add(time.Duration(h) * time.Hour)
		times = append(times, fmt.Sprint(at.Unix()))
		temps = append(temps, fmt.Sprintf("%.1f", 4+float64(h)/2))
		code := "3"
		if h >= 1 && h <= 3 {
			code = "61"
		}
		codes = append(codes, code)
		p := "0"
		if h >= 1 && h <= 3 {
			p = "0.5"
		}
		precip = append(precip, p)
		wind = append(wind, "2.0")
		gust = append(gust, "5.5")
		isDay = append(isDay, "1")
	}
	j := func(s []string) string { return "[" + strings.Join(s, ",") + "]" }
	day := time.Date(start.Year(), start.Month(), start.Day(), 0, 0, 0, 0, start.Location())
	return fmt.Sprintf(`{"current":{"time":%d,"temperature_2m":4.2,"relative_humidity_2m":81,"apparent_temperature":1.5,"is_day":1,"weather_code":61,"wind_speed_10m":3.1,"wind_gusts_10m":7.9},
"hourly":{"time":%s,"temperature_2m":%s,"weather_code":%s,"precipitation":%s,"wind_speed_10m":%s,"wind_gusts_10m":%s,"is_day":%s},
"daily":{"time":[%d],"temperature_2m_max":[9.5],"sunrise":[%d],"sunset":[%d]}}`,
		start.Unix(), j(times), j(temps), j(codes), j(precip), j(wind), j(gust), j(isDay),
		day.Unix(), day.Add(7*time.Hour+40*time.Minute).Unix(), day.Add(18*time.Hour+30*time.Minute).Unix())
}

func serveOpenMeteo(t *testing.T, body string) *http.Request {
	t.Helper()
	var seen http.Request
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen = *r
		w.Write([]byte(body))
	}))
	old := openMeteoURL
	openMeteoURL = srv.URL + "/v1/forecast"
	t.Cleanup(func() { openMeteoURL = old; srv.Close() })
	return &seen
}

func TestOpenMeteoMapsToTheSameModel(t *testing.T) {
	loc := berlin(t)
	start := time.Date(2026, 10, 13, 6, 0, 0, 0, loc)
	req := serveOpenMeteo(t, omFixture(start))
	cur, points, stale, err := fetchOpenMeteo(context.Background(), fetch.New(""), 52.52, 13.41, "de", loc)
	if err != nil || stale {
		t.Fatalf("err %v stale %v", err, stale)
	}
	if q := req.URL.Query(); q.Get("latitude") != "52.52" || q.Get("wind_speed_unit") != "ms" || q.Get("timeformat") != "unixtime" {
		t.Errorf("query %v", q)
	}
	want := current{Icon: "10d", Description: "Leichter Regen", Temp: 4.2, FeelsLike: 1.5, MaxTemp: 9.5, Humidity: 81,
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

func TestProviderChoice(t *testing.T) {
	common := modulestest.Common(t, `{"size":[984,290],"padding_x":40,"padding_y":20,"fontsize":20,"language":"de"}`)
	cases := []struct {
		raw, provider, err string
	}{
		{`{"api_key":"k","location":"Berlin"}`, "openweathermap", ""},                // existing settings stay
		{`{"latitude":52.5,"longitude":13.4,"location":"Berlin"}`, "open-meteo", ""}, // no key: Open-Meteo
		{`{"provider":"open-meteo","location":"Berlin"}`, "", "latitude"},
		{`{"provider":"openweathermap","location":"Berlin"}`, "", "api_key"},
	}
	for _, c := range cases {
		m, err := Def.New(common, json.RawMessage(c.raw))
		if c.err != "" {
			if err == nil || !strings.Contains(err.Error(), c.err) {
				t.Errorf("%s: err %v, want %q", c.raw, err, c.err)
			}
			continue
		}
		if err != nil || m.(*Module).provider != c.provider {
			t.Errorf("%s: err %v provider %q", c.raw, err, m.(*Module).provider)
		}
	}
}

func TestGenerateWithOpenMeteo(t *testing.T) {
	loc := berlin(t)
	now := time.Date(2026, 10, 13, 6, 5, 0, 0, loc)
	serveOpenMeteo(t, omFixture(time.Date(2026, 10, 13, 6, 0, 0, 0, loc)))
	common := modulestest.Common(t, `{"size":[984,290],"padding_x":40,"padding_y":20,"fontsize":20,"language":"de"}`)
	m, err := Def.New(common, json.RawMessage(`{"latitude":52.52,"longitude":13.41,"location":"Berlin"}`))
	if err != nil {
		t.Fatal(err)
	}
	c, err := m.Generate(context.Background(), &modules.Env{Now: now, Loc: loc, Fetch: fetch.New("")})
	if err != nil {
		t.Fatal(err)
	}
	if _, colour := modulestest.InkIn(c, 0, 0, 280, 125); !colour {
		t.Error("date numeral missing")
	}
}
