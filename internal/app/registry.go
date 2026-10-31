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

// Defs are the registered modules.
var Defs = []modules.Def{
	agenda.Def, calendar.Def, feeds.Def, fullweather.Def, greek.Def,
	imagemod.ImageDef, imagemod.ServerDef, jokes.Def, imagemod.SlideshowDef,
	stocks.Def, textfile.Def, todoist.Def, weather.Def, xkcd.Def,
}

// Registry maps the module names used in settings.json to their factories.
var Registry = func() map[string]modules.Factory {
	r := make(map[string]modules.Factory, len(Defs))
	for _, d := range Defs {
		r[d.Meta.Name] = d.New
	}
	return r
}()

// Catalog describes every registered module for the builder, sorted by title.
var Catalog = func() []modules.Meta {
	ms := make([]modules.Meta, len(Defs))
	for i, d := range Defs {
		ms[i] = d.Meta
	}
	sort.Slice(ms, func(i, j int) bool { return ms[i].Title < ms[j].Title })
	return ms
}()
