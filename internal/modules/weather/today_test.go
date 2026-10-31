package weather

import (
	"inkycal/internal/forecast"
	"inkycal/internal/forecast/forecasttest"
	"strings"
	"testing"
	"time"
)

func TestBuildDay(t *testing.T) {
	loc := forecasttest.Berlin(t)
	now := time.Date(2026, 10, 13, 6, 5, 0, 0, loc)
	cur := forecast.Current{Icon: "04n", Temp: 7.6, WindMean: 3, Sunrise: time.Date(2026, 10, 13, 7, 54, 0, 0, loc), Sunset: time.Date(2026, 10, 13, 18, 48, 0, 0, loc)}
	var pts []forecast.Point
	temps := []float64{8, 11, 14.4, 13, 10, 9, 6}
	for i, h := range []int{8, 11, 14, 17, 20, 23} {
		pts = append(pts, forecast.Point{Time: time.Date(2026, 10, 13, h, 0, 0, 0, loc), Temp: temps[i], Icon: "03d", Wind: 4 + float64(i)})
	}
	pts = append(pts, forecast.Point{Time: time.Date(2026, 10, 14, 2, 0, 0, 0, loc), Temp: -3, Icon: "13n", Wind: 30})

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
	if len(v.Slots) != 6 || v.Slots[0].Label != "6" || v.Slots[0].Temp != "8°" || v.Slots[1].Label != "8" || v.Slots[5].Label != "20" {
		t.Errorf("slots = %+v", v.Slots)
	}
	if v.Icon != "03d" || !v.Date.Equal(now) {
		t.Errorf("icon %s date %v", v.Icon, v.Date)
	}

	// A wake shortly before the hour: the observation counts as that hour, and the
	// forecast for the same hour would only repeat it.
	v = buildDay(settings{Lang: "de"}, cur, pts, time.Date(2026, 10, 13, 10, 40, 0, 0, loc))
	if v.Slots[0].Label != "11" || v.Slots[1].Label != "14" {
		t.Errorf("10:40 slots = %+v", v.Slots)
	}

	// Winter time: the 3-hour forecasts fall on 7, 10, 13 … A 06:00 update skips
	// 7 so the timeline keeps an even rhythm across the day.
	var winter []forecast.Point
	for _, h := range []int{7, 10, 13, 16, 19, 22} {
		winter = append(winter, forecast.Point{Time: time.Date(2026, 12, 1, h, 0, 0, 0, loc), Temp: 2, Icon: "04d"})
	}
	v = buildDay(settings{Lang: "de"}, cur, winter, time.Date(2026, 12, 1, 6, 0, 10, 0, loc))
	var labels []string
	for _, s := range v.Slots {
		labels = append(labels, s.Label)
	}
	if got := strings.Join(labels, " "); got != "6 10 13 16 19 22" {
		t.Errorf("winter 06:00 slots = %s", got)
	}
}

func TestHourLabel12h(t *testing.T) {
	loc := forecasttest.Berlin(t)
	for h, want := range map[int]string{0: "12am", 6: "6am", 12: "12pm", 17: "5pm"} {
		if got := (settings{Hour12: true}).hourLabel(time.Date(2026, 10, 13, h, 0, 0, 0, loc)); got != want {
			t.Errorf("%d → %s, want %s", h, got, want)
		}
	}
	if got := (settings{}).hourLabel(time.Date(2026, 10, 13, 17, 0, 0, 0, loc)); got != "17" {
		t.Errorf("24h: %s", got)
	}
}
