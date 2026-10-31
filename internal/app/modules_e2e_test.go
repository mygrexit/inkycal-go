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
	"sync"
	"testing"
	"time"
	_ "time/tzdata"

	"inkycal/internal/config"
	"inkycal/internal/epd"
	"inkycal/internal/fetch"
	"inkycal/internal/fetch/fetchtest"
)

type p2System struct {
	mu         sync.Mutex
	poweredOff bool
}

func (s *p2System) NTPSynced() bool       { return true }
func (s *p2System) Uptime() time.Duration { return 10 * time.Second }
func (s *p2System) Maintenance() bool     { return false }
func (s *p2System) PowerOff() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.poweredOff = true
	return nil
}

const p2ICS = "BEGIN:VCALENDAR\r\nVERSION:2.0\r\nPRODID:-//inkycal//test//EN\r\n" +
	"BEGIN:VEVENT\r\nUID:1@test\r\nDTSTAMP:20261001T000000Z\r\nDTSTART:20261010T080000Z\r\nDTEND:20261010T090000Z\r\nSUMMARY:Zahnarzt\r\nEND:VEVENT\r\n" +
	"BEGIN:VEVENT\r\nUID:2@test\r\nDTSTAMP:20261001T000000Z\r\nDTSTART;VALUE=DATE:20261012\r\nDTEND;VALUE=DATE:20261013\r\nSUMMARY:Urlaub\r\nEND:VEVENT\r\n" +
	"END:VCALENDAR\r\n"

const p2RSS = `<?xml version="1.0"?><rss version="2.0"><channel><title>T</title>
<item><title>Schlagzeile</title><description>Kurzer Text zur Meldung</description></item>
</channel></rss>`

const p2Settings = `{
  "model": "epd_7_in_5_v2_colour",
  "update_interval": 60,
  "orientation": 0,
  "info_section": true,
  "info_section_height": 70,
  "calibration_hours": [],
  "border_around_modules": true,
  "image_hash": false,
  "use_pi_sugar": false,
  "shutdown_after_run": false,
  "modules": [
    {"position": 1, "name": "Weather", "config": {"size": [480, 160], "padding_x": 10, "padding_y": 10, "fontsize": 12, "language": "de",
      "api_key": "test", "location": "Berlin,DE", "forecast_interval": "daily", "units": "metric", "hour_format": 24,
      "use_beaufort": false, "round_temperature": true, "round_windspeed": true}},
    {"position": 2, "name": "Agenda", "config": {"size": [480, 150], "padding_x": 10, "padding_y": 10, "fontsize": 12, "language": "de",
      "ical_urls": "{{BASE}}/cal.ics", "ical_files": null, "date_format": "ddd D MMM", "time_format": "HH:mm", "columns": 1}},
    {"position": 3, "name": "Calendar", "config": {"size": [480, 260], "padding_x": 10, "padding_y": 10, "fontsize": 12, "language": "de",
      "week_starts_on": "Monday", "show_events": true, "ical_urls": "{{BASE}}/cal.ics", "ical_files": null,
      "date_format": "D MMM", "time_format": "HH:mm"}},
    {"position": 4, "name": "Todoist", "config": {"size": [480, 110], "padding_x": 10, "padding_y": 10, "fontsize": 12, "language": "de",
      "api_key": "test", "project_filter": "", "show_priority": true}},
    {"position": 5, "name": "Feeds", "config": {"size": [480, 110], "padding_x": 10, "padding_y": 10, "fontsize": 12, "language": "de",
      "feed_urls": "{{BASE}}/feed.xml", "shuffle_feeds": false}}
  ]
}`

func TestRunCycleAllModules(t *testing.T) {
	loc, err := time.LoadLocation("Europe/Berlin")
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 10, 10, 9, 0, 0, 0, loc)

	var mu sync.Mutex
	hits := map[string]int{}
	mux := http.NewServeMux()
	handle := func(path, body string) {
		mux.HandleFunc(path, func(w http.ResponseWriter, r *http.Request) {
			mu.Lock()
			hits[path]++
			mu.Unlock()
			io.WriteString(w, body)
		})
	}
	handle("/data/2.5/weather", fmt.Sprintf(
		`{"weather":[{"icon":"04d"}],"main":{"temp":11.2,"temp_max":14.0,"humidity":70},"wind":{"speed":3.0},"sys":{"sunrise":%d,"sunset":%d}}`,
		now.Add(-90*time.Minute).Unix(), now.Add(9*time.Hour).Unix()))
	handle("/data/2.5/forecast", fmt.Sprintf(
		`{"list":[{"dt":%d,"main":{"temp":12},"weather":[{"icon":"10d"}]},{"dt":%d,"main":{"temp":9},"weather":[{"icon":"04d"}]}]}`,
		now.Add(24*time.Hour).Unix(), now.Add(27*time.Hour).Unix()))
	handle("/api/v1/projects", `{"results":[{"id":"p1","name":"Inbox"}],"next_cursor":null}`)
	handle("/api/v1/tasks", `{"results":[{"content":"Milch kaufen","project_id":"p1","priority":4,"due":{"date":"2026-10-10"}}],"next_cursor":null}`)
	handle("/cal.ics", p2ICS)
	handle("/feed.xml", p2RSS)
	srv := httptest.NewServer(mux)
	defer srv.Close()
	target, err := url.Parse(srv.URL)
	if err != nil {
		t.Fatal(err)
	}

	dir := t.TempDir()
	settingsPath := filepath.Join(dir, "settings.json")
	if err := os.WriteFile(settingsPath, []byte(strings.ReplaceAll(p2Settings, "{{BASE}}", srv.URL)), 0o600); err != nil {
		t.Fatal(err)
	}
	settings, err := config.Load(settingsPath)
	if err != nil {
		t.Fatal(err)
	}
	fc := fetch.New(filepath.Join(dir, "http"))
	fc.HTTP = &http.Client{Transport: fetchtest.Rewrite{Target: target}, Timeout: 10 * time.Second}
	preview := filepath.Join(dir, "preview.png")
	sys := &p2System{}

	res, err := RunCycle(context.Background(), &Options{
		Settings: settings,
		StateDir: dir,
		Display:  epd.NewPNG(preview, 800, 480),
		System:   sys,
		Fetch:    fc,
		Loc:      loc,
		Now:      func() time.Time { return now },
		Registry: Registry,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.ModuleErrors) != 0 {
		t.Fatalf("module errors: %v", res.ModuleErrors)
	}
	if !res.Refreshed {
		t.Error("first cycle did not refresh the display")
	}
	if info, err := os.Stat(preview); err != nil || info.Size() == 0 {
		t.Errorf("preview PNG missing or empty: %v", err)
	}
	for _, path := range []string{"/data/2.5/weather", "/data/2.5/forecast", "/api/v1/projects", "/api/v1/tasks", "/cal.ics", "/feed.xml"} {
		if hits[path] == 0 {
			t.Errorf("%s was never requested", path)
		}
	}
	if sys.poweredOff {
		t.Error("PowerOff called although use_pi_sugar is false")
	}
}
