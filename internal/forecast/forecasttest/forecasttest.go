// Package forecasttest serves weather answers to tests of the weather views.
package forecasttest

import (
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
	_ "time/tzdata"

	"inkycal/internal/fetch"
	"inkycal/internal/fetch/fetchtest"
)

// Berlin is the time zone the fixtures are written for.
func Berlin(t testing.TB) *time.Location {
	t.Helper()
	loc, err := time.LoadLocation("Europe/Berlin")
	if err != nil {
		t.Fatal(err)
	}
	return loc
}

// OWM answers OpenWeatherMap's /weather and /forecast with these bodies.
func OWM(t testing.TB, currentBody, forecastBody string) *fetch.Client {
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

// OWMCurrent builds a /weather body; gust "" omits the gust field.
func OWMCurrent(icon, gust string, sunrise, sunset time.Time) string {
	wind := `{"speed":3.1}`
	if gust != "" {
		wind = `{"speed":3.1,"gust":` + gust + `}`
	}
	return fmt.Sprintf(`{"weather":[{"icon":%q}],"main":{"temp":-1.6,"temp_max":2.4,"humidity":81},"wind":%s,"sys":{"sunrise":%d,"sunset":%d}}`,
		icon, wind, sunrise.Unix(), sunset.Unix())
}

// OpenMeteo is an Open-Meteo answer (timeformat=unixtime) of hours hourly values from
// start: light rain (0.5 mm) in the hours 1–3 of every 12, overcast otherwise, wind
// 2 m/s with 5.5 m/s gusts; now 4.2° in light rain, UV 3.4; sunrise 07:40, sunset 18:30.
func OpenMeteo(start time.Time, hours int) string {
	var times, temps, codes, precip, wind, gust, isDay, rises, sets []string
	for h := range hours {
		at := start.Add(time.Duration(h) * time.Hour)
		times = append(times, fmt.Sprint(at.Unix()))
		temps = append(temps, fmt.Sprintf("%.1f", 4+float64(h%12)/2))
		code, p := "3", "0"
		if h%12 >= 1 && h%12 <= 3 {
			code, p = "61", "0.5"
		}
		codes, precip = append(codes, code), append(precip, p)
		wind, gust, isDay = append(wind, "2.0"), append(gust, "5.5"), append(isDay, "1")
	}
	day := time.Date(start.Year(), start.Month(), start.Day(), 0, 0, 0, 0, start.Location())
	for d := 0; !day.AddDate(0, 0, d).After(start.Add(time.Duration(hours) * time.Hour)); d++ {
		rises = append(rises, fmt.Sprint(day.AddDate(0, 0, d).Add(7*time.Hour+40*time.Minute).Unix()))
		sets = append(sets, fmt.Sprint(day.AddDate(0, 0, d).Add(18*time.Hour+30*time.Minute).Unix()))
	}
	j := func(s []string) string { return "[" + strings.Join(s, ",") + "]" }
	return fmt.Sprintf(`{"current":{"time":%d,"temperature_2m":4.2,"relative_humidity_2m":81,"apparent_temperature":1.5,"is_day":1,"weather_code":61,"wind_speed_10m":3.1,"wind_gusts_10m":7.9,"uv_index":3.4},
"hourly":{"time":%s,"temperature_2m":%s,"weather_code":%s,"precipitation":%s,"wind_speed_10m":%s,"wind_gusts_10m":%s,"is_day":%s},
"daily":{"temperature_2m_max":[9.5],"sunrise":%s,"sunset":%s}}`,
		start.Unix(), j(times), j(temps), j(codes), j(precip), j(wind), j(gust), j(isDay), j(rises), j(sets))
}
