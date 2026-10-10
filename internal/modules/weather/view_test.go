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
