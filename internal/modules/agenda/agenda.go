// Package agenda lists upcoming iCalendar events grouped by day (port of inkycal_agenda.py).
package agenda

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"time"

	"inkycal/internal/config"
	"inkycal/internal/ical"
	"inkycal/internal/modules"
	"inkycal/internal/render"
	"inkycal/internal/timefmt"
)

type settings struct {
	ICalURLs   config.StringList `json:"ical_urls"`
	ICalFiles  config.StringList `json:"ical_files"`
	DateFormat string            `json:"date_format"`
	TimeFormat string            `json:"time_format"`
	Columns    config.FlexInt    `json:"columns"`
}

type Agenda struct {
	common config.Common
	s      settings
}

// New validates the config; defaults match the Python module.
func New(c config.Common, raw json.RawMessage) (modules.Module, error) {
	var s settings
	if err := json.Unmarshal(raw, &s); err != nil {
		return nil, fmt.Errorf("agenda: %w", err)
	}
	if s.DateFormat == "" {
		s.DateFormat = "ddd D MMM"
	}
	if s.TimeFormat == "" {
		s.TimeFormat = "HH:mm"
	}
	if s.Columns == 0 {
		s.Columns = 1
	}
	if s.Columns < 1 || s.Columns > 2 {
		return nil, fmt.Errorf("agenda: columns must be 1 or 2, got %d", s.Columns)
	}
	if err := modules.CheckMinSize(c, Meta); err != nil {
		return nil, err
	}
	return &Agenda{common: c, s: s}, nil
}

type item struct {
	isDate bool
	title  string
	start  time.Time
	allDay bool
}

// buildItems merges one header per day with the events; events that started before today
// sort under today's header, headers come before events of the same time.
func buildItems(today time.Time, days int, events []ical.Event, dateFmt, lang string) []item {
	var items []item
	for i := 0; i < days; i++ {
		d := today.AddDate(0, 0, i)
		items = append(items, item{isDate: true, title: timefmt.Format(d, dateFmt, lang), start: d})
	}
	for _, e := range events {
		items = append(items, item{title: e.Title, start: e.Start, allDay: e.AllDay})
	}
	key := func(it item) time.Time {
		if it.start.Before(today) {
			return today
		}
		return it.start
	}
	sort.SliceStable(items, func(i, j int) bool {
		ki, kj := key(items[i]), key(items[j])
		if !ki.Equal(kj) {
			return ki.Before(kj)
		}
		return items[i].isDate && !items[j].isDate
	})
	return items
}

type row struct {
	isDate bool
	text   string
	start  time.Time
	allDay bool
	col    int
	y, h   int
}

// layout places items into columns: a date header is never left alone at the bottom of a
// column, and a column that starts with an event repeats the current date header.
func layout(items []item, cv *render.Canvas, lh, colH, cols, titleWidth int) []row {
	height := func(it item) int {
		if it.isDate {
			return lh
		}
		return max(1, len(cv.Wrap(it.title, titleWidth))) * lh
	}
	var rows []row
	col, y := 0, 0
	lastDate := ""
	for i, it := range items {
		h := height(it)
		force := it.isDate && i+1 < len(items) && !items[i+1].isDate && y+h+height(items[i+1]) > colH
		if force || y+h > colH {
			col++
			y = 0
			if col >= cols {
				break
			}
			if !it.isDate && lastDate != "" {
				rows = append(rows, row{isDate: true, text: lastDate, col: col, y: 0, h: lh})
				y = lh
			}
		}
		if it.isDate {
			lastDate = it.title
		}
		rows = append(rows, row{isDate: it.isDate, text: it.title, start: it.start, allDay: it.allDay, col: col, y: y, h: h})
		y += h
	}
	return rows
}

func (a *Agenda) Generate(ctx context.Context, env *modules.Env) (*render.Canvas, error) {
	w, h := a.common.InnerSize()
	lang := a.common.Language
	cv := render.NewCanvas(w, h, render.FontDefault, int(a.common.FontSize))
	lh := cv.LineHeight() + 1
	cols := int(a.s.Columns)
	gutter := 0
	if cols > 1 {
		gutter = 10
	}
	colW := (w - (cols-1)*gutter) / cols
	days := (h / lh) * cols

	y, m, d := env.Now.Date()
	today := time.Date(y, m, d, 0, 0, 0, 0, env.Loc)
	events, stale, err := ical.Load(ctx, env.Fetch, a.s.ICalURLs, a.s.ICalFiles, today, today.AddDate(0, 0, days-1), env.Loc)
	if err != nil {
		return nil, err
	}
	env.Stale = stale

	items := buildItems(today, days, events, a.s.DateFormat, lang)
	dateWidth := 0
	for _, it := range items {
		if it.isDate {
			dateWidth = max(dateWidth, cv.TextWidth(it.title))
		}
	}
	dateWidth = min(dateWidth, colW)
	timeWidth := 0
	for _, e := range events {
		timeWidth = max(timeWidth, cv.TextWidth(timefmt.Format(e.Start, a.s.TimeFormat, lang)))
	}
	timeWidth += 10
	xTime := dateWidth / 3
	xEvent := xTime + timeWidth
	const bullet = " • "
	bulletWidth := cv.TextWidth(bullet)
	titleWidth := colW - xEvent - bulletWidth

	left := render.TextOpts{Align: render.AlignLeft}
	for _, r := range layout(items, cv, lh, h, cols, titleWidth) {
		x0 := r.col * (colW + gutter)
		if r.isDate {
			cv.HLine(x0, x0+colW-1, r.y, render.InkColour)
			cv.Write(x0, r.y, dateWidth, r.h, r.text, left)
			continue
		}
		if r.allDay {
			cv.Icon(x0+xTime+timeWidth-lh, r.y, lh, lh, "", render.FontMaterial, render.InkBlack, 0.8)
		} else {
			cv.Write(x0+xTime, r.y, timeWidth, lh, timefmt.Format(r.start, a.s.TimeFormat, lang), render.TextOpts{Align: render.AlignRight})
		}
		cv.Write(x0+xEvent, r.y, bulletWidth, lh, bullet, left)
		cv.Write(x0+xEvent+bulletWidth, r.y, titleWidth, r.h, r.text, left)
	}
	return cv, nil
}
