package agenda

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"inkycal/internal/config"
	"inkycal/internal/fetch"
	"inkycal/internal/ical"
	"inkycal/internal/modules"
	"inkycal/internal/render"
)

const ics = `BEGIN:VCALENDAR
VERSION:2.0
PRODID:test
BEGIN:VEVENT
UID:1
DTSTAMP:20261001T000000Z
DTSTART;TZID=Europe/Berlin:20261010T093000
DTEND;TZID=Europe/Berlin:20261010T100000
SUMMARY:Zahnarzt
END:VEVENT
BEGIN:VEVENT
UID:2
DTSTAMP:20261001T000000Z
DTSTART;VALUE=DATE:20261011
DTEND;VALUE=DATE:20261012
SUMMARY:Geburtstag
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

func TestBuildItemsOrder(t *testing.T) {
	loc := berlin(t)
	today := time.Date(2026, 10, 10, 0, 0, 0, 0, loc)
	events := []ical.Event{
		{Title: "Started yesterday", Start: today.Add(-20 * time.Hour), End: today.Add(5 * time.Hour)},
		{Title: "Zahnarzt", Start: today.Add(9*time.Hour + 30*time.Minute)},
		{Title: "Geburtstag", Start: today.AddDate(0, 0, 1), AllDay: true},
	}
	items := buildItems(today, 2, events, "ddd D MMM", "de")
	var got []string
	for _, it := range items {
		got = append(got, it.title)
	}
	want := "Sa 10 Okt|Started yesterday|Zahnarzt|So 11 Okt|Geburtstag"
	if strings.Join(got, "|") != want {
		t.Errorf("got %q want %q", strings.Join(got, "|"), want)
	}
}

func TestLayoutNoOrphanHeaderAndRepeatHeader(t *testing.T) {
	cv := render.NewCanvas(200, 100, render.FontDefault, 14)
	lh := 10
	items := []item{
		{isDate: true, title: "D1"}, {title: "e1"}, {title: "e2"},
		{isDate: true, title: "D2"}, {title: "e3"}, {title: "e4"},
	}
	// column height 40 → 4 lines per column, 2 columns
	rows := layout(items, cv, lh, 40, 2, 1000)
	var got []string
	for _, r := range rows {
		got = append(got, r.text+"@"+string(rune('0'+r.col)))
	}
	// D2 would land on line 4 of column 0 with its event below the edge → moves to column 1
	want := "D1@0|e1@0|e2@0|D2@1|e3@1|e4@1"
	if strings.Join(got, "|") != want {
		t.Errorf("got %s want %s", strings.Join(got, "|"), want)
	}

	items = []item{{isDate: true, title: "D1"}, {title: "e1"}, {title: "e2"}, {title: "e3"}, {title: "e4"}}
	rows = layout(items, cv, lh, 40, 2, 1000)
	got = nil
	for _, r := range rows {
		got = append(got, r.text+"@"+string(rune('0'+r.col)))
	}
	want = "D1@0|e1@0|e2@0|e3@0|D1@1|e4@1" // header repeated at the top of column 1
	if strings.Join(got, "|") != want {
		t.Errorf("got %s want %s", strings.Join(got, "|"), want)
	}
}

func TestLayoutStopsWhenColumnsFull(t *testing.T) {
	cv := render.NewCanvas(200, 100, render.FontDefault, 14)
	items := []item{{isDate: true, title: "D1"}, {title: "a"}, {title: "b"}, {title: "c"}}
	rows := layout(items, cv, 10, 20, 1, 1000)
	if len(rows) != 2 {
		t.Errorf("expected 2 rows in a 2-line column, got %d", len(rows))
	}
}

func TestNewValidatesColumns(t *testing.T) {
	c := config.Common{Size: [2]int{400, 300}, FontSize: 14, Language: "en"}
	if _, err := New(c, json.RawMessage(`{"ical_urls":"x","columns":3}`)); err == nil {
		t.Fatal("expected error for columns=3")
	}
}

func TestGenerate(t *testing.T) {
	loc := berlin(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(ics))
	}))
	defer srv.Close()
	c := config.Common{Size: [2]int{480, 300}, PaddingX: 10, PaddingY: 10, FontSize: 14, Language: "de"}
	m, err := New(c, json.RawMessage(`{"ical_urls":"`+srv.URL+`"}`))
	if err != nil {
		t.Fatal(err)
	}
	env := &modules.Env{Now: time.Date(2026, 10, 10, 8, 0, 0, 0, loc), Loc: loc, Fetch: fetch.New(t.TempDir())}
	cv, err := m.Generate(context.Background(), env)
	if err != nil {
		t.Fatal(err)
	}
	if cv.W != 460 || cv.H != 280 {
		t.Fatalf("canvas %dx%d", cv.W, cv.H)
	}
	if _, col := cv.InkAt(100, 0); !col {
		t.Error("expected red date separator line at the top")
	}
	if env.Stale {
		t.Error("fresh data reported as stale")
	}
}
