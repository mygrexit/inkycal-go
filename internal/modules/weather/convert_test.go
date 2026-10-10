package weather

import (
	"math"
	"testing"
	"time"
)

func TestBeaufort(t *testing.T) {
	cases := []struct {
		mps  float64
		want int
	}{{0, 0}, {0.29, 0}, {0.3, 1}, {1.5, 1}, {5.5, 4}, {7.9, 4}, {32.6, 11}, {40, 12}}
	for _, c := range cases {
		if got := beaufort(c.mps); got != c.want {
			t.Errorf("beaufort(%v) = %d, want %d", c.mps, got, c.want)
		}
	}
}

func TestConvertWind(t *testing.T) {
	if got := convertWind(10, windMetersSec); got != 10 {
		t.Errorf("meters_sec = %v", got)
	}
	if got := convertWind(10, windMilesHour); math.Abs(got-22.3694) > 1e-9 {
		t.Errorf("miles_hour = %v", got)
	}
	if got := convertWind(10, windBeaufort); got != 5 {
		t.Errorf("beaufort = %v", got)
	}
}

func TestToFahrenheit(t *testing.T) {
	if toFahrenheit(0) != 32 || toFahrenheit(100) != 212 {
		t.Errorf("toFahrenheit wrong: %v %v", toFahrenheit(0), toFahrenheit(100))
	}
}

func TestMoonPhaseGlyph(t *testing.T) {
	// 2001-01-01: lunation position 0.204 -> index 2.
	if got := moonPhaseGlyph(time.Date(2001, 1, 1, 0, 0, 0, 0, time.UTC)); got != "" {
		t.Errorf("2001-01-01: got %q", got)
	}
	// 2001-01-11: 0.2044 + 10*0.03386 = 0.543 -> index 4.
	if got := moonPhaseGlyph(time.Date(2001, 1, 11, 0, 0, 0, 0, time.UTC)); got != "" {
		t.Errorf("2001-01-11: got %q", got)
	}
}

func TestIconGlyph(t *testing.T) {
	if got := iconGlyph("01d"); got != "" {
		t.Errorf("01d = %q", got)
	}
	if got := iconGlyph("13n"); got != "" {
		t.Errorf("13n = %q", got)
	}
	if got := iconGlyph("77z"); got != glyphNA {
		t.Errorf("unknown code = %q, want glyphNA", got)
	}
}
