package forecast

import (
	"inkycal/internal/forecast/forecasttest"
	"testing"
	"time"
)

func TestBeaufort(t *testing.T) {
	cases := []struct {
		mps  float64
		want int
	}{{0, 0}, {0.29, 0}, {0.3, 1}, {1.5, 1}, {5.5, 4}, {7.9, 4}, {32.6, 11}, {40, 12}}
	for _, c := range cases {
		if got := Beaufort(c.mps); got != c.want {
			t.Errorf("Beaufort(%v) = %d, want %d", c.mps, got, c.want)
		}
	}
}

func TestToFahrenheit(t *testing.T) {
	if Fahrenheit(0) != 32 || Fahrenheit(100) != 212 {
		t.Errorf("Fahrenheit wrong: %v %v", Fahrenheit(0), Fahrenheit(100))
	}
}

func TestIconGlyph(t *testing.T) {
	for code, want := range map[string]string{"01d": "\uf00d", "04d": "\uf013", "09n": "\uf019", "10d": "\uf008", "13n": "\uf01b"} {
		if got := Glyph(code); got != want {
			t.Errorf("%s = %U, want %U", code, []rune(got)[0], []rune(want)[0])
		}
	}
	if got := Glyph("77z"); got != GlyphNA {
		t.Errorf("unknown code = %q, want GlyphNA", got)
	}
}

func TestWindLevel(t *testing.T) {
	for _, c := range []struct {
		mps  float64
		want int
	}{{0, 1}, {5.4, 1}, {5.5, 2}, {10.7, 2}, {10.8, 3}, {25, 3}} {
		if got := WindLevel(c.mps); got != c.want {
			t.Errorf("WindLevel(%v) = %d, want %d", c.mps, got, c.want)
		}
	}
}

func TestDayIconPrefersPrecipitationOverMajority(t *testing.T) {
	loc := forecasttest.Berlin(t)
	at := func(h int, icon string) Point {
		return Point{Time: time.Date(2026, 10, 13, h, 0, 0, 0, loc), Icon: icon}
	}
	now := time.Date(2026, 10, 13, 6, 0, 0, 0, loc)
	sunny := []Point{at(8, "01d"), at(11, "01d"), at(14, "02d"), at(17, "01d")}
	if got := DayIcon(sunny, "04n", now); got != "01d" {
		t.Errorf("sunny day = %s", got)
	}
	shower := append(sunny, at(20, "10n"))
	if got := DayIcon(shower, "04n", now); got != "10d" {
		t.Errorf("day with an evening shower = %s, want 10d", got)
	}
	storm := []Point{at(8, "10d"), at(11, "11d"), at(14, "10d")}
	if got := DayIcon(storm, "01d", now); got != "11d" {
		t.Errorf("thunderstorm = %s", got)
	}
	night := time.Date(2026, 10, 13, 22, 30, 0, 0, loc)
	if got := DayIcon(sunny, "04n", night); got != "04d" {
		t.Errorf("no daytime slots left: %s, want current icon as day variant", got)
	}
	// Tomorrow's slots and slots before 06:00 do not count.
	other := []Point{at(3, "13n"), {Time: time.Date(2026, 10, 14, 12, 0, 0, 0, loc), Icon: "11d"}, at(12, "03d")}
	if got := DayIcon(other, "01d", now); got != "03d" {
		t.Errorf("filtering = %s", got)
	}
}
