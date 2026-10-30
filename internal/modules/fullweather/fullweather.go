// Package fullweather ports inkycal_fullweather.py: a large weather view with the
// current conditions, a temperature/precipitation chart of the coming hours and a
// five-day forecast, all from OpenWeatherMap.
package fullweather

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/url"
	"strconv"
	"strings"
	"sync"

	"inkycal/internal/config"
	"inkycal/internal/fetch"
	"inkycal/internal/modules"
	"inkycal/internal/modules/weather"
	"inkycal/internal/problem"
	"inkycal/internal/render"
)

// Variables so tests can point them at an httptest server.
var (
	owmBaseURL     = weather.OWMBaseURL
	oneCallBaseURL = "https://api.openweathermap.org/data/3.0/onecall"
)

type Module struct {
	common   config.Common
	apiKey   string
	lat, lon float64
	oneCall  bool // api_version 3.0: also fetch the UV index
	set      settings
}

// rawConfig mirrors the upstream keys. orientation, keep_history, font, chart_title,
// weekly_title and icon_outline are accepted and ignored (see the package report).
type rawConfig struct {
	APIKey            string            `json:"api_key"`
	Latitude          *config.FlexFloat `json:"latitude"`
	Longitude         *config.FlexFloat `json:"longitude"`
	APIVersion        config.FlexString `json:"api_version"`
	TempUnit          string            `json:"temp_unit"`
	WindUnit          string            `json:"wind_unit"`
	WindGusts         *config.FlexBool  `json:"wind_gusts"`
	MinMaxAnnotations config.FlexBool   `json:"min_max_annotations"`
	Locale            string            `json:"locale"`
	HourFormat        config.FlexInt    `json:"hour_format"`
}

func New(c config.Common, raw json.RawMessage) (modules.Module, error) {
	var rc rawConfig
	if err := json.Unmarshal(raw, &rc); err != nil {
		return nil, fmt.Errorf("fullweather: config: %w", err)
	}
	if strings.TrimSpace(rc.APIKey) == "" {
		return nil, problem.Mark(problem.Missing, errors.New("fullweather: api_key is missing"))
	}
	if rc.Latitude == nil || rc.Longitude == nil {
		return nil, problem.Mark(problem.Missing, errors.New("fullweather: latitude and longitude are required"))
	}
	lat, lon := float64(*rc.Latitude), float64(*rc.Longitude)
	if lat < -90 || lat > 90 || lon < -180 || lon > 180 {
		return nil, fmt.Errorf("fullweather: latitude/longitude %v/%v out of range", lat, lon)
	}
	if err := modules.CheckMinSize(c, Meta); err != nil {
		return nil, err
	}
	lang := c.Language
	// Upstream takes the language from the locale, e.g. "de_DE.UTF-8".
	if parts := strings.FieldsFunc(rc.Locale, func(r rune) bool { return r == '_' || r == '-' || r == '.' || r == ' ' }); len(parts) > 0 {
		lang = strings.ToLower(parts[0])
	}
	v, _ := strconv.ParseFloat(strings.TrimSpace(string(rc.APIVersion)), 64)
	s := settings{
		Fahrenheit: strings.EqualFold(strings.TrimSpace(rc.TempUnit), "fahrenheit"),
		Wind:       parseWindUnit(rc.WindUnit),
		Gusts:      rc.WindGusts == nil || bool(*rc.WindGusts),
		MinMax:     bool(rc.MinMaxAnnotations),
		Hour12:     int(rc.HourFormat) == 12,
		Lang:       lang,
	}
	return &Module{common: c, apiKey: rc.APIKey, lat: lat, lon: lon, oneCall: v == 3, set: s}, nil
}

func (m *Module) Generate(ctx context.Context, env *modules.Env) (*render.Canvas, error) {
	var (
		wg             sync.WaitGroup
		cur            weather.Current
		points         []weather.ForecastPoint
		uvi            *float64
		staleC, staleF bool
		errC, errF     error
	)
	wg.Add(2)
	go func() {
		defer wg.Done()
		u := weather.CoordsURL(owmBaseURL, "weather", m.apiKey, m.lat, m.lon, m.set.Lang)
		cur, staleC, errC = weather.FetchCurrent(ctx, env.Fetch, u, env.Loc)
	}()
	go func() {
		defer wg.Done()
		u := weather.CoordsURL(owmBaseURL, "forecast", m.apiKey, m.lat, m.lon, m.set.Lang)
		points, staleF, errF = weather.FetchForecast(ctx, env.Fetch, u, env.Loc)
	}()
	if m.oneCall {
		wg.Go(func() {
			uvi = fetchUVI(ctx, env.Fetch, m.apiKey, m.lat, m.lon)
		})
	}
	wg.Wait()
	if err := errors.Join(errC, errF); err != nil {
		return nil, fmt.Errorf("fullweather: %w", err)
	}
	if len(points) == 0 {
		return nil, errors.New("fullweather: forecast is empty")
	}
	env.Stale = env.Stale || staleC || staleF

	w, h := m.common.InnerSize()
	c := env.Canvas(w, h, render.FontDefault, int(m.common.FontSize))
	draw(c, buildView(m.set, cur, points, uvi, env.Now))
	return c, nil
}

// fetchUVI reads the current UV index from the One Call 3.0 API. It is an extra the
// view can do without, so a failure only drops the UV line.
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
		slog.Warn("fullweather: UV index unavailable", "stale", stale, "err", err)
		return nil
	}
	return raw.Current.UVI
}
