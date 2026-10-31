package forecast

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"inkycal/internal/fetch"
)

const (
	owmBaseURL     = "https://api.openweathermap.org/data/2.5"
	oneCallBaseURL = "https://api.openweathermap.org/data/3.0/onecall"
)

// Current is the weather now (metric units, OpenWeatherMap icon codes).
type Current struct {
	Icon        string
	Description string // localised condition text, e.g. "leichter Regen"
	Temp        float64
	FeelsLike   float64
	Humidity    int
	WindMPS     float64 // gust if reported, else mean speed (as in Python)
	WindMean    float64 // mean speed, m/s
	Sunrise     time.Time
	Sunset      time.Time // both zero when the service has none (polar day or night)
}

// Point is one 3-hour forecast slot (metric units).
type Point struct {
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

// owm reads OpenWeatherMap 2.5 by place (name or city id) or else by coordinates.
type owm struct {
	apiKey, place, lang string
	lat, lon            float64
	oneCall             bool // the key may use One Call 3.0 (for the UV index)
}

// url builds a request URL for endpoint ("weather", "forecast"). Purely numeric
// places are city IDs.
func (o owm) url(endpoint string) string {
	q := url.Values{}
	if place := strings.TrimSpace(o.place); place == "" {
		q.Set("lat", strconv.FormatFloat(o.lat, 'f', -1, 64))
		q.Set("lon", strconv.FormatFloat(o.lon, 'f', -1, 64))
	} else if _, err := strconv.ParseUint(place, 10, 64); err == nil {
		q.Set("id", place)
	} else {
		q.Set("q", place)
	}
	q.Set("appid", o.apiKey)
	q.Set("units", "metric")
	q.Set("lang", o.lang)
	return owmBaseURL + "/" + endpoint + "?" + q.Encode()
}

// fetch loads current conditions, the forecast and, when wanted, the UV index in parallel.
func (o owm) fetch(ctx context.Context, fc *fetch.Client, loc *time.Location, w Want) (Report, error) {
	var (
		wg             sync.WaitGroup
		r              Report
		staleC, staleF bool
		errC, errF     error
	)
	wg.Go(func() { r.Now, staleC, errC = getCurrent(ctx, fc, o.url("weather"), loc) })
	wg.Go(func() { r.Points, staleF, errF = getForecast(ctx, fc, o.url("forecast"), loc) })
	if w.UV && o.oneCall {
		wg.Go(func() { r.UV = fetchUVI(ctx, fc, o.apiKey, o.lat, o.lon) })
	}
	wg.Wait()
	if err := errors.Join(errC, errF); err != nil {
		return Report{}, err
	}
	r.Stale = staleC || staleF
	return r, nil
}

func getCurrent(ctx context.Context, fc *fetch.Client, u string, loc *time.Location) (Current, bool, error) {
	var raw owmCurrent
	stale, err := fc.GetJSON(ctx, u, nil, &raw)
	if err != nil {
		return Current{}, false, fmt.Errorf("current: %w", err)
	}
	if len(raw.Weather) == 0 {
		return Current{}, false, errors.New("current: response has no weather entry")
	}
	wind := raw.Wind.Speed
	if raw.Wind.Gust != nil {
		wind = *raw.Wind.Gust
	}
	cur := Current{
		Icon:        raw.Weather[0].Icon,
		Description: raw.Weather[0].Description,
		Temp:        raw.Main.Temp,
		FeelsLike:   raw.Main.FeelsLike,
		Humidity:    raw.Main.Humidity,
		WindMPS:     wind,
		WindMean:    raw.Wind.Speed,
	}
	if raw.Sys.Sunrise != 0 && raw.Sys.Sunset != 0 { // 0 in polar day or night
		cur.Sunrise, cur.Sunset = time.Unix(raw.Sys.Sunrise, 0).In(loc), time.Unix(raw.Sys.Sunset, 0).In(loc)
	}
	return cur, stale, nil
}

func getForecast(ctx context.Context, fc *fetch.Client, u string, loc *time.Location) ([]Point, bool, error) {
	var raw owmForecast
	stale, err := fc.GetJSON(ctx, u, nil, &raw)
	if err != nil {
		return nil, false, fmt.Errorf("forecast: %w", err)
	}
	points := make([]Point, 0, len(raw.List))
	for _, f := range raw.List {
		if len(f.Weather) == 0 {
			continue
		}
		gust := f.Wind.Speed
		if f.Wind.Gust != nil {
			gust = *f.Wind.Gust
		}
		points = append(points, Point{
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

// fetchUVI reads the current UV index from the One Call 3.0 API. It is an extra the
// view can do without, so a failure only drops it.
func fetchUVI(ctx context.Context, fc *fetch.Client, apiKey string, lat, lon float64) *float64 {
	q := url.Values{}
	q.Set("lat", strconv.FormatFloat(lat, 'f', -1, 64))
	q.Set("lon", strconv.FormatFloat(lon, 'f', -1, 64))
	q.Set("appid", apiKey)
	q.Set("exclude", "minutely,hourly,daily,alerts")
	q.Set("units", "metric")
	var raw struct {
		Current struct {
			UVI *float64 `json:"uvi"`
		} `json:"current"`
	}
	stale, err := fc.GetJSON(ctx, oneCallBaseURL+"?"+q.Encode(), nil, &raw)
	if err != nil || stale {
		// A cached UV index can be from yesterday noon: better none than a wrong one.
		slog.Warn("UV index unavailable", "stale", stale, "err", err)
		return nil
	}
	return raw.Current.UVI
}
