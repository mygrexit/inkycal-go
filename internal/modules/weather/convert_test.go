package weather

import "testing"

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

func TestToFahrenheit(t *testing.T) {
	if toFahrenheit(0) != 32 || toFahrenheit(100) != 212 {
		t.Errorf("toFahrenheit wrong: %v %v", toFahrenheit(0), toFahrenheit(100))
	}
}

func TestIconGlyph(t *testing.T) {
	for code, want := range map[string]string{"01d": "\uf00d", "04d": "\uf013", "09n": "\uf019", "10d": "\uf008", "13n": "\uf01b"} {
		if got := iconGlyph(code); got != want {
			t.Errorf("%s = %U, want %U", code, []rune(got)[0], []rune(want)[0])
		}
	}
	if got := iconGlyph("77z"); got != glyphNA {
		t.Errorf("unknown code = %q, want glyphNA", got)
	}
}
