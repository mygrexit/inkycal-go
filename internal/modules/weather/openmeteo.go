package weather

import (
	"context"
	"errors"
	"net/url"
	"strconv"
	"time"

	"inkycal/internal/fetch"
	"inkycal/internal/i18n"
)

// openMeteoURL is a variable so tests can point it at an httptest server.
var openMeteoURL = "https://api.open-meteo.com/v1/forecast"

type omResponse struct {
	Current struct {
		Temp      float64 `json:"temperature_2m"`
		Humidity  int     `json:"relative_humidity_2m"`
		FeelsLike float64 `json:"apparent_temperature"`
		IsDay     int     `json:"is_day"`
		Code      int     `json:"weather_code"`
		Wind      float64 `json:"wind_speed_10m"`
		Gust      float64 `json:"wind_gusts_10m"`
	} `json:"current"`
	Hourly struct {
		Time   []int64   `json:"time"`
		Temp   []float64 `json:"temperature_2m"`
		Code   []int     `json:"weather_code"`
		Precip []float64 `json:"precipitation"`
		Wind   []float64 `json:"wind_speed_10m"`
		Gust   []float64 `json:"wind_gusts_10m"`
		IsDay  []int     `json:"is_day"`
	} `json:"hourly"`
	Daily struct {
		Max     []float64 `json:"temperature_2m_max"`
		Sunrise []int64   `json:"sunrise"`
		Sunset  []int64   `json:"sunset"`
	} `json:"daily"`
}

// fetchOpenMeteo reads current conditions and the hourly forecast from Open-Meteo (no
// account needed) into the same model as OpenWeatherMap: metric units, OWM-style icon
// codes and 3-hour slots whose precipitation covers the three hours up to the slot.
func fetchOpenMeteo(ctx context.Context, fc *fetch.Client, lat, lon float64, lang string, loc *time.Location) (current, []forecastPoint, bool, error) {
	q := url.Values{}
	q.Set("latitude", strconv.FormatFloat(lat, 'f', -1, 64))
	q.Set("longitude", strconv.FormatFloat(lon, 'f', -1, 64))
	q.Set("current", "temperature_2m,relative_humidity_2m,apparent_temperature,is_day,weather_code,wind_speed_10m,wind_gusts_10m")
	q.Set("hourly", "temperature_2m,weather_code,precipitation,wind_speed_10m,wind_gusts_10m,is_day")
	q.Set("daily", "temperature_2m_max,sunrise,sunset")
	q.Set("wind_speed_unit", "ms")
	q.Set("timezone", "auto")
	q.Set("timeformat", "unixtime")
	q.Set("forecast_days", "3")
	var r omResponse
	stale, err := fc.GetJSON(ctx, openMeteoURL+"?"+q.Encode(), nil, &r)
	if err != nil {
		return current{}, nil, false, err
	}
	h := r.Hourly
	n := len(h.Time)
	if n == 0 || len(h.Temp) < n || len(h.Code) < n || len(h.Precip) < n || len(h.Wind) < n || len(h.Gust) < n || len(h.IsDay) < n {
		return current{}, nil, false, errors.New("Open-Meteo answer without hourly data")
	}
	cur := current{
		Icon:        wmoIcon(r.Current.Code, r.Current.IsDay == 1),
		Description: wmoText(r.Current.Code, lang),
		Temp:        r.Current.Temp,
		FeelsLike:   r.Current.FeelsLike,
		Humidity:    r.Current.Humidity,
		WindMPS:     max(r.Current.Gust, r.Current.Wind),
		WindMean:    r.Current.Wind,
		MaxTemp:     r.Current.Temp,
	}
	if len(r.Daily.Max) > 0 {
		cur.MaxTemp = r.Daily.Max[0]
	}
	if len(r.Daily.Sunrise) > 0 && len(r.Daily.Sunset) > 0 {
		cur.Sunrise = time.Unix(r.Daily.Sunrise[0], 0).In(loc)
		cur.Sunset = time.Unix(r.Daily.Sunset[0], 0).In(loc)
	}
	var points []forecastPoint
	for i := range n {
		at := time.Unix(h.Time[i], 0).In(loc)
		if at.Hour()%3 != 0 {
			continue
		}
		p := forecastPoint{Time: at, Temp: h.Temp[i], Icon: wmoIcon(h.Code[i], h.IsDay[i] == 1),
			Wind: h.Wind[i], Gust: max(h.Gust[i], h.Wind[i])}
		for j := max(0, i-2); j <= i; j++ { // each value is the preceding hour's sum
			p.Precip += h.Precip[j]
		}
		points = append(points, p)
	}
	return cur, points, stale, nil
}

// wmoIcon maps a WMO weather code to the OpenWeatherMap icon code the views draw.
func wmoIcon(code int, day bool) string {
	var c string
	switch {
	case code == 0:
		c = "01"
	case code == 1:
		c = "02"
	case code == 2:
		c = "03"
	case code == 45 || code == 48:
		c = "50"
	case code == 56 || code == 57 || code == 66 || code == 67 || (code >= 71 && code <= 77) || code == 85 || code == 86:
		c = "13"
	case code >= 51 && code <= 55, code >= 80 && code <= 82:
		c = "09"
	case code >= 61 && code <= 65:
		c = "10"
	case code >= 95:
		c = "11"
	default:
		c = "04"
	}
	if day {
		return c + "d"
	}
	return c + "n"
}

var wmoTexts = map[int][2]string{ // en, de
	0: {"Clear sky", "Klarer Himmel"}, 1: {"Mainly clear", "Überwiegend klar"}, 2: {"Partly cloudy", "Teilweise bewölkt"},
	3: {"Overcast", "Bedeckt"}, 45: {"Fog", "Nebel"}, 48: {"Rime fog", "Reifnebel"},
	51: {"Light drizzle", "Leichter Nieselregen"}, 53: {"Drizzle", "Nieselregen"}, 55: {"Dense drizzle", "Starker Nieselregen"},
	56: {"Light freezing drizzle", "Leichter gefrierender Nieselregen"}, 57: {"Freezing drizzle", "Gefrierender Nieselregen"},
	61: {"Light rain", "Leichter Regen"}, 63: {"Rain", "Regen"}, 65: {"Heavy rain", "Starker Regen"},
	66: {"Light freezing rain", "Leichter gefrierender Regen"}, 67: {"Freezing rain", "Gefrierender Regen"},
	71: {"Light snow", "Leichter Schneefall"}, 73: {"Snow", "Schneefall"}, 75: {"Heavy snow", "Starker Schneefall"},
	77: {"Snow grains", "Schneegriesel"}, 80: {"Light showers", "Leichte Regenschauer"}, 81: {"Showers", "Regenschauer"},
	82: {"Heavy showers", "Heftige Regenschauer"}, 85: {"Snow showers", "Schneeschauer"}, 86: {"Heavy snow showers", "Starke Schneeschauer"},
	95: {"Thunderstorm", "Gewitter"}, 96: {"Thunderstorm with hail", "Gewitter mit Hagel"}, 99: {"Heavy thunderstorm with hail", "Schweres Gewitter mit Hagel"},
}

// wmoText describes a WMO code in English or German.
func wmoText(code int, lang string) string {
	t, ok := wmoTexts[code]
	if !ok {
		return ""
	}
	if i18n.German(lang) {
		return t[1]
	}
	return t[0]
}
