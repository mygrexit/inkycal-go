// Package weather ports inkycal_weather.py: current weather plus four forecasts, from
// OpenWeatherMap or Open-Meteo (internal/forecast).
package weather

import (
	"context"
	"strings"

	"inkycal/internal/config"
	"inkycal/internal/forecast"
	"inkycal/internal/modules"
	"inkycal/internal/render"
)

type Module struct {
	common config.Common
	src    forecast.Source
	set    settings
}

type rawConfig struct {
	forecast.Settings
	RoundTemperature config.FlexBool `json:"round_temperature"`
	RoundWindspeed   config.FlexBool `json:"round_windspeed"`
	ForecastInterval string          `json:"forecast_interval"`
	Units            string          `json:"units"`
	HourFormat       config.FlexInt  `json:"hour_format"`
	UseBeaufort      config.FlexBool `json:"use_beaufort"`
}

var Def = modules.Define(Meta, func(c config.Common, rc rawConfig) (modules.Module, error) {
	src, err := rc.Source(c.Language, forecast.Want{})
	if err != nil {
		return nil, err
	}
	s := settings{
		Fahrenheit: strings.EqualFold(rc.Units, "imperial"),
		Hour12:     int(rc.HourFormat) == 12,
		Lang:       c.Language,
	}
	return &Module{common: c, src: src, set: s}, nil
})

func (m *Module) Generate(ctx context.Context, env *modules.Env) (*render.Canvas, error) {
	r, err := m.src.Fetch(ctx, env.Fetch, env.Loc)
	if err != nil {
		return nil, err
	}
	env.Stale = env.Stale || r.Stale

	w, h := m.common.InnerSize()
	c := env.Canvas(w, h, render.FontDefault, int(m.common.FontSize))
	drawDay(c, buildDay(m.set, r.Now, r.Points, env.Now), m.set.Lang)
	return c, nil
}
