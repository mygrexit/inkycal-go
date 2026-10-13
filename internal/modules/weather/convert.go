package weather

// Glyphs of the Weather Icons font (render.FontWeather).
const (
	glyphNA      = ""
	glyphWind    = ""
	glyphSunrise = ""
	glyphSunset  = ""
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

// iconGlyph returns the glyph for an OWM icon code; unknown codes get "n/a"
// instead of failing the whole module like Python's KeyError.
func iconGlyph(code string) string {
	if g, ok := weatherIcons[code]; ok {
		return g
	}
	return glyphNA
}

func toFahrenheit(c float64) float64 { return c*9.0/5.0 + 32.0 }

// beaufort converts m/s to the Beaufort scale (thresholds from the Python wrapper).
func beaufort(mps float64) int {
	thresholds := []float64{0.3, 1.6, 3.4, 5.5, 8.0, 10.8, 13.9, 17.2, 20.8, 24.5, 28.5, 32.7}
	for i, t := range thresholds {
		if mps < t {
			return i
		}
	}
	return 12
}
