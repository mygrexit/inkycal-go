// Package weather ports inkycal_weather.py: current weather plus four forecasts from OpenWeatherMap.
package weather

import (
	"context"
	"errors"
	"strings"
	"sync"

	"inkycal/internal/config"
	"inkycal/internal/modules"
	"inkycal/internal/render"
)

type Module struct {
	common   config.Common
	apiKey   string
	location string
	provider string // "open-meteo" or "openweathermap"
	lat, lon float64
	set      settings
}

type rawConfig struct {
	Provider         string            `json:"provider"` // "" = OpenWeatherMap with an api_key, else Open-Meteo
	Latitude         *config.FlexFloat `json:"latitude"`
	Longitude        *config.FlexFloat `json:"longitude"`
	APIKey           string            `json:"api_key"`
	Location         config.FlexString `json:"location"`
	RoundTemperature config.FlexBool   `json:"round_temperature"`
	RoundWindspeed   config.FlexBool   `json:"round_windspeed"`
	ForecastInterval string            `json:"forecast_interval"`
	Units            string            `json:"units"`
	HourFormat       config.FlexInt    `json:"hour_format"`
	UseBeaufort      config.FlexBool   `json:"use_beaufort"`
}

var Def = modules.Define(Meta, func(c config.Common, rc rawConfig) (modules.Module, error) {
	provider := rc.Provider
	if provider == "" {
		provider = "open-meteo"
		if strings.TrimSpace(rc.APIKey) != "" {
			provider = "openweathermap" // settings from upstream Inkycal
		}
	}
	m := &Module{common: c, apiKey: rc.APIKey, location: string(rc.Location), provider: provider}
	switch provider {
	case "openweathermap":
		if strings.TrimSpace(rc.APIKey) == "" {
			return nil, modules.Missing("api_key")
		}
		if strings.TrimSpace(string(rc.Location)) == "" {
			return nil, modules.Missing("location")
		}
	case "open-meteo":
		if rc.Latitude == nil || rc.Longitude == nil {
			return nil, modules.Missing("latitude", "longitude") // the builder's place search writes them
		}
		m.lat, m.lon = float64(*rc.Latitude), float64(*rc.Longitude)
	default:
		return nil, modules.Bad("provider %q: want open-meteo or openweathermap", provider)
	}
	s := settings{
		Fahrenheit: strings.EqualFold(rc.Units, "imperial"),
		Hour12:     int(rc.HourFormat) == 12,
		Lang:       c.Language,
	}
	m.set = s
	return m, nil
})

func (m *Module) Generate(ctx context.Context, env *modules.Env) (*render.Canvas, error) {
	var (
		wg             sync.WaitGroup
		cur            current
		points         []forecastPoint
		staleC, staleF bool
		errC, errF     error
	)
	if m.provider == "open-meteo" {
		cur, points, staleC, errC = fetchOpenMeteo(ctx, env.Fetch, m.lat, m.lon, m.set.Lang, env.Loc)
	} else {
		wg.Go(func() {
			points, staleF, errF = fetchForecast(ctx, env.Fetch, m.apiKey, m.location, m.set.Lang, env.Loc)
		})
		cur, staleC, errC = fetchCurrent(ctx, env.Fetch, m.apiKey, m.location, m.set.Lang, env.Loc)
	}
	wg.Wait()
	if err := errors.Join(errC, errF); err != nil {
		return nil, err
	}
	env.Stale = env.Stale || staleC || staleF

	w, h := m.common.InnerSize()
	c := env.Canvas(w, h, render.FontDefault, int(m.common.FontSize))
	drawDay(c, buildDay(m.set, cur, points, env.Now), m.set.Lang)
	return c, nil
}
