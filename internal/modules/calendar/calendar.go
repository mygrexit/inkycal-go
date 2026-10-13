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
	"inkycal/internal/i18n"
	"inkycal/internal/ical"
	"inkycal/internal/modules"
	"inkycal/internal/render"
	"inkycal/internal/timefmt"
)

type Module struct {
	common        config.Common
	weekStart     time.Weekday
	showEvents    bool
	urls          []string
	files         []string
	dateFormat    string
	timeFormat    string
	dedupe        bool
	labels        []string
	weekdayPrefix bool
}

type rawConfig struct {
	WeekStartsOn  string            `json:"week_starts_on"`
	ShowEvents    *config.FlexBool  `json:"show_events"`
	ICalURLs      config.StringList `json:"ical_urls"`
	ICalFiles     config.StringList `json:"ical_files"`
	DateFormat    string            `json:"date_format"`
	TimeFormat    string            `json:"time_format"`
	Dedupe        config.FlexBool   `json:"dedupe_events"`
	Labels        config.StringList `json:"calendar_labels"`
	WeekdayPrefix config.FlexBool   `json:"weekday_prefix"`
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
		common:        c,
		weekStart:     time.Monday,
		showEvents:    true,
		urls:          []string(rc.ICalURLs),
		files:         []string(rc.ICalFiles),
		dateFormat:    "D MMM",
		timeFormat:    "HH:mm",
		dedupe:        bool(rc.Dedupe),
		labels:        []string(rc.Labels),
		weekdayPrefix: bool(rc.WeekdayPrefix),
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
	l := computeLayout(w, h, int(m.common.FontSize), m.showEvents, len(weeks))

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

// drawGrid draws the weekday header and the month as a quiet grid of numbers:
// today in a red disc, days with events marked by a red dot, past days lighter.
func (m *Module) drawGrid(c *render.Canvas, l layout, weeks [][7]int, now time.Time, events []ical.Event) {
	lang := m.common.Language
	head := render.Style{Font: render.FontRegular, Size: l.Body * 4 / 5}
	first := weekStartDate(now, m.weekStart)
	for i := 0; i < 7; i++ {
		name := timefmt.Format(first.AddDate(0, 0, i), "ddd", lang)
		c.TextCenter(l.GridX+l.CellW*i+l.CellW/2, l.HeaderH-l.Body/2, name, head, render.InkBlack)
	}
	c.HLine(0, c.W-1, l.HeaderH-1, render.InkBlack)

	marked := map[int]bool{}
	for _, d := range eventDays(events, now.Year(), now.Month(), now.Location()) {
		marked[d] = true
	}
	future := render.Style{Font: render.FontDefault, Size: l.NumSize}
	past := render.Style{Font: render.FontRegular, Size: l.NumSize}
	bold := render.Style{Font: render.FontBold, Size: l.NumSize}
	capH := c.CapHeight(future)
	dotR := max(2, l.CellH/14)
	for wi, week := range weeks {
		for col, day := range week {
			if day == 0 {
				continue
			}
			x, y := l.cell(wi, col)
			cx := x + l.CellW/2
			baseline := y + (l.CellH+capH)/2 - dotR
			label := strconv.Itoa(day)
			switch {
			case day == now.Day():
				c.FillCircle(cx, baseline-capH/2, l.CellH*9/20, render.InkColour)
				c.TextCenter(cx, baseline, label, bold, render.InkWhite)
				continue
			case day < now.Day():
				c.TextCenter(cx, baseline, label, past, render.InkBlack)
			default:
				c.TextCenter(cx, baseline, label, future, render.InkBlack)
			}
			if marked[day] {
				c.FillCircle(cx, baseline+dotR*3, dotR, render.InkColour)
			}
		}
	}
}

// dayName is "Today", "Tomorrow" or the formatted date of an event group.
func dayName(day, now time.Time, formatted, lang string) string {
	today := dateOnly(now)
	switch dateOnly(day) {
	case today:
		return i18n.T(lang, "today")
	case today.AddDate(0, 0, 1):
		return i18n.T(lang, "tomorrow")
	}
	return formatted
}

// drawEvents lists the coming events grouped by day, aligned with the grid
// above. Today's group sits in a red frame; what does not fit is counted.
func (m *Module) drawEvents(c *render.Canvas, l layout, now time.Time, events []ical.Event) {
	if l.EventsH <= 0 {
		return
	}
	lang := m.common.Language
	body := render.Style{Font: render.FontRegular, Size: l.Body}
	title := render.Style{Font: render.FontDefault, Size: l.Body}
	small := render.Style{Font: render.FontRegular, Size: l.Body * 5 / 6}
	rowH := l.Body * 3 / 2
	groupGap := l.Body / 3
	inset := l.Body / 2 // keeps text clear of today's frame
	capH := c.CapHeight(body)

	merged := mergeSources(events, m.labels, m.dedupe, lang)
	rows := buildEventRows(merged, now, 200, dateFormat(m.dateFormat, m.weekdayPrefix), m.timeFormat, lang)
	if len(rows) == 0 {
		msg := i18n.T(lang, "no_events")
		c.Text(inset, l.EventsY+rowH-(rowH-capH)/2, msg, body, render.InkBlack)
		return
	}

	// Place rows top to bottom until the area is full (keeping a line for the overflow count).
	type placed struct {
		row      eventRow
		y        int
		newGroup bool
	}
	var out []placed
	y := l.EventsY + inset/2
	bottom := l.EventsY + l.EventsH
	for i, r := range rows {
		newGroup := i == 0 || r.Date != rows[i-1].Date
		if newGroup && i > 0 {
			y += groupGap
		}
		reserve := 0
		if i < len(rows)-1 {
			reserve = rowH
		}
		if y+rowH+reserve > bottom {
			break
		}
		out = append(out, placed{r, y, newGroup})
		y += rowH
	}

	dayW, timeW := 0, 0
	for _, p := range out {
		dayW = max(dayW, c.Advance(dayName(p.row.Day, now, p.row.Date, lang), title))
		timeW = max(timeW, c.Advance(p.row.Time, body))
	}
	gap := l.Body
	xDay, xTime := inset, inset+dayW+gap
	xTitle := xTime + timeW + gap
	firstToday, lastToday := -1, -1
	for i, p := range out {
		baseline := p.y + (rowH+capH)/2
		if p.newGroup {
			ink := render.InkBlack
			if p.row.Today {
				ink = render.InkColour
			}
			c.Text(xDay, baseline, dayName(p.row.Day, now, p.row.Date, lang), title, ink)
		}
		xt := xTime
		if !p.row.AllDay {
			c.Text(xTime, baseline, p.row.Time, body, render.InkBlack)
			xt = xTitle
		}
		drawTitle(c, xt, baseline, c.W-inset-xt, p.row.Title, p.row.Label, title, small)
		if p.row.Today {
			if firstToday < 0 {
				firstToday = i
			}
			lastToday = i
		}
	}
	if firstToday >= 0 {
		top := out[firstToday].y - inset/2
		h := out[lastToday].y + rowH + inset/2 - top
		for t := 0; t < 2; t++ { // 2 px
			c.RoundedRect(t, top+t, c.W-2*t, h-2*t, l.Body/2-t, render.InkColour)
		}
	}
	if rest := len(rows) - len(out); rest > 0 {
		more := i18n.T(lang, "more", rest)
		c.Text(xTime, y+(rowH+capH)/2, more, small, render.InkBlack)
	}
}

// drawTitle sets the title and, if set, the calendar label in red after it.
// A long title is shortened so the label always stays visible.
func drawTitle(c *render.Canvas, x, baseline, w int, title, label string, st, labelSt render.Style) {
	if label == "" {
		c.Text(x, baseline, c.Fit(title, st, w), st, render.InkBlack)
		return
	}
	gap := c.Advance(" ", st)
	title = c.Fit(title, st, w-gap-c.Advance(label, labelSt))
	tw := 0
	if title != "" {
		tw = c.Text(x, baseline, title, st, render.InkBlack) + gap
	}
	c.Text(x+tw, baseline, label, labelSt, render.InkColour)
}
