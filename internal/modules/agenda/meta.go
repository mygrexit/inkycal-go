package agenda

import "inkycal/internal/modules"

var Meta = modules.Meta{
	Name: "Agenda", Title: "Agenda", Description: "The coming days as a list of events under date headers.",
	MinW: 100, MinH: 40,
	Fields: []modules.Field{
		{Key: "ical_urls", Label: "Calendar URLs (iCal)", Kind: modules.KindURLList},
		{Key: "ical_files", Label: "Calendar files", Kind: modules.KindTextList},
		{Key: "date_format", Label: "Date format", Kind: modules.KindText, Default: "ddd D MMM"},
		{Key: "time_format", Label: "Time format", Kind: modules.KindText, Default: "HH:mm"},
		{Key: "columns", Label: "Columns", Kind: modules.KindSelect, Options: []any{1, 2}, Default: 1},
	},
}
