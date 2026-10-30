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
	for y := range 480 {
		for x := range 800 {
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

func TestBuilderModeFailsOnBadListenAddress(t *testing.T) {
	dir := t.TempDir()
	code := run([]string{"--mode", "builder", "--settings", filepath.Join(dir, "settings.json"),
		"--state-dir", dir, "--listen", "not-an-address"})
	if code != 1 {
		t.Errorf("exit code %d, want 1", code)
	}
}

// The state dir decides where state.json and the HTTP cache live on the device.
func TestResolveStateDir(t *testing.T) {
	t.Setenv("STATE_DIRECTORY", "")
	cases := []struct {
		flag, env, mode, want string
	}{
		{"/flag", "/systemd", "run", "/flag"},
		{"", "/systemd", "run", "/systemd"},
		{"", "", "dry-run", filepath.Join(os.TempDir(), "inkycal-dry-run")},
		{"", "", "run", "/var/lib/inkycal"},
		{"", "", "once", "/var/lib/inkycal"},
	}
	for _, c := range cases {
		t.Setenv("STATE_DIRECTORY", c.env)
		if got := resolveStateDir(c.flag, c.mode); got != c.want {
			t.Errorf("flag %q env %q mode %s: %s, want %s", c.flag, c.env, c.mode, got, c.want)
		}
	}
}

func TestUnknownModelFails(t *testing.T) {
	dir := t.TempDir()
	sp := filepath.Join(dir, "settings.json")
	os.WriteFile(sp, []byte(`{"model":"no_such_panel","modules":[]}`), 0o600)
	err := runMode("dry-run", sp, filepath.Join(dir, "out.png"), dir, "")
	if err == nil || !strings.Contains(err.Error(), "no_such_panel") {
		t.Errorf("err = %v, want unsupported model", err)
	}
}
