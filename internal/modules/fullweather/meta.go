package fullweather

import "inkycal/internal/modules"

var Meta = modules.Meta{
	Name: "Fullweather", Title: "Weather (full)", Description: "Current conditions, a temperature and rain chart and a 5-day forecast.",
	MinW: 400, MinH: 80,
	Fields: []modules.Field{
		{Key: "api_key", Label: "OpenWeatherMap API key", Kind: modules.KindSecret, Required: true},
		{Key: "latitude", Label: "Latitude", Kind: modules.KindNumber, Required: true},
		{Key: "longitude", Label: "Longitude", Kind: modules.KindNumber, Required: true},
		{Key: "api_version", Label: "API", Help: "3.0 needs a One Call subscription", Kind: modules.KindSelect, Options: []any{"2.5", "3.0"}, Default: "2.5"},
		{Key: "temp_unit", Label: "Temperature", Kind: modules.KindSelect, Options: []any{"celsius", "fahrenheit"}, Default: "celsius"},
		{Key: "wind_unit", Label: "Wind", Kind: modules.KindSelect, Options: []any{"meters_sec", "km_hour", "miles_hour", "knots", "beaufort"}, Default: "meters_sec"},
		{Key: "wind_gusts", Label: "Show gusts", Kind: modules.KindBool, Default: true},
		{Key: "min_max_annotations", Label: "Label highs and lows", Kind: modules.KindBool, Default: false},
		{Key: "hour_format", Label: "Clock", Kind: modules.KindSelect, Options: []any{24, 12}, Default: 24},
		{Key: "locale", Label: "Locale", Help: "e.g. de_DE.UTF-8; overrides the language", Kind: modules.KindText},
	},
}
