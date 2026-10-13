package weather

// Exported wrappers for modules that build on this one (fullweather). They add no
// behaviour of their own; the today band keeps using the unexported names.

import (
	"context"
	"net/url"
	"strconv"
	"time"

	"inkycal/internal/fetch"
	"inkycal/internal/render"
)

// Current is the parsed /weather response (metric units).
type Current = current

// ForecastPoint is one 3-hour slot of /forecast (metric units).
type ForecastPoint = forecastPoint

// Glyphs of the Weather Icons font shared with other weather views.
const (
	GlyphWind    = glyphWind
	GlyphSunrise = glyphSunrise
	GlyphSunset  = glyphSunset
)

// CoordsURL builds an OWM 2.5 request URL for a latitude/longitude pair.
func CoordsURL(base, endpoint, apiKey string, lat, lon float64, lang string) string {
	q := url.Values{}
	q.Set("lat", strconv.FormatFloat(lat, 'f', -1, 64))
	q.Set("lon", strconv.FormatFloat(lon, 'f', -1, 64))
	return owmURL(base, endpoint, apiKey, q, lang)
}

// FetchCurrent fetches and parses a /weather URL; the bool reports cached data.
func FetchCurrent(ctx context.Context, fc *fetch.Client, u string, loc *time.Location) (Current, bool, error) {
	return getCurrent(ctx, fc, u, loc)
}

// FetchForecast fetches and parses a /forecast URL; the bool reports cached data.
func FetchForecast(ctx context.Context, fc *fetch.Client, u string, loc *time.Location) ([]ForecastPoint, bool, error) {
	return getForecast(ctx, fc, u, loc)
}

// IconGlyph returns the Weather Icons glyph for an OWM icon code ("n/a" if unknown).
func IconGlyph(code string) string { return iconGlyph(code) }

// SizeForCap returns the font size whose capitals are about capPx tall.
func SizeForCap(c *render.Canvas, font render.FontID, capPx int) int {
	return sizeForCap(c, font, capPx)
}

// ToFahrenheit converts °C to °F.
func ToFahrenheit(c float64) float64 { return toFahrenheit(c) }

// Beaufort converts m/s to the Beaufort scale.
func Beaufort(mps float64) int { return beaufort(mps) }

// WindLevel buckets a wind speed: 1 calm, 2 breezy (Bft 4–5), 3 strong (Bft ≥ 6).
func WindLevel(mps float64) int { return windLevel(mps) }

// DayIcon sums up the daytime forecasts of the day of now that are not yet past:
// the most severe precipitation, else the most frequent sky, else fallback's sky.
// The result is a day-variant OWM icon code.
func DayIcon(points []ForecastPoint, fallback string, now time.Time) string {
	return dayIcon(points, fallback, now)
}

// SameDay reports whether b falls on a's calendar day in a's location.
func SameDay(a, b time.Time) bool { return sameDay(a, b) }
