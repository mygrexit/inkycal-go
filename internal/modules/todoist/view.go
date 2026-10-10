package todoist

import (
	"fmt"
	"log/slog"
	"slices"
	"sort"
	"strings"
	"time"

	"inkycal/internal/render"
	"inkycal/internal/timefmt"
)

const (
	bulletFilled = "● "
	bulletEmpty  = "○ "
)

// row is one rendered todo line.
type row struct {
	Project        string
	Due            string
	Overdue        bool
	Priority       string
	PriorityColour bool
	Name           string
}

// parseDue reads the date part of a Todoist due value ("2026-10-10", "2026-10-10T15:00:00Z", ...).
func parseDue(s string, loc *time.Location) (time.Time, error) {
	if len(s) < 10 {
		return time.Time{}, fmt.Errorf("due date %q too short", s)
	}
	return time.ParseInLocation("2006-01-02", s[:10], loc)
}

// buildRows groups tasks by project (API order), sorts each group by due date
// (tasks with a date first) and priority, and formats every line.
func buildRows(projects []project, tasks []task, filter []string, showPriority bool, now time.Time) ([]row, error) {
	names := make(map[string]string, len(projects))
	var order []string
	for _, p := range projects {
		if len(filter) > 0 && !slices.Contains(filter, p.Name) {
			continue
		}
		names[p.ID] = p.Name
		order = append(order, p.ID)
	}
	if len(filter) > 0 && len(order) == 0 {
		return nil, fmt.Errorf("todoist: no project matches project_filter %q", strings.Join(filter, ","))
	}

	y, m, d := now.Date()
	today := time.Date(y, m, d, 0, 0, 0, 0, now.Location())

	type item struct {
		row
		due      time.Time
		hasDue   bool
		priority int
	}
	groups := map[string][]item{}
	for _, t := range tasks {
		name, ok := names[t.ProjectID]
		if !ok {
			continue
		}
		it := item{row: row{Project: name, Name: t.Content}, priority: t.Priority}
		if t.Due != nil && t.Due.Date != "" {
			dueDate, err := parseDue(t.Due.Date, now.Location())
			if err != nil {
				slog.Warn("todoist: skipping task with unparsable due date", "task", t.Content, "due", t.Due.Date)
				continue
			}
			it.due, it.hasDue = dueDate, true
			it.Overdue = dueDate.Before(today)
			if dueDate.Equal(today) {
				it.Due = "TODAY"
			} else {
				it.Due = timefmt.Format(dueDate, "D-MMM-YY", "en")
			}
		}
		if showPriority {
			switch t.Priority {
			case 4:
				it.Priority, it.PriorityColour = bulletFilled, true
			case 3:
				it.Priority = bulletFilled
			case 2:
				it.Priority = bulletEmpty
			}
		}
		groups[t.ProjectID] = append(groups[t.ProjectID], it)
	}

	var rows []row
	for _, id := range order {
		items := groups[id]
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
		for _, it := range items {
			rows = append(rows, it.row)
		}
	}
	return rows, nil
}

// columnWidth is the widest text plus 10 %, like Python's offsets.
func columnWidth(c *render.Canvas, rows []row, pick func(row) string) int {
	w := 0
	for _, r := range rows {
		if s := pick(r); s != "" {
			w = max(w, int(float64(c.TextWidth(s))*1.1))
		}
	}
	return w
}

func drawRows(c *render.Canvas, rows []row) {
	lh := c.LineHeight() + 1
	maxLines := c.H / lh
	top := (c.H % lh) / 2
	projW := columnWidth(c, rows, func(r row) string { return r.Project })
	dueW := columnWidth(c, rows, func(r row) string { return r.Due })
	prioW := columnWidth(c, rows, func(r row) string { return r.Priority })

	for i, r := range rows {
		if i >= maxLines {
			slog.Warn("todoist: more todos than available lines", "shown", maxLines, "total", len(rows))
			break
		}
		y := top + i*lh
		if r.Project != "" {
			c.Write(0, y, projW, lh, r.Project, render.TextOpts{Align: render.AlignLeft, Ink: render.InkColour})
		}
		if r.Due != "" {
			ink := render.InkBlack
			if r.Overdue {
				ink = render.InkColour
			}
			c.Write(projW, y, dueW, lh, r.Due, render.TextOpts{Align: render.AlignLeft, Ink: ink})
		}
		if r.Priority != "" {
			ink := render.InkBlack
			if r.PriorityColour {
				ink = render.InkColour
			}
			c.Write(projW+dueW, y, prioW, lh, r.Priority, render.TextOpts{Align: render.AlignLeft, Ink: ink})
		}
		if r.Name != "" {
			x := projW + dueW + prioW
			c.Write(x, y, c.W-x, lh, r.Name, render.TextOpts{Align: render.AlignLeft})
		}
	}
}
