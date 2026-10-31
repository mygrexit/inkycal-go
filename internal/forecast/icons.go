package forecast

import "time"

// Glyphs of the Weather Icons font (render.FontWeather).
const (
	GlyphNA      = ""
	GlyphWind    = ""
	GlyphSunrise = ""
	GlyphSunset  = ""
)

// weatherIcons maps OpenWeatherMap icon codes to Weather Icons glyphs, checked against a
// rendered sheet of the font (the table inherited from Python showed "broken clouds"
// as a windy cloud and "shower rain" as sleet).
var weatherIcons = map[string]string{
	"01d": "\uf00d", "02d": "\uf002", "03d": "\uf041", "04d": "\uf013", "09d": "\uf019",
	"10d": "\uf008", "11d": "\uf01e", "13d": "\uf01b", "50d": "\uf014",
	"01n": "\uf02e", "02n": "\uf086", "03n": "\uf041", "04n": "\uf013", "09n": "\uf019",
	"10n": "\uf028", "11n": "\uf01e", "13n": "\uf01b", "50n": "\uf014",
}

// Glyph returns the glyph for an OWM icon code; unknown codes get "n/a"
// instead of failing the whole module like Python's KeyError.
func Glyph(code string) string {
	if g, ok := weatherIcons[code]; ok {
		return g
	}
	return GlyphNA
}

// Fahrenheit converts °C to °F.
func Fahrenheit(c float64) float64 { return c*9.0/5.0 + 32.0 }

// Beaufort converts m/s to the Beaufort scale (thresholds from the Python wrapper).
func Beaufort(mps float64) int {
	thresholds := []float64{0.3, 1.6, 3.4, 5.5, 8.0, 10.8, 13.9, 17.2, 20.8, 24.5, 28.5, 32.7}
	for i, t := range thresholds {
		if mps < t {
			return i
		}
	}
	return 12
}

// WindLevel buckets a mean wind speed: Beaufort ≤3 calm (1), 4–5 breezy (2), ≥6 strong (3).
func WindLevel(mps float64) int {
	switch b := Beaufort(mps); {
	case b >= 6:
		return 3
	case b >= 4:
		return 2
	default:
		return 1
	}
}

// severity ranks OWM conditions by how much they change the day: storm, snow,
// shower rain (09), rain (10), fog, then clouds and sun.
var severity = map[string]int{"11": 9, "13": 8, "09": 7, "10": 6, "50": 5, "04": 4, "03": 3, "02": 2, "01": 1}

func isPrecip(code string) bool { return severity[code] >= 6 }

// DayIcon sums up the daytime (06–21 h) forecasts of now's day that are not yet past:
// any precipitation wins (the most severe one), otherwise the most frequent sky.
// Without daytime forecasts left it falls back to fallback's sky. The result is a
// day-variant OWM icon code.
func DayIcon(points []Point, fallback string, now time.Time) string {
	count := map[string]int{}
	worstPrecip := ""
	for _, p := range points {
		past := p.Time.Before(now.Add(-90 * time.Minute)) // the slot covering "now" still counts
		if past || !SameDay(p.Time, now) || p.Time.Hour() < 6 || p.Time.Hour() > 21 || len(p.Icon) < 2 {
			continue
		}
		code := p.Icon[:2]
		count[code]++
		if isPrecip(code) && severity[code] > severity[worstPrecip] {
			worstPrecip = code
		}
	}
	if worstPrecip != "" {
		return worstPrecip + "d"
	}
	best := ""
	for code, n := range count {
		if n > count[best] || (n == count[best] && severity[code] > severity[best]) {
			best = code
		}
	}
	if best == "" && len(fallback) >= 2 {
		best = fallback[:2]
	}
	return best + "d"
}

// SameDay reports whether b falls on a's calendar day in a's location.
func SameDay(a, b time.Time) bool {
	ay, am, ad := a.Date()
	by, bm, bd := b.In(a.Location()).Date()
	return ay == by && am == bm && ad == bd
}
