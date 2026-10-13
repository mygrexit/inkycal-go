package fullweather

import (
	"math"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"inkycal/internal/modules/weather"
	"inkycal/internal/timefmt"
)

// windUnit is one of upstream's wind_unit options.
type windUnit int

const (
	windMPS   windUnit = iota // meters_sec (default)
	windKMH                   // km_hour
	windMPH                   // miles_hour
	windKnots                 // knots
	windBft                   // beaufort
)

func parseWindUnit(s string) windUnit {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "km_hour":
		return windKMH
	case "miles_hour":
		return windMPH
	case "knots":
		return windKnots
	case "beaufort":
		return windBft
	}
	return windMPS // upstream falls back to m/s for anything else
}

func (u windUnit) convert(mps float64) float64 {
	switch u {
	case windKMH:
		return mps * 3.6
	case windMPH:
		return mps * 2.23694
	case windKnots:
		return mps * 1.94384
	case windBft:
		return float64(weather.Beaufort(mps))
	}
	return mps
}

func (u windUnit) label() string {
	return [...]string{"m/s", "km/h", "mph", "kn", "Bft"}[u]
}

// settings are the display options parsed from settings.json.
type settings struct {
	Fahrenheit bool
	Wind       windUnit
	Gusts      bool // show the gust next to the mean wind
	MinMax     bool // annotate the lowest and highest temperature in the chart
	Hour12     bool
	Lang       string
}

func (s settings) temp(celsius float64) float64 {
	if s.Fahrenheit {
		return weather.ToFahrenheit(celsius)
	}
	return celsius
}

func (s settings) deg(celsius float64) string {
	return strconv.Itoa(int(math.Round(s.temp(celsius)))) + "°"
}

func (s settings) german() bool { return strings.HasPrefix(s.Lang, "de") }

// chartHours is how far the chart looks ahead (upstream: 22 three-hour slots).
const chartHours = 66

// maxDays is the number of day columns, today included (as upstream).
const maxDays = 5

// sample is one point of the chart; Precip covers the three hours before Time.
type sample struct {
	Time   time.Time
	Temp   float64 // display unit
	Precip float64 // mm
}

// dayCol is one column of the multi-day forecast.
type dayCol struct {
	Label  string // "Heute" or the short weekday
	Date   string
	Today  bool
	Icon   string // OWM icon code
	Hi, Lo string
	Precip string // "" when dry
}

// view is everything the module shows, already formatted.
type view struct {
	Date        time.Time
	Lang        string
	Icon        string
	Temp        string
	FeelsLike   string // "gefühlt 6°"
	Description string
	Wind        string
	WindStrong  bool
	Precip      string // the coming three hours
	Humidity    string
	UV          string // "" without api_version 3.0
	UVHigh      bool
	Sunrise     string
	Sunset      string

	Now        sample // the observation
	Samples    []sample
	Fahrenheit bool
	MinMax     bool
	Hour12     bool

	Days []dayCol
}

// decimal formats v with one decimal and the language's separator.
func decimal(v float64, lang string) string {
	s := strconv.FormatFloat(v, 'f', 1, 64)
	if strings.HasPrefix(lang, "de") {
		s = strings.Replace(s, ".", ",", 1)
	}
	return s
}

// mm formats a precipitation amount: one decimal below 10 mm.
func mm(v float64, lang string) string {
	switch {
	case v <= 0:
		return "0 mm"
	case v < 9.95:
		return decimal(v, lang) + " mm"
	}
	return strconv.Itoa(int(math.Round(v))) + " mm"
}

func capitalise(s string) string {
	r, n := utf8.DecodeRuneInString(s)
	if r == utf8.RuneError {
		return s
	}
	return string(unicode.ToUpper(r)) + s[n:]
}

// windText renders mean wind and, if wanted and different, the gust: "3–7 m/s".
func (s settings) windText(mean, gust float64) string {
	m := int(math.Round(s.Wind.convert(mean)))
	g := int(math.Round(s.Wind.convert(gust)))
	if s.Gusts && g > m {
		return strconv.Itoa(m) + "–" + strconv.Itoa(g) + " " + s.Wind.label()
	}
	return strconv.Itoa(m) + " " + s.Wind.label()
}

// buildView condenses the observation and the 3-hour forecasts into the view.
func buildView(s settings, cur weather.Current, points []weather.ForecastPoint, uvi *float64, now time.Time) view {
	de := s.german()
	v := view{
		Date:        now,
		Lang:        s.Lang,
		Icon:        cur.Icon,
		Temp:        s.deg(cur.Temp),
		Description: capitalise(strings.TrimSpace(cur.Description)),
		Humidity:    strconv.Itoa(cur.Humidity) + " %",
		Now:         sample{Time: now, Temp: s.temp(cur.Temp)},
		Fahrenheit:  s.Fahrenheit,
		MinMax:      s.MinMax,
		Hour12:      s.Hour12,
	}
	if de {
		v.FeelsLike = "gefühlt " + s.deg(cur.FeelsLike)
	} else {
		v.FeelsLike = "feels like " + s.deg(cur.FeelsLike)
	}
	clock := "H:mm"
	if s.Hour12 {
		clock = "h:mm a"
	}
	if cur.Sunrise.Unix() != 0 { // OWM sends 0 in polar day/night
		v.Sunrise = timefmt.Format(cur.Sunrise.In(now.Location()), clock, "en")
		v.Sunset = timefmt.Format(cur.Sunset.In(now.Location()), clock, "en")
	}
	if uvi != nil {
		v.UV = "UV " + decimal(*uvi, s.Lang)
		v.UVHigh = *uvi >= 6
	}

	// Wind and precipitation of the coming three hours, as upstream: the first
	// forecast slot that has not passed yet, else the observation.
	mean, gust := cur.WindMean, cur.WindMPS
	v.Precip = mm(0, s.Lang)
	for _, p := range points {
		if p.Time.After(now) {
			mean, gust = p.Wind, p.Gust
			v.Precip = mm(p.Precip, s.Lang)
			break
		}
	}
	v.Wind = s.windText(mean, gust)
	v.WindStrong = weather.WindLevel(mean) >= 3

	end := now.Add(chartHours * time.Hour)
	for _, p := range points {
		if p.Time.After(now) && !p.Time.After(end) {
			v.Samples = append(v.Samples, sample{Time: p.Time, Temp: s.temp(p.Temp), Precip: p.Precip})
		}
	}

	y, mo, d := now.Date()
	for i := 0; i < maxDays; i++ {
		start := time.Date(y, mo, d+i, 0, 0, 0, 0, now.Location())
		var day []weather.ForecastPoint
		for _, p := range points {
			if weather.SameDay(p.Time, start) && (i > 0 || p.Time.After(now.Add(-90*time.Minute))) {
				day = append(day, p)
			}
		}
		if len(day) == 0 && i > 0 {
			break
		}
		col := dayCol{Today: i == 0}
		hi, lo, rain := math.Inf(-1), math.Inf(1), 0.0
		if i == 0 {
			hi, lo = cur.Temp, cur.Temp
			col.Icon = weather.DayIcon(points, cur.Icon, now)
			col.Label = map[bool]string{true: "Heute", false: "Today"}[de]
		} else {
			col.Icon = weather.DayIcon(points, day[0].Icon, start)
			col.Label = timefmt.Format(start, "ddd", s.Lang)
		}
		for _, p := range day {
			hi, lo, rain = math.Max(hi, p.Temp), math.Min(lo, p.Temp), rain+p.Precip
		}
		col.Hi, col.Lo = s.deg(hi), s.deg(lo)
		if de {
			col.Date = timefmt.Format(start, "D. MMM", s.Lang)
		} else {
			col.Date = timefmt.Format(start, "MMM D", s.Lang)
		}
		if rain >= 0.05 {
			col.Precip = mm(rain, s.Lang)
		}
		v.Days = append(v.Days, col)
	}
	return v
}
