package ical

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"inkycal/internal/fetch"
)

const testICS = `BEGIN:VCALENDAR
VERSION:2.0
PRODID:test
BEGIN:VEVENT
UID:timed-1
DTSTAMP:20261001T000000Z
DTSTART;TZID=Europe/Berlin:20261012T093000
DTEND;TZID=Europe/Berlin:20261012T100000
SUMMARY: Zahnarzt
END:VEVENT
BEGIN:VEVENT
UID:allday-today
DTSTAMP:20261001T000000Z
DTSTART;VALUE=DATE:20261010
DTEND;VALUE=DATE:20261011
SUMMARY:Geburtstag Oma
END:VEVENT
BEGIN:VEVENT
UID:weekly
DTSTAMP:20261001T000000Z
DTSTART;TZID=Europe/Berlin:20261005T180000
DTEND;TZID=Europe/Berlin:20261005T190000
RRULE:FREQ=WEEKLY;COUNT=5
EXDATE;TZID=Europe/Berlin:20261019T180000
SUMMARY:Training
END:VEVENT
BEGIN:VEVENT
UID:outlook
DTSTAMP:20261001T000000Z
DTSTART;TZID=W. Europe Standard Time:20261013T080000
DTEND;TZID=W. Europe Standard Time:20261013T083000
SUMMARY:Standup
END:VEVENT
BEGIN:VEVENT
UID:past
DTSTAMP:20261001T000000Z
DTSTART:20260101T100000Z
DTEND:20260101T110000Z
SUMMARY:Old
END:VEVENT
BEGIN:VEVENT
UID:broken
SUMMARY:No DTSTART and no DTSTAMP
END:VEVENT
END:VCALENDAR
`

func berlin(t *testing.T) *time.Location {
	loc, err := time.LoadLocation("Europe/Berlin")
	if err != nil {
		t.Skip("no tzdata")
	}
	return loc
}

func TestParse(t *testing.T) {
	loc := berlin(t)
	from := time.Date(2026, 10, 10, 0, 0, 0, 0, loc)
	to := from.AddDate(0, 0, 14)
	ev, err := Parse(strings.NewReader(testICS), from, to, loc)
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, e := range ev {
		got = append(got, e.Start.Format("01-02 15:04")+" "+e.Title)
	}
	want := []string{
		"10-10 00:00 Geburtstag Oma",
		"10-12 09:30 Zahnarzt",
		"10-12 18:00 Training",
		"10-13 08:00 Standup",
		// 10-19 Training is excluded by EXDATE
	}
	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Errorf("events:\n got %q\nwant %q", got, want)
	}
	if !ev[0].AllDay || !ev[0].End.Equal(from.AddDate(0, 0, 1)) {
		t.Errorf("all-day event: allDay=%v end=%s", ev[0].AllDay, ev[0].End)
	}
	if ev[1].AllDay {
		t.Error("timed event marked all-day")
	}
}

func TestParseTimedEventStartingExactlyAtFrom(t *testing.T) {
	loc := berlin(t)
	from := time.Date(2026, 10, 12, 9, 30, 0, 0, loc)
	ev, err := Parse(strings.NewReader(testICS), from, from.Add(time.Hour), loc)
	if err != nil {
		t.Fatal(err)
	}
	if len(ev) != 1 || ev[0].Title != "Zahnarzt" {
		t.Fatalf("got %+v", ev)
	}
}

func TestLoadURLsAndFiles(t *testing.T) {
	loc := berlin(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(testICS))
	}))
	defer srv.Close()
	file := filepath.Join(t.TempDir(), "local.ics")
	os.WriteFile(file, []byte(strings.ReplaceAll(testICS, "Zahnarzt", "Lokal")), 0o644)
	fc := fetch.New(t.TempDir())
	from := time.Date(2026, 10, 12, 0, 0, 0, 0, loc)
	ev, stale, err := Load(context.Background(), fc, []string{srv.URL}, []string{file}, from, from.AddDate(0, 0, 1), loc)
	if err != nil || stale {
		t.Fatalf("err=%v stale=%v", err, stale)
	}
	titles := map[string]bool{}
	for _, e := range ev {
		titles[e.Title] = true
	}
	if !titles["Zahnarzt"] || !titles["Lokal"] || !titles["Training"] {
		t.Errorf("titles: %v", titles)
	}
}

func TestLoadFailsWithoutCache(t *testing.T) {
	srv := httptest.NewServer(http.NotFoundHandler())
	srv.Close()
	fc := fetch.New("")
	fc.Attempts = 1
	now := time.Now()
	if _, _, err := Load(context.Background(), fc, []string{srv.URL}, nil, now, now.Add(time.Hour), time.UTC); err == nil {
		t.Fatal("expected error")
	}
}
