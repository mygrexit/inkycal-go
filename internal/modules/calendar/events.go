package calendar

import (
	"math"
	"sort"
	"strings"
	"time"

	"inkycal/internal/i18n"
	"inkycal/internal/ical"
	"inkycal/internal/timefmt"
)

type eventRow struct {
	Day    time.Time // start date (local midnight)
	Date   string
	Time   string
	Title  string
	Label  string // calendar name(s), drawn in colour after the title
	AllDay bool
	Today  bool // runs on today's date: framed together with the other ones
}

// labeledEvent is an event with the name of the calendar(s) it came from.
type labeledEvent struct {
	ical.Event
	Label string
}

// mergeSources names each event's calendar (labels[i] for calendar i). With dedupe,
// identical events from several calendars (same title, start and end) appear once,
// labelled "beide"/"both" for two calendars or with all names joined by "+".
func mergeSources(events []ical.Event, labels []string, dedupe bool, lang string) []labeledEvent {
	name := func(src int) string {
		if src >= 0 && src < len(labels) {
			return labels[src]
		}
		return ""
	}
	type key struct {
		title      string
		start, end int64
		allDay     bool
	}
	out := make([]labeledEvent, 0, len(events))
	sources := map[key][]int{}
	index := map[key]int{}
	for _, e := range events {
		k := key{strings.ToLower(strings.TrimSpace(e.Title)), e.Start.UnixNano(), e.End.UnixNano(), e.AllDay}
		if i, ok := index[k]; ok && dedupe {
			if len(labels) > 0 && !containsInt(sources[k], e.Source) {
				sources[k] = append(sources[k], e.Source)
				out[i].Label = joinSources(sources[k], labels, lang)
			}
			continue
		}
		index[k] = len(out)
		sources[k] = []int{e.Source}
		out = append(out, labeledEvent{Event: e, Label: name(e.Source)})
	}
	return out
}

func containsInt(s []int, v int) bool {
	for _, x := range s {
		if x == v {
			return true
		}
	}
	return false
}

func joinSources(srcs []int, labels []string, lang string) string {
	if len(srcs) == 2 && len(labels) == 2 {
		return i18n.T(lang, "both")
	}
	sorted := append([]int(nil), srcs...)
	sort.Ints(sorted)
	var names []string
	for _, s := range sorted {
		if s < len(labels) {
			names = append(names, labels[s])
		}
	}
	return strings.Join(names, "+")
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

// dateFormat prepends the weekday ("Sa, 24. Okt") when weekday_prefix is on.
func dateFormat(base string, weekdayPrefix bool) string {
	if weekdayPrefix {
		return "ddd, " + base
	}
	return base
}

// spanDays counts the calendar days an event covers: all-day ends are exclusive,
// timed events count both the first and the last day.
func spanDays(e ical.Event, loc *time.Location) int {
	end := e.End
	if !e.AllDay && e.End.After(e.Start) {
		end = end.Add(-time.Nanosecond) // ending at midnight does not touch the next day
	}
	first, last := dateOnly(e.Start.In(loc)), dateOnly(end.In(loc))
	days := int(math.Round(last.Sub(first).Hours() / 24))
	if !e.AllDay {
		days++
	}
	return days
}

// buildEventRows lists events that have not ended and start within four weeks,
// sorted by start, at most limit rows. Today marks events running on today's date.
func buildEventRows(events []labeledEvent, now time.Time, limit int, dateFmt, timeFmt, lang string) []eventRow {
	loc := now.Location()
	horizon := now.AddDate(0, 0, 28)
	todayStart := dateOnly(now)
	tomorrow := todayStart.AddDate(0, 0, 1)
	var upcoming []labeledEvent
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
		start := e.Start.In(loc)
		title := e.Title
		if days := spanDays(e.Event, loc); days > 1 {
			title += " " + i18n.T(lang, "span_days", days)
		}
		// An event that started on an earlier day and is still running belongs to today.
		shown := start
		if start.Before(todayStart) {
			shown = todayStart
		}
		clock := timefmt.Format(start, timeFmt, lang)
		if e.End.After(e.Start) {
			clock += "–" + timefmt.Format(e.End.In(loc), timeFmt, lang)
		}
		rows = append(rows, eventRow{
			Day:    dateOnly(shown),
			Date:   timefmt.Format(shown, dateFmt, lang),
			Time:   clock,
			Title:  title,
			Label:  e.Label,
			AllDay: e.AllDay,
			Today:  e.Start.Before(tomorrow) && e.End.After(todayStart),
		})
	}
	return rows
}
