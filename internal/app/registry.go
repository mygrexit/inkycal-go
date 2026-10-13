package app

import (
	"inkycal/internal/modules"
	"inkycal/internal/modules/agenda"
	"inkycal/internal/modules/calendar"
	"inkycal/internal/modules/feeds"
	"inkycal/internal/modules/fullweather"
	"inkycal/internal/modules/greek"
	imagemod "inkycal/internal/modules/image"
	"inkycal/internal/modules/jokes"
	"inkycal/internal/modules/stocks"
	"inkycal/internal/modules/textfile"
	"inkycal/internal/modules/todoist"
	"inkycal/internal/modules/weather"
	"inkycal/internal/modules/xkcd"
)

// Registry maps the module names used in settings.json to their factories.
var Registry = map[string]modules.Factory{
	"Agenda":        agenda.New,
	"Calendar":      calendar.New,
	"Feeds":         feeds.New,
	"Fullweather":   fullweather.New,
	"Greek":         greek.New,
	"Inkyimage":     imagemod.NewImage,
	"Inkyserver":    imagemod.NewServer,
	"Jokes":         jokes.New,
	"Slideshow":     imagemod.NewSlideshow,
	"Stocks":        stocks.New,
	"TextToDisplay": textfile.New,
	"Todoist":       todoist.New,
	"Weather":       weather.New,
	"Xkcd":          xkcd.New,
}
