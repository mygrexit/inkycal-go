package main

import (
	"image/png"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const ics = `BEGIN:VCALENDAR
VERSION:2.0
PRODID:test
BEGIN:VEVENT
UID:1
DTSTAMP:20261001T000000Z
DTSTART:20300101T090000Z
DTEND:20300101T100000Z
SUMMARY:Future
END:VEVENT
END:VCALENDAR
`

func TestDryRunWritesPreview(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.Write([]byte(ics)) }))
	defer srv.Close()
	dir := t.TempDir()
	settings := `{"model":"epd_7_in_5_v2_colour","update_interval":60,"orientation":0,"info_section":true,
	  "info_section_height":40,"calibration_hours":[],"border_around_modules":true,
	  "modules":[{"position":1,"name":"Agenda","config":{"size":[480,760],"ical_urls":"URL",
	  "ical_files":null,"date_format":"ddd D MMM","time_format":"HH:mm","padding_x":10,"padding_y":10,"fontsize":14,"language":"de"}}]}`
	sp := filepath.Join(dir, "settings.json")
	os.WriteFile(sp, []byte(strings.Replace(settings, "URL", srv.URL, 1)), 0o644)
	out := filepath.Join(dir, "preview.png")

	if code := run([]string{"--mode", "dry-run", "--settings", sp, "--out", out, "--state-dir", filepath.Join(dir, "state")}); code != 0 {
		t.Fatalf("exit code %d", code)
	}
	f, err := os.Open(out)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	img, err := png.Decode(f)
	if err != nil {
		t.Fatal(err)
	}
	if b := img.Bounds(); b.Dx() != 800 || b.Dy() != 480 {
		t.Fatalf("preview %v", b)
	}
	red := 0
	for y := 0; y < 480; y++ {
		for x := 0; x < 800; x++ {
			r, g, _, _ := img.At(x, y).RGBA()
			if r > 0xF000 && g < 0x1000 {
				red++
			}
		}
	}
	if red == 0 {
		t.Error("expected red agenda separator lines in preview")
	}
	if _, err := os.Stat(filepath.Join(dir, "state", "state.json")); err != nil {
		t.Error("state not written")
	}
}

func TestExitCodes(t *testing.T) {
	if code := run([]string{"--mode", "bogus"}); code != 2 {
		t.Errorf("bad mode: %d", code)
	}
	if code := run([]string{"--mode", "dry-run", "--settings", filepath.Join(t.TempDir(), "missing.json")}); code != 1 {
		t.Errorf("missing settings: %d", code)
	}
}
