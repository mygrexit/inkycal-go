package config

import (
	"os"
	"path/filepath"
	"testing"
)

const pythonSettings = `{
    "model": "epd_7_in_5_v2_colour",
    "update_interval": "30",
    "orientation": 0,
    "info_section": true,
    "calibration_hours": [0, 12, 18],
    "border_around_modules": "True",
    "modules": [
        {"position": 2, "name": "Agenda", "config": {"size": [480, 400], "ical_urls": "https://x/a.ics", "padding_x": 10, "padding_y": 10, "fontsize": 14, "language": "de"}},
        {"position": 1, "name": "Weather", "config": {"size": [480, 100], "padding_x": 10, "padding_y": 10, "fontsize": 12, "language": "de"}}
    ]
}`

func writeTemp(t *testing.T, content string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "settings.json")
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestLoadPythonSettings(t *testing.T) {
	s, err := Load(writeTemp(t, pythonSettings))
	if err != nil {
		t.Fatal(err)
	}
	if s.UpdateInterval != 30 || !bool(s.BorderAroundModules) || s.InfoSectionHeight != 70 || s.CalibrationCycles != 3 {
		t.Errorf("unexpected settings: %+v", s)
	}
	if s.Modules[0].Name != "Weather" || s.Modules[1].Name != "Agenda" {
		t.Errorf("modules not sorted by position: %s, %s", s.Modules[0].Name, s.Modules[1].Name)
	}
	c, err := ParseCommon(s.Modules[1].Config)
	if err != nil {
		t.Fatal(err)
	}
	if w, h := c.InnerSize(); w != 460 || h != 380 {
		t.Errorf("inner size %dx%d", w, h)
	}
	if c.Language != "de" || c.FontSize != 14 {
		t.Errorf("common: %+v", c)
	}
}

func TestLoadRejectsBadOrientation(t *testing.T) {
	if _, err := Load(writeTemp(t, `{"model":"x","orientation":90}`)); err == nil {
		t.Fatal("expected error")
	}
}

func TestLoadRejectsMissingModel(t *testing.T) {
	if _, err := Load(writeTemp(t, `{"update_interval": 5}`)); err == nil {
		t.Fatal("expected error")
	}
}

func TestParseCommonDefaultsAndTooSmall(t *testing.T) {
	c, err := ParseCommon([]byte(`{"size":[100,50]}`))
	if err != nil {
		t.Fatal(err)
	}
	if c.FontSize != 14 || c.Language != "en" {
		t.Errorf("defaults not applied: %+v", c)
	}
	if _, err := ParseCommon([]byte(`{"size":[20,20],"padding_x":10,"padding_y":10}`)); err == nil {
		t.Error("expected error for zero inner size")
	}
}

func TestFindSettings(t *testing.T) {
	p := writeTemp(t, `{}`)
	got, err := FindSettings(p)
	if err != nil || got != p {
		t.Fatalf("explicit: %q %v", got, err)
	}
	t.Setenv("INKYCAL_SETTINGS_PATH", p)
	old := SearchPaths
	SearchPaths = []string{filepath.Join(t.TempDir(), "missing.json")}
	defer func() { SearchPaths = old }()
	got, err = FindSettings("")
	if err != nil || got != p {
		t.Fatalf("env: %q %v", got, err)
	}
	t.Setenv("INKYCAL_SETTINGS_PATH", "")
	if _, err := FindSettings(""); err == nil {
		t.Fatal("expected error when nothing exists")
	}
}

func TestLoadRejectsBadUpdateTimes(t *testing.T) {
	p := filepath.Join(t.TempDir(), "settings.json")
	os.WriteFile(p, []byte(`{"model":"epd_7_in_5_v2_colour","update_times":["6 Uhr"],"modules":[]}`), 0o600)
	if _, err := Load(p); err == nil {
		t.Error("invalid update_times accepted")
	}
}
