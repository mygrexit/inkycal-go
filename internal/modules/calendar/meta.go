package calendar

import "inkycal/internal/modules"

var Meta = modules.Meta{
	Name: "Calendar", Title: "Calendar", Description: "Month grid with event dots and the coming events grouped by day.", Group: "calendar", Icon: "📅",
	MinW: 150, MinH: 100,
	Fields: []modules.Field{
		{Key: "ical_urls", Label: "Calendar URLs (iCal)", Kind: modules.KindURLList},
		{Key: "ical_files", Label: "Calendar files", Kind: modules.KindTextList, Advanced: true},
		{Key: "calendar_labels", Label: "Calendar labels", Help: "One short name per calendar, shown in red after the title", Kind: modules.KindTextList, Advanced: true},
		{Key: "dedupe_events", Label: "Merge identical events", Kind: modules.KindBool, Default: false, Advanced: true},
		{Key: "show_events", Label: "Show the event list", Kind: modules.KindBool, Default: true},
		{Key: "week_starts_on", Label: "Week starts on", Kind: modules.KindSelect, Options: []any{"Monday", "Sunday"}, Default: "Monday", Advanced: true},
		{Key: "date_format", Label: "Date", Kind: modules.KindSelect, Options: []any{"D MMM", "D. MMM", "ddd, D. MMM", "ddd D MMM", "DD.MM.", "dddd, D. MMMM", "MMM D"}, Custom: true, Default: "D MMM"},
		{Key: "weekday_prefix", Label: "Weekday before the date", Kind: modules.KindBool, Default: false, Advanced: true},
		{Key: "time_format", Label: "Time", Kind: modules.KindSelect, Options: []any{"HH:mm", "H:mm", "h:mm a"}, Custom: true, Default: "HH:mm"},
	},
}
