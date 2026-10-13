package ical

import (
	"strings"
	"testing"
	"time"
)

// Expected dates come from python-dateutil, which upstream Inkycal uses.
const recurrenceICS = `BEGIN:VCALENDAR
VERSION:2.0
PRODID:test
BEGIN:VEVENT
UID:second-monday
DTSTAMP:20261001T000000Z
DTSTART;TZID=Europe/Berlin:20261012T190000
DTEND;TZID=Europe/Berlin:20261012T200000
RRULE:FREQ=MONTHLY;BYDAY=2MO
SUMMARY:Elternabend
END:VEVENT
BEGIN:VEVENT
UID:last-friday
DTSTAMP:20261001T000000Z
DTSTART;TZID=Europe/Berlin:20261030T190000
DTEND;TZID=Europe/Berlin:20261030T210000
RRULE:FREQ=MONTHLY;BYDAY=-1FR
SUMMARY:Stammtisch
END:VEVENT
BEGIN:VEVENT
UID:fifteenth
DTSTAMP:20261001T000000Z
DTSTART;VALUE=DATE:20261015
RRULE:FREQ=MONTHLY;BYMONTHDAY=15
SUMMARY:Miete
END:VEVENT
BEGIN:VEVENT
UID:last-workday
DTSTAMP:20261001T000000Z
DTSTART;TZID=Europe/Berlin:20261030T080000
DTEND;TZID=Europe/Berlin:20261030T083000
RRULE:FREQ=MONTHLY;BYDAY=MO,TU,WE,TH,FR;BYSETPOS=-1
SUMMARY:Abrechnung
END:VEVENT
BEGIN:VEVENT
UID:count-exdate
DTSTAMP:20261001T000000Z
DTSTART;TZID=Europe/Berlin:20261012T070000
DTEND;TZID=Europe/Berlin:20261012T080000
RRULE:FREQ=WEEKLY;BYDAY=MO;COUNT=3
EXDATE;TZID=Europe/Berlin:20261019T070000
SUMMARY:Kurs
END:VEVENT
BEGIN:VEVENT
UID:rdate
DTSTAMP:20261001T000000Z
DTSTART;TZID=Europe/Berlin:20261014T100000
DTEND;TZID=Europe/Berlin:20261014T110000
RDATE;TZID=Europe/Berlin:20261111T100000,20261209T100000
SUMMARY:Sprechstunde
END:VEVENT
BEGIN:VEVENT
UID:weekly-dst
DTSTAMP:20261001T000000Z
DTSTART;TZID=Europe/Berlin:20261020T180000
DTEND;TZID=Europe/Berlin:20261020T190000
RRULE:FREQ=WEEKLY;UNTIL=20261103T170000Z
SUMMARY:Training
END:VEVENT
BEGIN:VEVENT
UID:weekly-dst
DTSTAMP:20261001T000000Z
RECURRENCE-ID;TZID=Europe/Berlin:20261027T180000
DTSTART;TZID=Europe/Berlin:20261028T180000
DTEND;TZID=Europe/Berlin:20261028T190000
SUMMARY:Training (verschoben)
END:VEVENT
BEGIN:VEVENT
DTSTART;TZID=Europe/Berlin:20261016T120000
SUMMARY:Ohne UID, DTSTAMP und Ende
END:VEVENT
BEGIN:VEVENT
UID:duration
DTSTART;TZID=Europe/Berlin:20261016T140000
DURATION:PT1H30M
SUMMARY:Mit Dauer
BEGIN:VALARM
TRIGGER:-PT15M
DESCRIPTION:not a summary
END:VALARM
END:VEVENT
END:VCALENDAR
`

func TestRecurrenceMatchesDateutil(t *testing.T) {
	loc := berlin(t)
	from := time.Date(2026, 10, 1, 0, 0, 0, 0, loc)
	to := time.Date(2027, 2, 1, 0, 0, 0, 0, loc)
	ev, err := Parse(strings.NewReader(recurrenceICS), from, to, loc)
	if err != nil {
		t.Fatal(err)
	}
	got := map[string][]string{}
	for _, e := range ev {
		got[e.Title] = append(got[e.Title], e.Start.Format("01-02 15:04"))
	}
	want := map[string]string{
		"Elternabend":                "10-12 19:00 11-09 19:00 12-14 19:00 01-11 19:00",
		"Stammtisch":                 "10-30 19:00 11-27 19:00 12-25 19:00 01-29 19:00",
		"Miete":                      "10-15 00:00 11-15 00:00 12-15 00:00 01-15 00:00",
		"Abrechnung":                 "10-30 08:00 11-30 08:00 12-31 08:00 01-29 08:00",
		"Kurs":                       "10-12 07:00 10-26 07:00",
		"Sprechstunde":               "10-14 10:00 11-11 10:00 12-09 10:00",
		"Training":                   "10-20 18:00 11-03 18:00", // 10-27 moved; wall clock kept across DST (25 Oct)
		"Training (verschoben)":      "10-28 18:00",
		"Ohne UID, DTSTAMP und Ende": "10-16 12:00",
		"Mit Dauer":                  "10-16 14:00",
	}
	for title, w := range want {
		if g := strings.Join(got[title], " "); g != w {
			t.Errorf("%s:\n got %s\nwant %s", title, g, w)
		}
	}
	if len(got) != len(want) {
		t.Errorf("unexpected titles: %v", got)
	}
	for _, e := range ev {
		switch e.Title {
		case "Mit Dauer":
			if e.End.Sub(e.Start) != 90*time.Minute {
				t.Errorf("DURATION end: %s", e.End)
			}
		case "Miete":
			if !e.AllDay || e.End.Sub(e.Start) != 24*time.Hour {
				t.Errorf("all-day without DTEND: allDay=%v %s–%s", e.AllDay, e.Start, e.End)
			}
		case "Ohne UID, DTSTAMP und Ende":
			if !e.End.Equal(e.Start) {
				t.Errorf("timed event without end must be zero-length: %s", e.End)
			}
		}
	}
}

func TestParseLineQuotesAndFolding(t *testing.T) {
	loc := berlin(t)
	ics := "BEGIN:VCALENDAR\r\nBEGIN:VEVENT\r\nUID:x\r\nDTSTART;TZID=\"Europe/Berlin\":20261016T090000\r\nSUMMARY:Lange Be\r\n sprechung\\, mit Komma\r\nEND:VEVENT\r\nEND:VCALENDAR\r\n"
	from := time.Date(2026, 10, 16, 0, 0, 0, 0, loc)
	ev, err := Parse(strings.NewReader(ics), from, from.AddDate(0, 0, 1), loc)
	if err != nil || len(ev) != 1 {
		t.Fatalf("%v %+v", err, ev)
	}
	if ev[0].Title != "Lange Besprechung, mit Komma" || ev[0].Start.Hour() != 9 {
		t.Errorf("%q at %s", ev[0].Title, ev[0].Start)
	}
}

func TestParseDuration(t *testing.T) {
	for in, want := range map[string]time.Duration{
		"PT1H30M": 90 * time.Minute, "P1D": 24 * time.Hour, "P1W": 7 * 24 * time.Hour,
		"-PT15M": -15 * time.Minute, "P1DT2H": 26 * time.Hour,
	} {
		if got, err := parseDuration(in); err != nil || got != want {
			t.Errorf("%s → %v %v", in, got, err)
		}
	}
	if _, err := parseDuration("1H"); err == nil {
		t.Error("garbage accepted")
	}
}
