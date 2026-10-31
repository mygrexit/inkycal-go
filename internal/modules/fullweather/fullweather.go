// Package fullweather ports inkycal_fullweather.py: a large weather view with the
// current conditions, a temperature/precipitation chart of the coming hours and a
// five-day forecast, all from OpenWeatherMap.
package fullweather

import (
	"context"
	"errors"
	"strconv"
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

var Def = modules.Define(Meta, func(c config.Common, rc rawConfig) (modules.Module, error) {
	if rc.Latitude == nil || rc.Longitude == nil {
		return nil, modules.Missing("latitude", "longitude")
	}
	lang := c.Language
	// Upstream takes the language from the locale, e.g. "de_DE.UTF-8".
	if parts := strings.FieldsFunc(rc.Locale, func(r rune) bool { return r == '_' || r == '-' || r == '.' || r == ' ' }); len(parts) > 0 {
		lang = strings.ToLower(parts[0])
	}
	v, _ := strconv.ParseFloat(strings.TrimSpace(string(rc.APIVersion)), 64)
	place := forecast.Settings{Provider: "openweathermap", APIKey: rc.APIKey, Latitude: rc.Latitude, Longitude: rc.Longitude}
	src, err := place.Source(lang, forecast.Want{Days: 5, UV: true, OneCall: v == 3})
	if err != nil {
		return nil, err
	}
	s := settings{
		Fahrenheit: strings.EqualFold(strings.TrimSpace(rc.TempUnit), "fahrenheit"),
		Wind:       parseWindUnit(rc.WindUnit),
		Gusts:      rc.WindGusts == nil || bool(*rc.WindGusts),
		MinMax:     bool(rc.MinMaxAnnotations),
		Hour12:     int(rc.HourFormat) == 12,
		Lang:       lang,
	}
	return &Module{common: c, src: src, set: s}, nil
})

func (m *Module) Generate(ctx context.Context, env *modules.Env) (*render.Canvas, error) {
	r, err := m.src.Fetch(ctx, env.Fetch, env.Loc)
	if err != nil {
		return nil, err
	}
	if len(r.Points) == 0 {
		return nil, errors.New("forecast is empty")
	}
	env.Stale = env.Stale || r.Stale

	w, h := m.common.InnerSize()
	c := env.Canvas(w, h, render.FontDefault, int(m.common.FontSize))
	draw(c, buildView(m.set, r.Now, r.Points, r.UV, env.Now))
	return c, nil
}
