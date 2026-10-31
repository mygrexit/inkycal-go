package weather

import (
	"inkycal/internal/forecast"
	"inkycal/internal/modules"
)

var Meta = modules.Meta{
	Name: "Weather", Title: "Weather", Description: "Today: date, conditions, high/low, wind, sun times and a timeline.", Group: "weather", Icon: "🌤",
	MinW: 300, MinH: 60,
	Fields: []modules.Field{
		{Key: "location", Label: "Place", Help: "Type your town and pick it from the list", Kind: modules.KindPlace, Required: true},
		{Key: "provider", Label: "Weather service", Kind: modules.KindSelect, Options: []any{forecast.OpenMeteo, forecast.OpenWeatherMap},
			Labels:  map[string]string{forecast.OpenMeteo: "Open-Meteo (free, no account)", forecast.OpenWeatherMap: "OpenWeatherMap (needs an API key)"},
			Default: forecast.OpenMeteo, Advanced: true},
		{Key: "api_key", Label: "OpenWeatherMap API key", Kind: modules.KindSecret, Required: true,
			ShowIf: &modules.Cond{Key: "provider", Values: []any{forecast.OpenWeatherMap}}},
		{Key: "latitude", Label: "Latitude", Kind: modules.KindNumber, Required: true, Advanced: true,
			ShowIf: &modules.Cond{Key: "provider", Values: []any{forecast.OpenMeteo}}},
		{Key: "longitude", Label: "Longitude", Kind: modules.KindNumber, Required: true, Advanced: true,
			ShowIf: &modules.Cond{Key: "provider", Values: []any{forecast.OpenMeteo}}},
		{Key: "units", Label: "Units", Kind: modules.KindSelect, Options: []any{"metric", "imperial"}, Default: "metric"},
		{Key: "hour_format", Label: "Clock", Kind: modules.KindSelect, Options: []any{24, 12}, Default: 24},
		{Key: "forecast_interval", Label: "Forecast", Help: "Kept for upstream Inkycal; this design ignores it", Kind: modules.KindSelect, Options: []any{"daily", "hourly"}, Default: "daily", Advanced: true},
		{Key: "round_temperature", Label: "Round temperatures", Help: "Kept for upstream Inkycal; this design ignores it", Kind: modules.KindBool, Default: true, Advanced: true},
		{Key: "round_windspeed", Label: "Round wind speed", Help: "Kept for upstream Inkycal; this design ignores it", Kind: modules.KindBool, Default: true, Advanced: true},
		{Key: "use_beaufort", Label: "Wind in Beaufort", Help: "Kept for upstream Inkycal; this design ignores it", Kind: modules.KindBool, Default: false, Advanced: true},
	},
}
