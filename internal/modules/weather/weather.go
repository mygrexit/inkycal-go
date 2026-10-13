// Package weather ports inkycal_weather.py: current weather plus four forecasts from OpenWeatherMap.
package weather

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
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
	set      settings
}

type rawConfig struct {
	APIKey           string          `json:"api_key"`
	Location         flexString      `json:"location"`
	RoundTemperature config.FlexBool `json:"round_temperature"`
	RoundWindspeed   config.FlexBool `json:"round_windspeed"`
	ForecastInterval string          `json:"forecast_interval"`
	Units            string          `json:"units"`
	HourFormat       config.FlexInt  `json:"hour_format"`
	UseBeaufort      config.FlexBool `json:"use_beaufort"`
}

// flexString accepts a JSON string or number; the web UI stores city IDs as numbers.
type flexString string

func (f *flexString) UnmarshalJSON(b []byte) error {
	var s string
	if err := json.Unmarshal(b, &s); err == nil {
		*f = flexString(s)
		return nil
	}
	var n json.Number
	if err := json.Unmarshal(b, &n); err != nil {
		return fmt.Errorf("want string or number, got %s", b)
	}
	*f = flexString(n.String())
	return nil
}

func New(c config.Common, raw json.RawMessage) (modules.Module, error) {
	var rc rawConfig
	if err := json.Unmarshal(raw, &rc); err != nil {
		return nil, fmt.Errorf("weather: config: %w", err)
	}
	if strings.TrimSpace(rc.APIKey) == "" {
		return nil, errors.New("weather: api_key is missing")
	}
	if strings.TrimSpace(string(rc.Location)) == "" {
		return nil, errors.New("weather: location is missing")
	}
	if w, h := c.InnerSize(); w < 400 || h < 120 {
		return nil, fmt.Errorf("weather: inner size %dx%d too small (min 400x120)", w, h)
	}
	s := settings{
		Fahrenheit: strings.EqualFold(rc.Units, "imperial"),
		Hour12:     int(rc.HourFormat) == 12,
		Lang:       c.Language,
	}
	return &Module{common: c, apiKey: rc.APIKey, location: string(rc.Location), set: s}, nil
}

func (m *Module) Generate(ctx context.Context, env *modules.Env) (*render.Canvas, error) {
	var (
		wg             sync.WaitGroup
		cur            current
		points         []forecastPoint
		staleC, staleF bool
		errC, errF     error
	)
	wg.Add(2)
	go func() {
		defer wg.Done()
		cur, staleC, errC = fetchCurrent(ctx, env.Fetch, m.apiKey, m.location, m.set.Lang, env.Loc)
	}()
	go func() {
		defer wg.Done()
		points, staleF, errF = fetchForecast(ctx, env.Fetch, m.apiKey, m.location, m.set.Lang, env.Loc)
	}()
	wg.Wait()
	if err := errors.Join(errC, errF); err != nil {
		return nil, err
	}
	env.Stale = env.Stale || staleC || staleF

	w, h := m.common.InnerSize()
	c := render.NewCanvas(w, h, render.FontDefault, int(m.common.FontSize))
	drawDay(c, buildDay(m.set, cur, points, env.Now), m.set.Lang)
	return c, nil
}
