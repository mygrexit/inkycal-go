package calendar

import (
	"slices"
	"testing"
	"time"

	"inkycal/internal/ical"
	"inkycal/internal/timefmt"
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

	rows := buildEventRows(events, now, 10, "D MMM", "HH:mm", "en")
	if len(rows) != 3 {
		t.Fatalf("rows = %+v, want 3 rows", rows)
	}
	if rows[0].Title != "Läuft" || rows[1].Title != "Zahnarzt" {
		t.Errorf("order = %q, %q", rows[0].Title, rows[1].Title)
	}
	if rows[1].Date != "10 Oct" || rows[1].Time != "10:00" || rows[1].AllDay {
		t.Errorf("Zahnarzt row = %+v", rows[1])
	}
	wantTitle := "Urlaub (" + timefmt.HumanizeDays(3, "en") + ")"
	if rows[2].Title != wantTitle || !rows[2].AllDay {
		t.Errorf("Urlaub row = %+v, want title %q", rows[2], wantTitle)
	}

	if limited := buildEventRows(events, now, 2, "D MMM", "HH:mm", "en"); len(limited) != 2 {
		t.Errorf("limit 2 gave %d rows", len(limited))
	}
	if none := buildEventRows(events, now, -1, "D MMM", "HH:mm", "en"); len(none) != 0 {
		t.Errorf("negative limit gave %d rows", len(none))
	}
}
