package weather

import (
	"math"
	"strconv"
	"time"

	"inkycal/internal/timefmt"
)

// settings are the display options parsed from settings.json.
type settings struct {
	Fahrenheit bool
	Hour12     bool
	Lang       string
}

func (s settings) temp(celsius float64) float64 {
	if s.Fahrenheit {
		return toFahrenheit(celsius)
	}
	return celsius
}

const maxSlots = 6 // "now" plus the next 3-hour forecasts of today

// slot is one column of the day's timeline.
type slot struct {
	Label string // "jetzt" or the hour
	Icon  string // OWM icon code
	Temp  string
}

// dayView is everything the today band shows, already formatted.
type dayView struct {
	Date            time.Time
	Icon            string // OWM icon code (day variant) summing up the day
	Hi, Lo          string
	Wind            int // 1 calm, 2 breezy, 3 strong
	Sunrise, Sunset string
	Slots           []slot
}

// windLevel buckets the mean wind speed: Beaufort ≤3 calm, 4–5 breezy, ≥6 strong.
func windLevel(mps float64) int {
	switch b := beaufort(mps); {
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

// dayIcon sums up today's daytime (06–21 h) forecasts: any precipitation wins
// (the most severe one), otherwise the most frequent sky. Without daytime
// forecasts left it falls back to the current conditions.
func dayIcon(points []forecastPoint, currentIcon string, now time.Time) string {
	count := map[string]int{}
	worstPrecip := ""
	for _, p := range points {
		past := p.Time.Before(now.Add(-90 * time.Minute)) // the slot covering "now" still counts
		if past || !sameDay(p.Time, now) || p.Time.Hour() < 6 || p.Time.Hour() > 21 || len(p.Icon) < 2 {
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
	if best == "" && len(currentIcon) >= 2 {
		best = currentIcon[:2]
	}
	return best + "d"
}

func nowLabel(lang string) string {
	if lang == "de" {
		return "jetzt"
	}
	return "now"
}

func sameDay(a, b time.Time) bool {
	ay, am, ad := a.Date()
	by, bm, bd := b.In(a.Location()).Date()
	return ay == by && am == bm && ad == bd
}

func (s settings) roundTemp(celsius float64) string {
	v := s.temp(celsius)
	return strconv.Itoa(int(math.Round(v))) + "°"
}

// buildDay condenses current weather and today's forecasts into the band's content.
func buildDay(s settings, cur current, points []forecastPoint, now time.Time) dayView {
	hi, lo, wind := cur.Temp, cur.Temp, cur.WindMean
	slots := []slot{{Label: nowLabel(s.Lang), Icon: cur.Icon, Temp: s.roundTemp(cur.Temp)}}
	for _, p := range points {
		if !sameDay(p.Time, now) || !p.Time.After(now) {
			continue
		}
		hi, lo, wind = math.Max(hi, p.Temp), math.Min(lo, p.Temp), math.Max(wind, p.Wind)
		if len(slots) < maxSlots {
			slots = append(slots, slot{Label: strconv.Itoa(p.Time.Hour()), Icon: p.Icon, Temp: s.roundTemp(p.Temp)})
		}
	}
	clock := "H:mm"
	if s.Hour12 {
		clock = "h:mm a"
	}
	return dayView{
		Date:    now,
		Icon:    dayIcon(points, cur.Icon, now),
		Hi:      s.roundTemp(hi),
		Lo:      s.roundTemp(lo),
		Wind:    windLevel(wind),
		Sunrise: timefmt.Format(cur.Sunrise.In(now.Location()), clock, "en"),
		Sunset:  timefmt.Format(cur.Sunset.In(now.Location()), clock, "en"),
		Slots:   slots,
	}
}
