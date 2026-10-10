package app

import (
	"inkycal/internal/modules"
	"inkycal/internal/modules/agenda"
	"inkycal/internal/modules/calendar"
	"inkycal/internal/modules/feeds"
	"inkycal/internal/modules/todoist"
	"inkycal/internal/modules/weather"
)

// Registry maps the module names used in settings.json to their factories.
var Registry = map[string]modules.Factory{
	"Agenda":   agenda.New,
	"Calendar": calendar.New,
	"Feeds":    feeds.New,
	"Todoist":  todoist.New,
	"Weather":  weather.New,
}
