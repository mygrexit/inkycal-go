package todoist

import (
	"fmt"
	"log/slog"
	"slices"
	"sort"
	"strings"
	"time"

	"inkycal/internal/i18n"
	"inkycal/internal/render"
	"inkycal/internal/timefmt"
)

// row is one task as drawn: a checkbox, the task and its due date.
type row struct {
	Name   string
	Due    string // "" without due date
	Urgent bool   // overdue or due today: red due label
	High   bool   // highest priority: red checkbox
}

// parseDue reads the date part of a Todoist due value ("2026-10-10", "2026-10-10T15:00:00Z", ...).
func parseDue(s string, loc *time.Location) (time.Time, error) {
	if len(s) < 10 {
		return time.Time{}, fmt.Errorf("due date %q too short", s)
	}
	return time.ParseInLocation("2006-01-02", s[:10], loc)
}

func midnight(t time.Time) time.Time {
	y, m, d := t.Date()
	return time.Date(y, m, d, 0, 0, 0, 0, t.Location())
}

// dueLabel words a due date the way one would say it: overdue, today, tomorrow,
// the weekday within the coming week, else the date.
func dueLabel(due, now time.Time, lang string) string {
	days := int(midnight(due).Sub(midnight(now)).Hours()/24 + 0.5)
	if midnight(due).Before(midnight(now)) {
		days = -1
	}
	switch {
	case days < 0:
		return i18n.T(lang, "due.overdue")
	case days == 0:
		return i18n.T(lang, "due.today")
	case days == 1:
		return i18n.T(lang, "due.tomorrow")
	case days < 7:
		return timefmt.Format(due, "ddd", lang)
	}
	return timefmt.Format(due, "D. MMM", lang)
}

// buildRows keeps the tasks of the selected projects and sorts them by urgency:
// overdue, today, upcoming, undated; equal dates by priority (4 = highest).
func buildRows(projects []project, tasks []task, filter []string, showPriority bool, now time.Time, lang string) ([]row, error) {
	keep := make(map[string]bool, len(projects))
	for _, p := range projects {
		if len(filter) == 0 || slices.Contains(filter, p.Name) {
			keep[p.ID] = true
		}
	}
	if len(filter) > 0 && len(keep) == 0 {
		return nil, fmt.Errorf("todoist: no project matches project_filter %q", strings.Join(filter, ","))
	}
	today := midnight(now)
	type item struct {
		row
		due      time.Time
		hasDue   bool
		priority int
	}
	var items []item
	for _, t := range tasks {
		if !keep[t.ProjectID] {
			continue
		}
		it := item{row: row{Name: t.Content, High: showPriority && t.Priority == 4}, priority: t.Priority}
		if t.Due != nil && t.Due.Date != "" {
			d, err := parseDue(t.Due.Date, now.Location())
			if err != nil {
				slog.Warn("todoist: skipping task with unparsable due date", "task", t.Content, "due", t.Due.Date)
				continue
			}
			it.due, it.hasDue = d, true
			it.Due = dueLabel(d, now, lang)
			it.Urgent = !d.After(today)
		}
		items = append(items, it)
	}
	sort.SliceStable(items, func(i, j int) bool {
		a, b := items[i], items[j]
		if a.hasDue != b.hasDue {
			return a.hasDue
		}
		if !a.due.Equal(b.due) {
			return a.due.Before(b.due)
		}
		return a.priority > b.priority
	})
	rows := make([]row, len(items))
	for i, it := range items {
		rows[i] = it.row
	}
	return rows, nil
}

// drawRows sets the tasks in two columns, each line a checkbox, the task and its
// due date on the right; one full-width column when they fit in one or the tile is
// narrow. What does not fit is counted in the last line.
func drawRows(c *render.Canvas, rows []row, body int, lang string) {
	name := render.Style{Font: render.FontDefault, Size: body}
	small := render.Style{Font: render.FontRegular, Size: body * 5 / 6}
	rowH := body * 8 / 5
	perCol := max(1, c.H/rowH)
	colGap := body * 2
	cols, colW := 2, (c.W-colGap)/2
	if len(rows) <= perCol || colW < body*12 {
		cols, colW = 1, c.W
	}
	capH := c.CapHeight(name)
	box := capH + capH/4

	if len(rows) == 0 {
		msg := i18n.T(lang, "all_done")
		c.Text(0, (rowH+capH)/2, msg, name, render.InkBlack)
		return
	}
	capacity := cols * perCol
	shown := rows
	if len(rows) > capacity {
		shown = rows[:capacity-1]
	}
	for i, r := range shown {
		x := (i / perCol) * (colW + colGap)
		y := (i % perCol) * rowH
		baseline := y + (rowH+capH)/2
		ink := render.InkBlack
		if r.High {
			ink = render.InkColour
		}
		for t := range 2 { // 2 px checkbox
			c.RoundedRect(x+t, baseline-box+t+1, box-2*t, box-2*t, 2, ink)
		}
		xt := x + box + body/2
		dueW := 0
		if r.Due != "" {
			dueInk := render.InkBlack
			if r.Urgent {
				dueInk = render.InkColour
			}
			c.TextRight(x+colW, baseline, r.Due, small, dueInk)
			dueW = c.Advance(r.Due, small) + body/2
		}
		c.Text(xt, baseline, c.Fit(r.Name, name, x+colW-dueW-xt), name, render.InkBlack)
	}
	if rest := len(rows) - len(shown); rest > 0 {
		i := len(shown)
		x := (i / perCol) * (colW + colGap)
		baseline := (i%perCol)*rowH + (rowH+capH)/2
		more := i18n.T(lang, "more", rest)
		c.Text(x+box+body/2, baseline, more, small, render.InkBlack)
	}
}
