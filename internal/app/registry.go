package app

import (
	"sort"

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

// Catalog describes every registered module for the builder, sorted by title.
var Catalog = sortedCatalog(
	agenda.Meta, calendar.Meta, feeds.Meta, fullweather.Meta, greek.Meta,
	imagemod.ImageMeta, imagemod.ServerMeta, jokes.Meta, imagemod.SlideshowMeta,
	stocks.Meta, textfile.Meta, todoist.Meta, weather.Meta, xkcd.Meta,
)

func sortedCatalog(ms ...modules.Meta) []modules.Meta {
	sort.Slice(ms, func(i, j int) bool { return ms[i].Title < ms[j].Title })
	return ms
}
