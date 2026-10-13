package calendar

import (
	"fmt"
	"slices"
	"strings"
	"testing"
	"time"

	"inkycal/internal/ical"
)

func TestEventDays(t *testing.T) {
	loc := berlin(t)
	d := func(month time.Month, day, hour int) time.Time {
		return time.Date(2026, month, day, hour, 0, 0, 0, loc)
	}
	events := []ical.Event{
		{Title: "all-day 5th", Start: d(10, 5, 0), End: d(10, 6, 0), AllDay: true},
		{Title: "timed 7th", Start: d(10, 7, 10), End: d(10, 7, 11)},
		{Title: "from september", Start: d(9, 29, 0), End: d(10, 3, 0), AllDay: true},
		{Title: "november", Start: d(11, 3, 10), End: d(11, 3, 11)},
		{Title: "over midnight", Start: d(10, 20, 22), End: d(10, 21, 2)},
	}
	got := eventDays(events, 2026, time.October, loc)
	want := []int{1, 2, 5, 7, 20, 21}
	if !slices.Equal(got, want) {
		t.Errorf("eventDays = %v, want %v", got, want)
	}
}

func TestBuildEventRows(t *testing.T) {
	loc := berlin(t)
	now := time.Date(2026, 10, 10, 9, 0, 0, 0, loc)
	at := func(month time.Month, day, hour int) time.Time {
		return time.Date(2026, month, day, hour, 0, 0, 0, loc)
	}
	events := []ical.Event{
		{Title: "Zahnarzt", Start: at(10, 10, 10), End: at(10, 10, 11)},
		{Title: "Vergangen", Start: at(10, 9, 10), End: at(10, 9, 11)},
		{Title: "Urlaub", Start: at(10, 12, 0), End: at(10, 15, 0), AllDay: true},
		{Title: "Läuft", Start: at(10, 9, 8), End: at(10, 10, 12)},
		{Title: "Fern", Start: at(11, 20, 10), End: at(11, 20, 11)},
	}

	rows := buildEventRows(mergeSources(events, nil, false, "en"), now, 10, "D MMM", "HH:mm", "en")
	if len(rows) != 3 {
		t.Fatalf("rows = %+v, want 3 rows", rows)
	}
	// Läuft spans two calendar days (counted inclusively for timed events).
	if rows[0].Title != "Läuft (2 days)" || rows[1].Title != "Zahnarzt" {
		t.Errorf("order = %q, %q", rows[0].Title, rows[1].Title)
	}
	if rows[1].Date != "10 Oct" || rows[1].Time != "10:00–11:00" || rows[1].AllDay {
		t.Errorf("Zahnarzt row = %+v", rows[1])
	}
	if rows[2].Title != "Urlaub (3 days)" || !rows[2].AllDay {
		t.Errorf("Urlaub row = %+v", rows[2])
	}
	if !rows[0].Today || !rows[1].Today || rows[2].Today {
		t.Errorf("today flags: %v %v %v", rows[0].Today, rows[1].Today, rows[2].Today)
	}

	if limited := buildEventRows(mergeSources(events, nil, false, "en"), now, 2, "D MMM", "HH:mm", "en"); len(limited) != 2 {
		t.Errorf("limit 2 gave %d rows", len(limited))
	}
	if none := buildEventRows(mergeSources(events, nil, false, "en"), now, -1, "D MMM", "HH:mm", "en"); len(none) != 0 {
		t.Errorf("negative limit gave %d rows", len(none))
	}
}

func TestMergeSourcesLabelsAndDedupes(t *testing.T) {
	loc := time.UTC
	at := func(h int) time.Time { return time.Date(2026, 10, 13, h, 0, 0, 0, loc) }
	events := []ical.Event{
		{Title: "Kinderarzt", Start: at(11), End: at(12), Source: 0},
		{Title: "Kinderarzt ", Start: at(11), End: at(12), Source: 1}, // same event, other calendar
		{Title: "Barber", Start: at(10), End: at(11), Source: 0},
		{Title: "Babyschwimmen", Start: at(16), End: at(17), Source: 1},
		{Title: "Kinderarzt", Start: at(15), End: at(16), Source: 1}, // same title, other time
	}
	got := mergeSources(events, []string{"Niko", "Sofia"}, true, "de")
	want := map[string]string{"Kinderarzt@11": "beide", "Barber@10": "Niko", "Babyschwimmen@16": "Sofia", "Kinderarzt@15": "Sofia"}
	if len(got) != len(want) {
		t.Fatalf("got %d events: %+v", len(got), got)
	}
	for _, e := range got {
		k := fmt.Sprintf("%s@%d", strings.TrimSpace(e.Title), e.Start.Hour())
		if want[k] != e.Label {
			t.Errorf("%s: label %q, want %q", k, e.Label, want[k])
		}
	}
}

func TestMergeSourcesWithoutDedupeOrLabels(t *testing.T) {
	at := time.Date(2026, 10, 13, 11, 0, 0, 0, time.UTC)
	events := []ical.Event{{Title: "A", Start: at, End: at, Source: 0}, {Title: "A", Start: at, End: at, Source: 1}}
	if got := mergeSources(events, []string{"Niko", "Sofia"}, false, "de"); len(got) != 2 || got[0].Label != "Niko" || got[1].Label != "Sofia" {
		t.Errorf("no dedupe: %+v", got)
	}
	if got := mergeSources(events, nil, true, "de"); len(got) != 1 || got[0].Label != "" {
		t.Errorf("dedupe without labels: %+v", got)
	}
	if got := mergeSources(events, []string{"Niko", "Sofia"}, true, "en"); got[0].Label != "both" {
		t.Errorf("english: %+v", got)
	}
}

func TestBuildEventRowsCarriesLabel(t *testing.T) {
	now := time.Date(2026, 10, 13, 8, 0, 0, 0, time.UTC)
	ev := []labeledEvent{{Event: ical.Event{Title: "Barber", Start: now.Add(time.Hour), End: now.Add(2 * time.Hour)}, Label: "Niko"}}
	rows := buildEventRows(ev, now, 5, "D MMM", "HH:mm", "de")
	if len(rows) != 1 || rows[0].Label != "Niko" || rows[0].Title != "Barber" {
		t.Fatalf("rows: %+v", rows)
	}
}

func TestBuildEventRowsGermanDayCountAndWeekdayPrefix(t *testing.T) {
	loc := berlin(t)
	now := time.Date(2026, 10, 13, 8, 0, 0, 0, loc)
	ev := []labeledEvent{{Event: ical.Event{Title: "Urlaub", Start: time.Date(2026, 10, 24, 0, 0, 0, 0, loc), End: time.Date(2026, 10, 27, 0, 0, 0, 0, loc), AllDay: true}}}
	rows := buildEventRows(ev, now, 5, dateFormat("DD. MMM", true), "HH:mm", "de")
	if rows[0].Title != "Urlaub (3 Tage)" || rows[0].Date != "Sa, 24. Okt" {
		t.Fatalf("row: %+v", rows[0])
	}
	if dateFormat("D MMM", false) != "D MMM" {
		t.Error("prefix added although disabled")
	}
}

func TestReviewFixes(t *testing.T) {
	loc := berlin(t)
	now := time.Date(2026, 10, 13, 6, 0, 0, 0, loc)
	at := func(day, hour int) time.Time { return time.Date(2026, 10, day, hour, 0, 0, 0, loc) }
	evs := []labeledEvent{
		{Event: ical.Event{Title: "Party", Start: at(14, 20), End: at(15, 0)}},     // ends at midnight
		{Event: ical.Event{Title: "Messe", Start: at(11, 9), End: at(15, 18)}},     // started before today
		{Event: ical.Event{Title: "Erinnerung", Start: at(13, 9), End: at(13, 9)}}, // no DTEND
	}
	rows := buildEventRows(evs, now, 10, "D MMM", "HH:mm", "de")
	byTitle := map[string]eventRow{}
	for _, r := range rows {
		byTitle[strings.Fields(r.Title)[0]] = r
	}
	if r := byTitle["Party"]; r.Title != "Party" {
		t.Errorf("event ending at midnight: %q, want no day count", r.Title)
	}
	if r := byTitle["Messe"]; !r.Day.Equal(dateOnly(now)) || !r.Today {
		t.Errorf("running event grouped under %v (today=%v), want today", r.Day, r.Today)
	}
	if r := byTitle["Erinnerung"]; r.Time != "09:00" {
		t.Errorf("event without end shows %q", r.Time)
	}
}
