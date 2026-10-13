package weather

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"

	"inkycal/internal/fetch"
)

// OWMBaseURL is the OpenWeatherMap 2.5 API root.
const OWMBaseURL = "https://api.openweathermap.org/data/2.5"

// owmBaseURL is a variable so tests can point it at an httptest server.
var owmBaseURL = OWMBaseURL

// current holds the parts of /weather the module displays (metric units).
type current struct {
	Icon        string
	Description string // localised condition text, e.g. "leichter Regen"
	Temp        float64
	FeelsLike   float64
	MaxTemp     float64
	Humidity    int
	WindMPS     float64 // gust if reported, else mean speed (as in Python)
	WindMean    float64 // mean speed, m/s
	Sunrise     time.Time
	Sunset      time.Time
}

// forecastPoint is one 3-hour slot of /forecast (metric units).
type forecastPoint struct {
	Time   time.Time
	Temp   float64
	Icon   string
	Wind   float64 // mean speed, m/s
	Gust   float64 // gust if reported, else mean speed, m/s
	Precip float64 // rain + snow of the 3-hour slot, mm
}

type owmWeather struct {
	Icon        string `json:"icon"`
	Description string `json:"description"`
}

type owm3h struct {
	H3 float64 `json:"3h"`
}

type owmCurrent struct {
	Weather []owmWeather `json:"weather"`
	Main    struct {
		Temp      float64 `json:"temp"`
		FeelsLike float64 `json:"feels_like"`
		TempMax   float64 `json:"temp_max"`
		Humidity  int     `json:"humidity"`
	} `json:"main"`
	Wind struct {
		Speed float64  `json:"speed"`
		Gust  *float64 `json:"gust"`
	} `json:"wind"`
	Sys struct {
		Sunrise int64 `json:"sunrise"`
		Sunset  int64 `json:"sunset"`
	} `json:"sys"`
}

type owmForecast struct {
	List []struct {
		Dt   int64 `json:"dt"`
		Main struct {
			Temp float64 `json:"temp"`
		} `json:"main"`
		Weather []owmWeather `json:"weather"`
		Wind    struct {
			Speed float64  `json:"speed"`
			Gust  *float64 `json:"gust"`
		} `json:"wind"`
		Rain owm3h `json:"rain"`
		Snow owm3h `json:"snow"`
	} `json:"list"`
}

// queryURL builds an OWM 2.5 request URL. Purely numeric locations are city IDs.
func queryURL(endpoint, apiKey, location, lang string) string {
	q := url.Values{}
	loc := strings.TrimSpace(location)
	if _, err := strconv.ParseUint(loc, 10, 64); err == nil {
		q.Set("id", loc)
	} else {
		q.Set("q", loc)
	}
	return owmURL(owmBaseURL, endpoint, apiKey, q, lang)
}

// owmURL completes the place parameters q with key, units and language.
func owmURL(base, endpoint, apiKey string, q url.Values, lang string) string {
	q.Set("appid", apiKey)
	q.Set("units", "metric")
	q.Set("lang", lang)
	return base + "/" + endpoint + "?" + q.Encode()
}

func fetchCurrent(ctx context.Context, fc *fetch.Client, apiKey, location, lang string, loc *time.Location) (current, bool, error) {
	return getCurrent(ctx, fc, queryURL("weather", apiKey, location, lang), loc)
}

func getCurrent(ctx context.Context, fc *fetch.Client, u string, loc *time.Location) (current, bool, error) {
	var raw owmCurrent
	stale, err := fc.GetJSON(ctx, u, nil, &raw)
	if err != nil {
		return current{}, false, fmt.Errorf("weather: current: %w", err)
	}
	if len(raw.Weather) == 0 {
		return current{}, false, errors.New("weather: current: response has no weather entry")
	}
	wind := raw.Wind.Speed
	if raw.Wind.Gust != nil {
		wind = *raw.Wind.Gust
	}
	return current{
		Icon:        raw.Weather[0].Icon,
		Description: raw.Weather[0].Description,
		Temp:        raw.Main.Temp,
		FeelsLike:   raw.Main.FeelsLike,
		MaxTemp:     raw.Main.TempMax,
		Humidity:    raw.Main.Humidity,
		WindMPS:     wind,
		WindMean:    raw.Wind.Speed,
		Sunrise:     time.Unix(raw.Sys.Sunrise, 0).In(loc),
		Sunset:      time.Unix(raw.Sys.Sunset, 0).In(loc),
	}, stale, nil
}

func fetchForecast(ctx context.Context, fc *fetch.Client, apiKey, location, lang string, loc *time.Location) ([]forecastPoint, bool, error) {
	return getForecast(ctx, fc, queryURL("forecast", apiKey, location, lang), loc)
}

func getForecast(ctx context.Context, fc *fetch.Client, u string, loc *time.Location) ([]forecastPoint, bool, error) {
	var raw owmForecast
	stale, err := fc.GetJSON(ctx, u, nil, &raw)
	if err != nil {
		return nil, false, fmt.Errorf("weather: forecast: %w", err)
	}
	points := make([]forecastPoint, 0, len(raw.List))
	for _, f := range raw.List {
		if len(f.Weather) == 0 {
			continue
		}
		gust := f.Wind.Speed
		if f.Wind.Gust != nil {
			gust = *f.Wind.Gust
		}
		points = append(points, forecastPoint{
			Time:   time.Unix(f.Dt, 0).In(loc),
			Temp:   f.Main.Temp,
			Icon:   f.Weather[0].Icon,
			Wind:   f.Wind.Speed,
			Gust:   gust,
			Precip: f.Rain.H3 + f.Snow.H3,
		})
	}
	return points, stale, nil
}
