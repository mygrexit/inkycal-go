package weather

import "inkycal/internal/modules"

var Meta = modules.Meta{
	Name: "Weather", Title: "Weather", Description: "Today: date, conditions, high/low, wind, sun times and a timeline.",
	MinW: 300, MinH: 60,
	Fields: []modules.Field{
		{Key: "api_key", Label: "OpenWeatherMap API key", Kind: modules.KindSecret, Required: true},
		{Key: "location", Label: "Location", Help: "City ID or name, e.g. 2643743 or Berlin,DE", Kind: modules.KindText, Required: true},
		{Key: "units", Label: "Units", Kind: modules.KindSelect, Options: []any{"metric", "imperial"}, Default: "metric"},
		{Key: "hour_format", Label: "Clock", Kind: modules.KindSelect, Options: []any{24, 12}, Default: 24},
		{Key: "forecast_interval", Label: "Forecast", Help: "Kept for upstream Inkycal; this design ignores it", Kind: modules.KindSelect, Options: []any{"daily", "hourly"}, Default: "daily"},
		{Key: "round_temperature", Label: "Round temperatures", Help: "Kept for upstream Inkycal; this design ignores it", Kind: modules.KindBool, Default: true},
		{Key: "round_windspeed", Label: "Round wind speed", Help: "Kept for upstream Inkycal; this design ignores it", Kind: modules.KindBool, Default: true},
		{Key: "use_beaufort", Label: "Wind in Beaufort", Help: "Kept for upstream Inkycal; this design ignores it", Kind: modules.KindBool, Default: false},
	},
}
