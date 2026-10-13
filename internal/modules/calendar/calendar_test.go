package calendar

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"inkycal/internal/config"
	"inkycal/internal/fetch"
	"inkycal/internal/modules"
	"inkycal/internal/render"
)

var testICS = strings.Join([]string{
	"BEGIN:VCALENDAR",
	"VERSION:2.0",
	"PRODID:-//inkycal//test//EN",
	"BEGIN:VEVENT",
	"UID:1@test",
	"DTSTAMP:20261001T000000Z",
	"DTSTART:20261010T080000Z",
	"DTEND:20261010T090000Z",
	"SUMMARY:Zahnarzt",
	"END:VEVENT",
	"BEGIN:VEVENT",
	"UID:2@test",
	"DTSTAMP:20261001T000000Z",
	"DTSTART;VALUE=DATE:20261012",
	"DTEND;VALUE=DATE:20261013",
	"SUMMARY:Urlaub",
	"END:VEVENT",
	"END:VCALENDAR",
	"",
}, "\r\n")

func mustCommon(t *testing.T, raw string) config.Common {
	t.Helper()
	c, err := config.ParseCommon(json.RawMessage(raw))
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func inkIn(c *render.Canvas, x0, y0, x1, y1 int) (black, colour bool) {
	for y := y0; y < y1; y++ {
		for x := x0; x < x1; x++ {
			b, col := c.InkAt(x, y)
			black = black || b
			colour = colour || col
		}
	}
	return black, colour
}

func serveICS(t *testing.T) (url string, hits *atomic.Int32) {
	t.Helper()
	hits = &atomic.Int32{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		io.WriteString(w, testICS)
	}))
	t.Cleanup(srv.Close)
	return srv.URL + "/cal.ics", hits
}

// cellRect returns the grid cell of a day as used by Generate.
func cellRect(t *testing.T, w, h, fontSize int, now time.Time, showEvents bool, day int) (x0, y0, x1, y1 int) {
	t.Helper()
	weeks := monthGrid(now.Year(), now.Month(), time.Monday)
	lh := render.NewCanvas(w, h, render.FontDefault, fontSize).LineHeight()
	l := computeLayout(w, h, lh, showEvents, len(weeks))
	for wi, week := range weeks {
		for col, d := range week {
			if d == day {
				x, y := l.cell(wi, col)
				return x, y, x + l.CellW, y + l.CellH
			}
		}
	}
	t.Fatalf("day %d not in grid", day)
	return
}

// cellInk reports ink inside the grid cell of a day (canvas size taken from c, font size 14).
func cellInk(t *testing.T, c *render.Canvas, now time.Time, showEvents bool, day int) (black, colour bool) {
	t.Helper()
	x0, y0, x1, y1 := cellRect(t, c.W, c.H, 14, now, showEvents, day)
	return inkIn(c, x0, y0, x1, y1)
}

func TestNewRejectsTinySize(t *testing.T) {
	tiny := mustCommon(t, `{"size":[60,60],"padding_x":0,"padding_y":0,"fontsize":14}`)
	if _, err := New(tiny, json.RawMessage(`{}`)); err == nil {
		t.Error("60x60 accepted")
	}
}

func TestGenerateWithEvents(t *testing.T) {
	loc := berlin(t)
	now := time.Date(2026, 10, 10, 9, 0, 0, 0, loc)
	icsURL, hits := serveICS(t)
	common := mustCommon(t, `{"size":[480,400],"padding_x":10,"padding_y":10,"fontsize":14,"language":"en"}`)
	m, err := New(common, json.RawMessage(fmt.Sprintf(
		`{"week_starts_on":"Monday","show_events":true,"ical_urls":%q,"ical_files":null,"date_format":"D MMM","time_format":"HH:mm"}`, icsURL)))
	if err != nil {
		t.Fatal(err)
	}
	env := &modules.Env{Now: now, Loc: loc, Fetch: fetch.New("")}

	c, err := m.Generate(context.Background(), env)
	if err != nil {
		t.Fatal(err)
	}
	if c.W != 460 || c.H != 380 {
		t.Fatalf("canvas = %dx%d", c.W, c.H)
	}
	if hits.Load() == 0 {
		t.Error("iCal URL was never fetched")
	}
	if _, colour := cellInk(t, c, now, true, 10); !colour {
		t.Error("today (10th) has no red circle")
	}
	if _, colour := cellInk(t, c, now, true, 12); !colour {
		t.Error("event day (12th) has no red border")
	}
	if _, colour := cellInk(t, c, now, true, 20); colour {
		t.Error("day without events (20th) has red ink")
	}
	weeks := monthGrid(2026, time.October, time.Monday)
	l := computeLayout(460, 380, render.NewCanvas(460, 380, render.FontDefault, 14).LineHeight(), true, len(weeks))
	// +2 skips the bottom edge of the last grid row.
	if black, _ := inkIn(c, 0, c.H-l.EventsH+2, c.W, c.H); !black {
		t.Error("events list area is empty")
	}
}

func TestGenerateWithoutEventsSkipsFetch(t *testing.T) {
	loc := berlin(t)
	now := time.Date(2026, 10, 10, 9, 0, 0, 0, loc)
	icsURL, hits := serveICS(t)
	common := mustCommon(t, `{"size":[480,400],"padding_x":10,"padding_y":10,"fontsize":14,"language":"de"}`)
	m, err := New(common, json.RawMessage(fmt.Sprintf(`{"show_events":"False","ical_urls":%q}`, icsURL)))
	if err != nil {
		t.Fatal(err)
	}
	c, err := m.Generate(context.Background(), &modules.Env{Now: now, Loc: loc, Fetch: fetch.New("")})
	if err != nil {
		t.Fatal(err)
	}
	if hits.Load() != 0 {
		t.Error("show_events=false still fetched the calendar")
	}
	if _, colour := cellInk(t, c, now, false, 10); !colour {
		t.Error("today circle missing")
	}
}
