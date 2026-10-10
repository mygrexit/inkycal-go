// Package calendar ports inkycal_calendar.py: month grid plus upcoming events.
package calendar

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"inkycal/internal/config"
	"inkycal/internal/ical"
	"inkycal/internal/modules"
	"inkycal/internal/render"
	"inkycal/internal/timefmt"
)

type Module struct {
	common     config.Common
	weekStart  time.Weekday
	showEvents bool
	urls       []string
	files      []string
	dateFormat string
	timeFormat string
}

type rawConfig struct {
	WeekStartsOn string            `json:"week_starts_on"`
	ShowEvents   *config.FlexBool  `json:"show_events"`
	ICalURLs     config.StringList `json:"ical_urls"`
	ICalFiles    config.StringList `json:"ical_files"`
	DateFormat   string            `json:"date_format"`
	TimeFormat   string            `json:"time_format"`
}

func New(c config.Common, raw json.RawMessage) (modules.Module, error) {
	var rc rawConfig
	if err := json.Unmarshal(raw, &rc); err != nil {
		return nil, fmt.Errorf("calendar: config: %w", err)
	}
	if w, h := c.InnerSize(); w < 70 || h < 100 {
		return nil, fmt.Errorf("calendar: inner size %dx%d too small (min 70x100)", w, h)
	}
	m := &Module{
		common:     c,
		weekStart:  time.Monday,
		showEvents: true,
		urls:       []string(rc.ICalURLs),
		files:      []string(rc.ICalFiles),
		dateFormat: "D MMM",
		timeFormat: "HH:mm",
	}
	if strings.EqualFold(rc.WeekStartsOn, "Sunday") {
		m.weekStart = time.Sunday
	}
	if rc.ShowEvents != nil {
		m.showEvents = bool(*rc.ShowEvents)
	}
	if rc.DateFormat != "" {
		m.dateFormat = rc.DateFormat
	}
	if rc.TimeFormat != "" {
		m.timeFormat = rc.TimeFormat
	}
	return m, nil
}

func (m *Module) Generate(ctx context.Context, env *modules.Env) (*render.Canvas, error) {
	now := env.Now
	w, h := m.common.InnerSize()
	c := render.NewCanvas(w, h, render.FontDefault, int(m.common.FontSize))
	weeks := monthGrid(now.Year(), now.Month(), m.weekStart)
	l := computeLayout(w, h, c.LineHeight(), m.showEvents, len(weeks))

	var events []ical.Event
	if m.showEvents && (len(m.urls) > 0 || len(m.files) > 0) {
		// One load covers both the month (event borders) and the next four weeks (list).
		from := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, env.Loc)
		to := from.AddDate(0, 1, 0)
		if horizon := now.AddDate(0, 0, 28); horizon.After(to) {
			to = horizon
		}
		evs, stale, err := ical.Load(ctx, env.Fetch, m.urls, m.files, from, to, env.Loc)
		if err != nil {
			return nil, fmt.Errorf("calendar: %w", err)
		}
		env.Stale = env.Stale || stale
		events = evs
	}

	m.drawGrid(c, l, weeks, now, events)
	if m.showEvents {
		m.drawEvents(c, l, now, events)
	}
	return c, nil
}

func (m *Module) drawGrid(c *render.Canvas, l layout, weeks [][7]int, now time.Time, events []ical.Event) {
	lang := m.common.Language
	c.Write(0, 0, c.W, l.MonthH, timefmt.Format(now, "MMMM", lang), render.TextOpts{Autofit: true})

	first := weekStartDate(now, m.weekStart)
	for i := 0; i < 7; i++ {
		name := timefmt.Format(first.AddDate(0, 0, i), "ddd", lang)
		c.Write(l.GridX+l.CellW*i, l.MonthH, l.CellW, l.WeekdaysH, name, render.TextOpts{Autofit: true, FillHeight: 0.9})
	}

	_, baseSize := c.Font()
	pos := map[int][2]int{}
	c.SetFont(render.FontDefault, l.NumSize)
	for wi, week := range weeks {
		for col, day := range week {
			if day == 0 {
				continue
			}
			x, y := l.cell(wi, col)
			pos[day] = [2]int{x, y}
			if day != now.Day() {
				c.Write(x, y, l.CellW, l.CellH, strconv.Itoa(day), render.TextOpts{})
			}
		}
	}

	// Today: filled circle (red on the panel) with a white number.
	today := pos[now.Day()]
	c.FillCircle(today[0]+l.CellW/2, today[1]+l.CellH/2, l.CellW/4, render.InkColour)
	c.SetFont(render.FontRegular, l.NumSize)
	c.Write(today[0], today[1], l.CellW, l.CellH, strconv.Itoa(now.Day()), render.TextOpts{Ink: render.InkWhite})
	c.SetFont(render.FontDefault, baseSize)

	for _, day := range eventDays(events, now.Year(), now.Month(), now.Location()) {
		p := pos[day]
		render.Border(c, p[0], p[1], l.CellW, l.CellH, 6, 0.1, 0.1, render.InkColour)
	}
}

func (m *Module) drawEvents(c *render.Canvas, l layout, now time.Time, events []ical.Event) {
	const lineSpacing = 2
	lh := c.LineHeight() + lineSpacing
	maxLines := l.EventsH / (lh + lineSpacing)
	if maxLines <= 0 {
		return
	}
	offset := c.H - l.EventsH
	lineY := func(i int) int { return offset + int(float64(l.EventsH)/float64(maxLines)*float64(i)) }
	left := render.TextOpts{Align: render.AlignLeft}

	rows := buildEventRows(events, now, maxLines, m.dateFormat, m.timeFormat, m.common.Language)
	if len(rows) == 0 {
		dash := "- "
		n := c.W / max(1, c.TextWidth(dash))
		c.Write(0, lineY(0), c.W, lh, strings.Repeat(dash, n), left)
		return
	}

	dateW, timeW := 0, 0
	for _, r := range rows {
		dateW = max(dateW, int(float64(c.TextWidth(r.Date))*1.1))
		timeW = max(timeW, int(float64(c.TextWidth(r.Time))*1.1))
	}
	for i, r := range rows {
		y := lineY(i)
		c.Write(0, y, dateW, lh, r.Date, left)
		if r.AllDay {
			c.Write(dateW, y, c.W-dateW, lh, r.Title, left)
			continue
		}
		c.Write(dateW, y, timeW, lh, r.Time, left)
		c.Write(dateW+timeW, y, c.W-dateW-timeW, lh, r.Title, left)
	}
}
