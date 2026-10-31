package weather

import (
	"math"
	"strconv"
	"time"

	"inkycal/internal/forecast"
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
		return forecast.Fahrenheit(celsius)
	}
	return celsius
}

const maxSlots = 6 // the current observation plus the next 3-hour forecasts of today

// slot is one column of the day's timeline.
type slot struct {
	Label string // the hour
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

// hourLabel names a timeline column: "14" or, with hour_format 12, "2pm".
func (s settings) hourLabel(t time.Time) string {
	if !s.Hour12 {
		return strconv.Itoa(t.Hour())
	}
	h := t.Hour() % 12
	if h == 0 {
		h = 12
	}
	if t.Hour() < 12 {
		return strconv.Itoa(h) + "am"
	}
	return strconv.Itoa(h) + "pm"
}

func (s settings) roundTemp(celsius float64) string {
	v := s.temp(celsius)
	return strconv.Itoa(int(math.Round(v))) + "°"
}

// buildDay condenses current weather and today's forecasts into the band's content.
func buildDay(s settings, cur forecast.Current, points []forecast.Point, now time.Time) dayView {
	hi, lo, wind := cur.Temp, cur.Temp, cur.WindMean
	// The first slot is labelled with the hour of the observation, not "now": the
	// panel is read for hours after the update. Forecasts follow at least 90 minutes
	// later, so the same hour never appears twice and the columns keep an even rhythm.
	slots := []slot{{Label: s.hourLabel(now.Add(30 * time.Minute)), Icon: cur.Icon, Temp: s.roundTemp(cur.Temp)}}
	for _, p := range points {
		if !forecast.SameDay(p.Time, now) || !p.Time.After(now) {
			continue
		}
		hi, lo, wind = math.Max(hi, p.Temp), math.Min(lo, p.Temp), math.Max(wind, p.Wind)
		if len(slots) < maxSlots && p.Time.After(now.Add(90*time.Minute)) {
			slots = append(slots, slot{Label: s.hourLabel(p.Time), Icon: p.Icon, Temp: s.roundTemp(p.Temp)})
		}
	}
	clock := "H:mm"
	if s.Hour12 {
		clock = "h:mm a"
	}
	v := dayView{
		Date:  now,
		Icon:  forecast.DayIcon(points, cur.Icon, now),
		Hi:    s.roundTemp(hi),
		Lo:    s.roundTemp(lo),
		Wind:  forecast.WindLevel(wind),
		Slots: slots,
	}
	if !cur.Sunrise.IsZero() { // none in polar day or night
		v.Sunrise = timefmt.Format(cur.Sunrise.In(now.Location()), clock, "en")
		v.Sunset = timefmt.Format(cur.Sunset.In(now.Location()), clock, "en")
	}
	return v
}
