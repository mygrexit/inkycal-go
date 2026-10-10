package calendar

import (
	"sort"
	"time"

	"inkycal/internal/ical"
	"inkycal/internal/timefmt"
)

type eventRow struct {
	Date   string
	Time   string
	Title  string
	AllDay bool
}

func dateOnly(t time.Time) time.Time {
	y, m, d := t.Date()
	return time.Date(y, m, d, 0, 0, 0, 0, t.Location())
}

// eventDays returns the sorted day numbers of (year, month) touched by an event.
// End is exclusive (an all-day event ending at the next midnight marks one day)
// and only days of the shown month count — Python marked both wrongly.
func eventDays(events []ical.Event, year int, month time.Month, loc *time.Location) []int {
	seen := map[int]bool{}
	for _, e := range events {
		start := e.Start.In(loc)
		last := start
		if e.End.After(e.Start) {
			last = e.End.In(loc).Add(-time.Nanosecond)
		}
		for d := dateOnly(start); !d.After(dateOnly(last)); d = d.AddDate(0, 0, 1) {
			if d.Year() == year && d.Month() == month {
				seen[d.Day()] = true
			}
		}
	}
	days := make([]int, 0, len(seen))
	for d := range seen {
		days = append(days, d)
	}
	sort.Ints(days)
	return days
}

// buildEventRows lists events that have not ended and start within four weeks,
// sorted by start, at most limit rows.
func buildEventRows(events []ical.Event, now time.Time, limit int, dateFmt, timeFmt, lang string) []eventRow {
	horizon := now.AddDate(0, 0, 28)
	var upcoming []ical.Event
	for _, e := range events {
		if e.End.After(now) && e.Start.Before(horizon) {
			upcoming = append(upcoming, e)
		}
	}
	sort.SliceStable(upcoming, func(i, j int) bool { return upcoming[i].Start.Before(upcoming[j].Start) })
	limit = max(0, limit)
	if len(upcoming) > limit {
		upcoming = upcoming[:limit]
	}
	rows := make([]eventRow, 0, len(upcoming))
	for _, e := range upcoming {
		start := e.Start.In(now.Location())
		title := e.Title
		if days := int(e.End.Sub(e.Start).Hours() / 24); days > 1 {
			title += " (" + timefmt.HumanizeDays(days, lang) + ")"
		}
		rows = append(rows, eventRow{
			Date:   timefmt.Format(start, dateFmt, lang),
			Time:   timefmt.Format(start, timeFmt, lang),
			Title:  title,
			AllDay: e.AllDay,
		})
	}
	return rows
}
