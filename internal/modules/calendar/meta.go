package calendar

import "inkycal/internal/modules"

var Meta = modules.Meta{
	Name: "Calendar", Title: "Calendar", Description: "Month grid with event dots and the coming events grouped by day.",
	MinW: 150, MinH: 100,
	Fields: []modules.Field{
		{Key: "ical_urls", Label: "Calendar URLs (iCal)", Kind: modules.KindURLList},
		{Key: "ical_files", Label: "Calendar files", Kind: modules.KindTextList},
		{Key: "calendar_labels", Label: "Calendar labels", Help: "One short name per calendar, shown in red after the title", Kind: modules.KindTextList},
		{Key: "dedupe_events", Label: "Merge identical events", Kind: modules.KindBool, Default: false},
		{Key: "show_events", Label: "Show the event list", Kind: modules.KindBool, Default: true},
		{Key: "week_starts_on", Label: "Week starts on", Kind: modules.KindSelect, Options: []any{"Monday", "Sunday"}, Default: "Monday"},
		{Key: "date_format", Label: "Date format", Help: "Tokens: D DD ddd dddd MMM MMMM YYYY", Kind: modules.KindText, Default: "D MMM"},
		{Key: "weekday_prefix", Label: "Weekday before the date", Kind: modules.KindBool, Default: false},
		{Key: "time_format", Label: "Time format", Kind: modules.KindText, Default: "HH:mm"},
	},
}
