package weather

import (
	"math"
	"time"
)

// Glyphs of the Weather Icons font (render.FontWeather).
const (
	glyphNA          = ""
	glyphThermometer = ""
	glyphHumidity    = ""
	glyphWind        = ""
	glyphSunrise     = ""
	glyphSunset      = ""
)

// Wind units, named like the Python OpenWeatherMap wrapper.
const (
	windMetersSec = "meters_sec"
	windMilesHour = "miles_hour"
	windBeaufort  = "beaufort"
)

// weatherIcons maps OpenWeatherMap icon codes to Weather Icons glyphs (table from inkycal_weather.py).
var weatherIcons = map[string]string{
	"01d": "", "02d": "", "03d": "", "04d": "", "09d": "",
	"10d": "", "11d": "", "13d": "", "50d": "",
	"01n": "", "02n": "", "03n": "", "04n": "", "09n": "",
	"10n": "", "11n": "", "13n": "", "50n": "",
}

var moonGlyphs = [8]string{"", "", "", "", "", "", "", ""}

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

func convertWind(mps float64, unit string) float64 {
	switch unit {
	case windBeaufort:
		return float64(beaufort(mps))
	case windMilesHour:
		return mps * 2.23694
	default:
		return mps
	}
}

// moonPhaseGlyph approximates the moon phase exactly like Python's get_moon_phase.
func moonPhaseGlyph(t time.Time) string {
	epoch := time.Date(2001, 1, 1, 0, 0, 0, 0, time.UTC)
	days := t.Sub(epoch).Hours() / 24
	lunations := 0.20439731 + days*0.03386319269
	position := lunations - math.Floor(lunations)
	return moonGlyphs[int(math.Floor(position*8+0.5))&7]
}
