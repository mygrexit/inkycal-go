package app

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"inkycal/internal/config"
	"inkycal/internal/epd"
	"inkycal/internal/fetch"
)

// The dashboard as it hangs in the kitchen: 12.48" panel in portrait, two family calendars.
// Set INKYCAL_PREVIEW_DIR to keep the rendered PNGs for a visual check.
const previewSettings = `{
  "model": "epd_12_in_48_colour_V2",
  "update_interval": 720,
  "orientation": 0,
  "info_section": false,
  "calibration_hours": [],
  "border_around_modules": false,
  "dividers": true,
  "image_hash": false,
  "modules": [
    {"position": 1, "name": "Weather", "config": {"size": [984, 290], "padding_x": 40, "padding_y": 20, "fontsize": 20, "language": "de",
      "api_key": "test", "location": "2825297", "units": "metric", "hour_format": 24}},
    {"position": 2, "name": "Calendar", "config": {"size": [984, 660], "padding_x": 40, "padding_y": 18, "fontsize": 22, "language": "de",
      "week_starts_on": "Monday", "show_events": true, "ical_urls": ["{{BASE}}/alex.ics", "{{BASE}}/sam.ics"],
      "date_format": "DD. MMM", "time_format": "HH:mm", "dedupe_events": true, "calendar_labels": "Alex,Sam", "weekday_prefix": true}},
    {"position": 3, "name": "Todoist", "config": {"size": [984, 180], "padding_x": 40, "padding_y": 18, "fontsize": 22, "language": "de",
      "api_key": "{{TODOIST}}"}},
    {"position": 4, "name": "Greek", "config": {"size": [984, 174], "padding_x": 40, "padding_y": 18, "fontsize": 22, "language": "de"}}
  ]
}`

func previewICS(events ...string) string {
	return "BEGIN:VCALENDAR\r\nVERSION:2.0\r\nPRODID:-//inkycal//preview//EN\r\n" + strings.Join(events, "") + "END:VCALENDAR\r\n"
}

func previewEvent(uid, start, end, title string) string {
	return fmt.Sprintf("BEGIN:VEVENT\r\nUID:%s\r\nDTSTAMP:20261001T000000Z\r\nDTSTART;TZID=Europe/Berlin:%s\r\nDTEND;TZID=Europe/Berlin:%s\r\nSUMMARY:%s\r\nEND:VEVENT\r\n", uid, start, end, title)
}

func previewAllDay(uid, start, end, title string) string {
	return fmt.Sprintf("BEGIN:VEVENT\r\nUID:%s\r\nDTSTAMP:20261001T000000Z\r\nDTSTART;VALUE=DATE:%s\r\nDTEND;VALUE=DATE:%s\r\nSUMMARY:%s\r\nEND:VEVENT\r\n", uid, start, end, title)
}

func previewServer(t testing.TB, now time.Time, todoistOK bool) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	day := func(h int) int64 {
		return time.Date(now.Year(), now.Month(), now.Day(), h, 0, 0, 0, now.Location()).Unix()
	}
	mux.HandleFunc("/data/2.5/weather", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, `{"weather":[{"icon":"04n"}],"main":{"temp":7.6,"temp_max":9,"humidity":90},"wind":{"speed":3.2,"gust":6},"sys":{"sunrise":%d,"sunset":%d}}`,
			day(7)+54*60, day(18)+48*60)
	})
	mux.HandleFunc("/data/2.5/forecast", func(w http.ResponseWriter, r *http.Request) {
		var list []string
		for i, s := range []struct {
			h     int
			temp  float64
			icon  string
			speed float64
		}{{8, 8.4, "04d", 3}, {11, 11.6, "03d", 5}, {14, 14.2, "10d", 7}, {17, 13.1, "10d", 6.5}, {20, 10.4, "04n", 4}, {23, 9.1, "04n", 3}, {26, 8, "01n", 2}} {
			_ = i
			list = append(list, fmt.Sprintf(`{"dt":%d,"main":{"temp":%v},"weather":[{"icon":%q}],"wind":{"speed":%v}}`, day(s.h), s.temp, s.icon, s.speed))
		}
		fmt.Fprintf(w, `{"list":[%s]}`, strings.Join(list, ","))
	})
	d := func(offset, h, m int) string {
		t := time.Date(now.Year(), now.Month(), now.Day()+offset, h, m, 0, 0, now.Location())
		return t.Format("20060102T150405")
	}
	date := func(offset int) string {
		return time.Date(now.Year(), now.Month(), now.Day()+offset, 0, 0, 0, 0, now.Location()).Format("20060102")
	}
	isoDate := func(offset int) string {
		return time.Date(now.Year(), now.Month(), now.Day()+offset, 0, 0, 0, 0, now.Location()).Format("2006-01-02")
	}
	shared := []string{
		previewEvent("kinderarzt", d(0, 11, 30), d(0, 12, 30), "Kinderarzt U7"),
		previewEvent("kita", d(0, 13, 30), d(0, 15, 30), "Laternenbasteln Kita"),
		previewEvent("spielplatz", d(1, 16, 0), d(1, 18, 0), "Spielplatz mit Mia"),
		previewEvent("pt", d(2, 16, 0), d(2, 17, 0), "Personal Training"),
		previewEvent("past", d(-6, 9, 0), d(-6, 10, 0), "Elternabend"),
		previewEvent("later", d(9, 19, 0), d(9, 22, 0), "Kino"),
	}
	alex := append([]string{
		previewEvent("friseur", d(0, 10, 0), d(0, 11, 0), "Friseur"),
		previewEvent("griechisch", d(0, 16, 30), d(0, 17, 30), "Griechischkurs"),
		previewEvent("werkstatt", d(4, 8, 0), d(4, 9, 0), "Auto in die Werkstatt"),
	}, shared...)
	sam := append([]string{
		previewEvent("schwimmen", d(1, 16, 15), d(1, 17, 0), "Babyschwimmen"),
		previewEvent("sport", d(3, 14, 0), d(3, 15, 0), "Yoga"),
		previewAllDay("oma", date(4), date(7), "Oma und Opa zu Besuch"),
	}, shared...)
	mux.HandleFunc("/alex.ics", func(w http.ResponseWriter, r *http.Request) { io.WriteString(w, previewICS(alex...)) })
	mux.HandleFunc("/sam.ics", func(w http.ResponseWriter, r *http.Request) { io.WriteString(w, previewICS(sam...)) })
	mux.HandleFunc("/api/v1/projects", func(w http.ResponseWriter, r *http.Request) {
		if !todoistOK {
			http.Error(w, `{"error":"Unauthorized"}`, http.StatusUnauthorized)
			return
		}
		io.WriteString(w, `{"results":[{"id":"p1","name":"Inbox"},{"id":"p2","name":"Einkauf"},{"id":"p3","name":"Haushalt"}],"next_cursor":null}`)
	})
	mux.HandleFunc("/api/v1/tasks", func(w http.ResponseWriter, r *http.Request) {
		if !todoistOK {
			http.Error(w, `{"error":"Unauthorized"}`, http.StatusUnauthorized)
			return
		}
		fmt.Fprintf(w, `{"results":[
		  {"content":"Kita-Anmeldung abgeben","project_id":"p1","priority":4,"due":{"date":%q}},
		  {"content":"Milch und Brot","project_id":"p2","priority":1,"due":{"date":%q}},
		  {"content":"Geschenk für Oma besorgen","project_id":"p2","priority":2},
		  {"content":"Fahrradschloss reparieren","project_id":"p3","priority":1},
		  {"content":"Winterreifen aufziehen lassen","project_id":"p3","priority":3,"due":{"date":%q}},
		  {"content":"Fotos für das Album aussuchen","project_id":"p1","priority":1}
		],"next_cursor":null}`, isoDate(-1), isoDate(0), isoDate(3))
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

// previewOptions sets up the kitchen dashboard against a local fixture server.
func previewOptions(t testing.TB, now time.Time, todoistOK bool, display epd.Display) *Options {
	t.Helper()
	srv := previewServer(t, now, todoistOK)
	target, err := url.Parse(srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	settingsPath := filepath.Join(dir, "settings.json")
	body := strings.NewReplacer("{{BASE}}", srv.URL, "{{TODOIST}}", "test").Replace(previewSettings)
	if err := os.WriteFile(settingsPath, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	settings, err := config.Load(settingsPath)
	if err != nil {
		t.Fatal(err)
	}
	fc := fetch.New(filepath.Join(dir, "http"))
	fc.HTTP = &http.Client{Transport: p2Rewrite{target: target}, Timeout: 10 * time.Second}
	fc.Attempts = 1
	return &Options{
		Settings: settings,
		StateDir: dir,
		Display:  display,
		System:   &p2System{},
		Fetch:    fc,
		Loc:      now.Location(),
		Now:      func() time.Time { return now },
		Registry: Registry,
	}
}

func renderPreview(t *testing.T, name string, now time.Time, todoistOK bool) Result {
	t.Helper()
	out := filepath.Join(t.TempDir(), name+".png")
	if keep := os.Getenv("INKYCAL_PREVIEW_DIR"); keep != "" {
		out = filepath.Join(keep, name+".png")
	}
	res, err := RunCycle(context.Background(), previewOptions(t, now, todoistOK, epd.NewPNG(out, 1304, 984)))
	if err != nil {
		t.Fatal(err)
	}
	return res
}

func TestPreviewDashboard(t *testing.T) {
	loc, err := time.LoadLocation("Europe/Berlin")
	if err != nil {
		t.Fatal(err)
	}
	morning := time.Date(2026, 10, 13, 6, 5, 0, 0, loc)
	if res := renderPreview(t, "dashboard-0600", morning, true); len(res.ModuleErrors) != 0 {
		t.Fatalf("module errors: %v", res.ModuleErrors)
	}
	noon := time.Date(2026, 10, 13, 12, 5, 0, 0, loc)
	if res := renderPreview(t, "dashboard-1200", noon, true); len(res.ModuleErrors) != 0 {
		t.Fatalf("module errors: %v", res.ModuleErrors)
	}
	// An expired Todoist token must show a hint in the Todoist area, not break the dashboard.
	res := renderPreview(t, "dashboard-todoist-401", morning, false)
	if len(res.ModuleErrors) != 1 || !strings.Contains(res.ModuleErrors[0], "Todoist") {
		t.Fatalf("module errors: %v", res.ModuleErrors)
	}
}
