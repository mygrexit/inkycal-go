package weather

import (
	"errors"
	"fmt"
	"math"
	"strconv"
	"time"

	"inkycal/internal/timefmt"
)

// settings are the display options parsed from settings.json.
type settings struct {
	Fahrenheit bool
	WindUnit   string // windMetersSec, windMilesHour or windBeaufort
	RoundTemp  bool
	RoundWind  bool
	Daily      bool
	Hour12     bool
	Lang       string
}

type fcCell struct {
	Stamp string
	Icon  string // glyph
	Temp  string
}

// view holds every string the draw step writes, already formatted.
type view struct {
	CurrentIcon  string
	Temperature  string
	TempNegative bool
	Humidity     string
	Wind         string
	Moon         string
	Sunrise      string
	Sunset       string
	Forecasts    []fcCell
}

type dayForecast struct {
	Date     time.Time
	Icon     string
	Min, Max float64
}

func (s settings) temp(celsius float64) float64 {
	if s.Fahrenheit {
		return toFahrenheit(celsius)
	}
	return celsius
}

func (s settings) formatTemp(v float64) string {
	dec := 1
	if s.RoundTemp {
		dec = 0
	}
	unit := "°"
	if s.Fahrenheit {
		unit = "F"
	}
	return strconv.FormatFloat(v, 'f', dec, 64) + unit
}

func (s settings) windLabel() string {
	switch s.WindUnit {
	case windBeaufort:
		return "bft"
	case windMilesHour:
		return "mph"
	default:
		return "m/s"
	}
}

// dailyForecast aggregates the 3-hour slots of the day `offset` days after now.
// Without slots for that day it falls back to the first slot, like Python.
// points must not be empty. Ties for the icon go to the first-seen code.
func dailyForecast(points []forecastPoint, now time.Time, offset int) dayForecast {
	y, m, d := now.Date()
	start := time.Date(y, m, d+offset, 0, 0, 0, 0, now.Location())
	end := start.AddDate(0, 0, 1)
	var day []forecastPoint
	for _, p := range points {
		if !p.Time.Before(start) && p.Time.Before(end) {
			day = append(day, p)
		}
	}
	if len(day) == 0 {
		day = points[:1]
	}
	df := dayForecast{Date: start, Min: day[0].Temp, Max: day[0].Temp}
	counts := map[string]int{}
	best := 0
	for _, p := range day {
		df.Min = math.Min(df.Min, p.Temp)
		df.Max = math.Max(df.Max, p.Temp)
		counts[p.Icon]++
		if counts[p.Icon] > best {
			best = counts[p.Icon]
			df.Icon = p.Icon
		}
	}
	return df
}

// buildView formats current weather and four forecast cells.
func buildView(s settings, cur current, points []forecastPoint, now time.Time) (view, error) {
	if len(points) == 0 {
		return view{}, errors.New("weather: forecast contains no entries")
	}
	loc := now.Location()
	temp := s.temp(cur.Temp)
	freezing := 0.0
	if s.Fahrenheit {
		freezing = 32
	}
	windDec := 1
	if s.RoundWind || s.WindUnit == windBeaufort {
		windDec = 0
	}
	clock := "H:mm"
	if s.Hour12 {
		clock = "h:mm a"
	}
	v := view{
		CurrentIcon:  iconGlyph(cur.Icon),
		Temperature:  s.formatTemp(temp) + "/" + s.formatTemp(s.temp(cur.MaxTemp)),
		TempNegative: math.Round(temp) <= freezing,
		Humidity:     fmt.Sprintf("%d %%", cur.Humidity),
		Wind:         strconv.FormatFloat(convertWind(cur.WindMPS, s.WindUnit), 'f', windDec, 64) + " " + s.windLabel(),
		Moon:         moonPhaseGlyph(now),
		Sunrise:      timefmt.Format(cur.Sunrise.In(loc), clock, "en"),
		Sunset:       timefmt.Format(cur.Sunset.In(loc), clock, "en"),
	}
	if s.Daily {
		for d := 1; d <= 4; d++ {
			df := dailyForecast(points, now, d)
			v.Forecasts = append(v.Forecasts, fcCell{
				Stamp: timefmt.Format(df.Date, "ddd", s.Lang),
				Icon:  iconGlyph(df.Icon),
				Temp:  s.formatTemp(s.temp(df.Min)) + "/" + s.formatTemp(s.temp(df.Max)),
			})
		}
		return v, nil
	}
	stamp := "H:mm"
	if s.Hour12 {
		stamp = "h a"
	}
	for i, p := range points {
		if i == 4 {
			break
		}
		v.Forecasts = append(v.Forecasts, fcCell{
			Stamp: timefmt.Format(p.Time.In(loc), stamp, "en"),
			Icon:  iconGlyph(p.Icon),
			Temp:  s.formatTemp(s.temp(p.Temp)),
		})
	}
	return v, nil
}
