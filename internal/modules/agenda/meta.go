package agenda

import "inkycal/internal/modules"

var Meta = modules.Meta{
	Name: "Agenda", Title: "Agenda", Description: "The coming days as a list of events under date headers.", Group: "calendar", Icon: "🗓",
	MinW: 100, MinH: 40,
	Fields: []modules.Field{
		{Key: "ical_urls", Label: "Calendar URLs (iCal)", Kind: modules.KindURLList},
		{Key: "ical_files", Label: "Calendar files", Kind: modules.KindTextList, Advanced: true},
		{Key: "date_format", Label: "Date", Kind: modules.KindSelect, Options: []any{"D MMM", "D. MMM", "ddd, D. MMM", "ddd D MMM", "DD.MM.", "dddd, D. MMMM", "MMM D"}, Custom: true, Default: "ddd D MMM"},
		{Key: "time_format", Label: "Time", Kind: modules.KindSelect, Options: []any{"HH:mm", "H:mm", "h:mm a"}, Custom: true, Default: "HH:mm"},
		{Key: "columns", Label: "Columns", Kind: modules.KindSelect, Options: []any{1, 2}, Default: 1},
	},
}
