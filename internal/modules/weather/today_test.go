package weather

import (
	"testing"
	"time"
)

func TestWindLevel(t *testing.T) {
	for _, c := range []struct {
		mps  float64
		want int
	}{{0, 1}, {5.4, 1}, {5.5, 2}, {10.7, 2}, {10.8, 3}, {25, 3}} {
		if got := windLevel(c.mps); got != c.want {
			t.Errorf("windLevel(%v) = %d, want %d", c.mps, got, c.want)
		}
	}
}

func TestDayIconPrefersPrecipitationOverMajority(t *testing.T) {
	loc := berlin(t)
	at := func(h int, icon string) forecastPoint {
		return forecastPoint{Time: time.Date(2026, 10, 13, h, 0, 0, 0, loc), Icon: icon}
	}
	now := time.Date(2026, 10, 13, 6, 0, 0, 0, loc)
	sunny := []forecastPoint{at(8, "01d"), at(11, "01d"), at(14, "02d"), at(17, "01d")}
	if got := dayIcon(sunny, "04n", now); got != "01d" {
		t.Errorf("sunny day = %s", got)
	}
	shower := append(sunny, at(20, "10n"))
	if got := dayIcon(shower, "04n", now); got != "10d" {
		t.Errorf("day with an evening shower = %s, want 10d", got)
	}
	storm := []forecastPoint{at(8, "10d"), at(11, "11d"), at(14, "10d")}
	if got := dayIcon(storm, "01d", now); got != "11d" {
		t.Errorf("thunderstorm = %s", got)
	}
	night := time.Date(2026, 10, 13, 22, 30, 0, 0, loc)
	if got := dayIcon(sunny, "04n", night); got != "04d" {
		t.Errorf("no daytime slots left: %s, want current icon as day variant", got)
	}
	// Tomorrow's slots and slots before 06:00 do not count.
	other := []forecastPoint{at(3, "13n"), {Time: time.Date(2026, 10, 14, 12, 0, 0, 0, loc), Icon: "11d"}, at(12, "03d")}
	if got := dayIcon(other, "01d", now); got != "03d" {
		t.Errorf("filtering = %s", got)
	}
}

func TestBuildDay(t *testing.T) {
	loc := berlin(t)
	now := time.Date(2026, 10, 13, 6, 5, 0, 0, loc)
	cur := current{Icon: "04n", Temp: 7.6, WindMean: 3, Sunrise: time.Date(2026, 10, 13, 7, 54, 0, 0, loc), Sunset: time.Date(2026, 10, 13, 18, 48, 0, 0, loc)}
	var pts []forecastPoint
	temps := []float64{8, 11, 14.4, 13, 10, 9, 6}
	for i, h := range []int{8, 11, 14, 17, 20, 23} {
		pts = append(pts, forecastPoint{Time: time.Date(2026, 10, 13, h, 0, 0, 0, loc), Temp: temps[i], Icon: "03d", Wind: 4 + float64(i)})
	}
	pts = append(pts, forecastPoint{Time: time.Date(2026, 10, 14, 2, 0, 0, 0, loc), Temp: -3, Icon: "13n", Wind: 30})

	v := buildDay(settings{Lang: "de"}, cur, pts, now)
	if v.Hi != "14°" || v.Lo != "8°" {
		t.Errorf("hi/lo = %s/%s (tomorrow's -3° must not count)", v.Hi, v.Lo)
	}
	if v.Wind != 2 {
		t.Errorf("wind level %d, want 2 (max 9 m/s today)", v.Wind)
	}
	if v.Sunrise != "7:54" || v.Sunset != "18:48" {
		t.Errorf("sun %s %s", v.Sunrise, v.Sunset)
	}
	if len(v.Slots) != 6 || v.Slots[0].Label != "jetzt" || v.Slots[0].Temp != "8°" || v.Slots[1].Label != "8" || v.Slots[5].Label != "20" {
		t.Errorf("slots = %+v", v.Slots)
	}
	if v.Icon != "03d" || !v.Date.Equal(now) {
		t.Errorf("icon %s date %v", v.Icon, v.Date)
	}
}
