# Inkycal-Go Plan 1: Kern, Display, PiSugar, Agenda – Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Ein lauffähiges Go-Binary, das auf dem Pi Zero 2 W mit PiSugar das Agenda-Modul auf dem Waveshare-7,5″-V2-Farbdisplay anzeigt, den RTC-Wecker stellt und sich ausschaltet – mit minimaler Wachzeit.

**Architecture:** Ein statisches Binary (`cmd/inkycal`) orchestriert einen Zyklus (`internal/app`): Module laufen parallel und zeichnen auf einen Zwei-Ebenen-Canvas (`internal/render`), das zusammengesetzte Bild wird in Panel-Bitebenen gepackt und – nur wenn sich der Hash geändert hat – über SPI/GPIO (`internal/epd`) angezeigt. Während das Panel refresht, werden RTC-Zeit und Wecker über `pisugar-server` gesetzt; danach entscheidet eine abgesicherte Regel über das Ausschalten.

**Tech Stack:** Go 1.26, `golang.org/x/image` (OpenType-Text), `golang.org/x/sys` (ioctl, adjtimex), `github.com/warthog618/go-gpiocdev` (GPIO), `github.com/apognu/gocal` (iCal + RRULE).

**Spec:** `docs/superpowers/specs/2026-10-10-inkycal-go-design.md`

**Danach:** Plan 2 (`docs/superpowers/plans/2026-10-10-inkycal-go-plan2-modules.md`) – Weather, Calendar, Todoist, Feeds.

## Global Constraints

- Go 1.26 (`go 1.26` in go.mod; `golang.org/x/image v0.47.0` verlangt ≥ 1.26), Modulpfad `inkycal`
- Lizenz GPL-3.0-only (abgeleitet von Inkycal, GPL-3.0); Fonts behalten OFL 1.1 / Apache 2.0
- `CGO_ENABLED=0`; Ziele `linux/arm64` und `linux/arm` mit `GOARM=7`
- Erlaubte Abhängigkeiten: `golang.org/x/image v0.47.0`, `golang.org/x/sys v0.49.0`, `github.com/warthog618/go-gpiocdev v0.9.2`, `github.com/apognu/gocal v0.9.1` (`gofeed` erst in Plan 2)
- Tests nur mit der Standardbibliothek, kein Netzwerk in Tests (`httptest`)
- Logging mit `log/slog` (Text auf stderr → journald)
- `settings.json` bleibt kompatibel zum Python-Format (gleiche Schlüssel)
- Pins wie Waveshare: RST=17, DC=25, PWR=18, BUSY=24, SPI `/dev/spidev0.0`, 4 MHz, Mode 0
- Ausgabe muss nicht pixelgleich zu Python sein; Layout und Inhalte gleich
- Namen in Test-Dateien des Pakets `app` dürfen nicht mit `p2` beginnen (reserviert für Plan 2)

## Review Focus

1. **Ganztägiger Termin, der heute beginnt** – gocal schließt Termine, die exakt am Bereichsanfang starten, aus; der Geburtstag „heute“ würde fehlen. Erwartet: sichtbar unter dem heutigen Datum. → `TestParse` (Task 10).
2. **Weckzeit liegt zu knapp in der Zukunft** (z.B. 10:14:30 bei 15-min-Intervall) – der RTC-Wecker würde ablaufen, während der Pi noch herunterfährt, und das Gerät wacht nie wieder auf. Erwartet: nächster Slot (≥ 90 s). → `TestNextWake` (Task 8).
3. **Ausschalten, obwohl der Wecker nicht gesetzt ist / USB-Strom / Wartungsdatei** – Gerät wäre tot oder per SSH unerreichbar. Erwartet: bleibt an. → `TestPowerOffGuards` (Task 12).
4. **Outlook/Exchange-Kalender mit Windows-TZIDs** (`W. Europe Standard Time`) – gocal fällt still auf UTC zurück, Termine um 1–2 h verschoben. Erwartet: richtige Ortszeit. → `TestParse` „Standup 08:00“ (Task 10).
5. **Kein Netz beim Aufwachen** – Erwartet: letzte gecachte Daten anzeigen, Info-Zeile markiert „im N: cache“, API-Keys tauchen nie im Log auf. → `TestGetSuccessAndCacheFallback`, `TestNetworkErrorDoesNotLeakKey` (Task 7), `TestInfoText` (Task 12).

---

## Dateistruktur

| Datei | Verantwortung |
|---|---|
| `go.mod`, `LICENSE`, `.gitignore`, `Makefile` | Projekt, Lizenz, Build/Deploy |
| `internal/config/flex.go` | tolerante JSON-Typen (`FlexInt`, `FlexBool`, `StringList`) |
| `internal/config/config.go` | `settings.json` laden, Defaults, Validierung, Suche |
| `internal/timefmt/locale.go`, `format.go` | Arrow-Tokens, Locales en/de, Humanize |
| `internal/render/fonts.go` + `fonts/` | eingebettete Fonts |
| `internal/render/canvas.go` | Zwei-Ebenen-Canvas, Ink, Paste |
| `internal/render/text.go` | Text messen/umbrechen/schreiben, Icons |
| `internal/render/shapes.go` | Linien, abgerundete Rahmen, Kreise |
| `internal/render/compose.go` | Module stapeln + Info-Zeile |
| `internal/render/pack.go` | Drehen, Bitebenen, Hash, Vorschau |
| `internal/epd/epd.go` | `Display`/`Bus`-Interfaces, PNG-Display |
| `internal/epd/epd7in5bv2.go` | Treiber 7,5″ V2 Farbe |
| `internal/epd/bus_linux.go`, `bus_other.go` | spidev + gpiocdev, `Open` |
| `internal/fetch/fetch.go` | HTTP mit Retry + Disk-Cache-Fallback |
| `internal/state/state.go` | `state.json` atomar, `timings.csv` |
| `internal/schedule/schedule.go` | Weckzeit, Kalibrierungs-Fälligkeit |
| `internal/pisugar/pisugar.go` | TCP-Client für `pisugar-server` |
| `internal/sysinfo/*.go` | NTP-Status, Uptime, PowerOff, Wartungsflag |
| `internal/ical/ical.go` | iCal laden/expandieren |
| `internal/modules/module.go` | `Module`, `Factory`, `Env` |
| `internal/modules/agenda/agenda.go` | Agenda-Modul |
| `internal/app/app.go`, `registry.go` | Zyklus, Loop, Registry |
| `cmd/inkycal/main.go` | CLI |
| `deploy/inkycal-go.service`, `deploy/README.md` | systemd, Installation, Messung |

---

### Task 1: Projekt-Grundgerüst und `config`

**Files:**
- Create: `go.mod`, `LICENSE`, `.gitignore`
- Create: `internal/config/flex.go`, `internal/config/config.go`
- Test: `internal/config/flex_test.go`, `internal/config/config_test.go`

**Interfaces:**
- Consumes: –
- Produces: `config.FlexInt`, `config.FlexBool`, `config.StringList`, `config.Settings`, `config.ModuleEntry`, `config.Common` (+ `InnerSize() (w, h int)`), `config.ParseCommon(raw json.RawMessage) (Common, error)`, `config.Load(path string) (*Settings, error)`, `config.FindSettings(explicit string) (string, error)`, `var config.SearchPaths []string`

- [ ] **Step 1: Repository und Modul anlegen**

```bash
git init
go mod init inkycal
go mod edit -go=1.26
curl -fsSL https://www.gnu.org/licenses/gpl-3.0.txt -o LICENSE
```

`.gitignore`:

```
/dist/
preview.png
```

- [ ] **Step 2: Failing Tests schreiben**

`internal/config/flex_test.go`:

```go
package config

import (
	"encoding/json"
	"reflect"
	"testing"
)

func TestFlexInt(t *testing.T) {
	cases := map[string]FlexInt{`5`: 5, `"7"`: 7, `null`: 0, `""`: 0, `12.0`: 12}
	for in, want := range cases {
		var got FlexInt
		if err := json.Unmarshal([]byte(in), &got); err != nil {
			t.Fatalf("%s: %v", in, err)
		}
		if got != want {
			t.Errorf("%s: got %d want %d", in, got, want)
		}
	}
	var bad FlexInt
	if err := json.Unmarshal([]byte(`"abc"`), &bad); err == nil {
		t.Error("expected error for \"abc\"")
	}
}

func TestFlexBool(t *testing.T) {
	cases := map[string]FlexBool{`true`: true, `false`: false, `"True"`: true, `"false"`: false, `null`: false}
	for in, want := range cases {
		var got FlexBool
		if err := json.Unmarshal([]byte(in), &got); err != nil {
			t.Fatalf("%s: %v", in, err)
		}
		if got != want {
			t.Errorf("%s: got %v want %v", in, got, want)
		}
	}
	var bad FlexBool
	if err := json.Unmarshal([]byte(`"yes"`), &bad); err == nil {
		t.Error("expected error for \"yes\"")
	}
}

func TestStringList(t *testing.T) {
	cases := map[string]StringList{
		`"https://a/x.ics, https://b/y.ics"`: {"https://a/x.ics", "https://b/y.ics"},
		`["a", " b "]`:                       {"a", "b"},
		`"a,,"`:                              {"a"},
		`null`:                               nil,
		`""`:                                 {},
	}
	for in, want := range cases {
		var got StringList
		if err := json.Unmarshal([]byte(in), &got); err != nil {
			t.Fatalf("%s: %v", in, err)
		}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("%s: got %#v want %#v", in, got, want)
		}
	}
}
```

`internal/config/config_test.go`:

```go
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
```

- [ ] **Step 3: Tests laufen lassen – müssen fehlschlagen**

Run: `go test ./internal/config/`
Expected: FAIL – Build-Fehler `undefined: FlexInt`, `undefined: Load` usw.

- [ ] **Step 4: Implementieren**

`internal/config/flex.go`:

```go
package config

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

// FlexInt accepts 5, "5" and null (→ 0). The Python web UI is not strict about types.
type FlexInt int

func (f *FlexInt) UnmarshalJSON(b []byte) error {
	b = bytes.TrimSpace(b)
	if string(b) == "null" {
		*f = 0
		return nil
	}
	if len(b) > 0 && b[0] == '"' {
		var s string
		if err := json.Unmarshal(b, &s); err != nil {
			return err
		}
		s = strings.TrimSpace(s)
		if s == "" {
			*f = 0
			return nil
		}
		n, err := strconv.Atoi(s)
		if err != nil {
			return fmt.Errorf("not an integer: %q", s)
		}
		*f = FlexInt(n)
		return nil
	}
	var n float64
	if err := json.Unmarshal(b, &n); err != nil {
		return err
	}
	*f = FlexInt(int(n))
	return nil
}

// FlexBool accepts true/false, "true"/"True"/"false"/"False" and null (→ false).
type FlexBool bool

func (f *FlexBool) UnmarshalJSON(b []byte) error {
	b = bytes.TrimSpace(b)
	switch strings.ToLower(strings.Trim(string(b), `"`)) {
	case "true":
		*f = true
	case "false", "null", "":
		*f = false
	default:
		return fmt.Errorf("not a boolean: %s", b)
	}
	return nil
}

// StringList accepts "a, b" (comma separated), ["a","b"] and null.
type StringList []string

func (s *StringList) UnmarshalJSON(b []byte) error {
	b = bytes.TrimSpace(b)
	if string(b) == "null" {
		*s = nil
		return nil
	}
	var raw []string
	if len(b) > 0 && b[0] == '[' {
		if err := json.Unmarshal(b, &raw); err != nil {
			return err
		}
	} else {
		var one string
		if err := json.Unmarshal(b, &one); err != nil {
			return err
		}
		raw = strings.Split(one, ",")
	}
	out := StringList{}
	for _, v := range raw {
		if v = strings.TrimSpace(v); v != "" {
			out = append(out, v)
		}
	}
	*s = out
	return nil
}
```

`internal/config/config.go`:

```go
// Package config reads Inkycal's settings.json (format compatible with the Python version).
package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"sort"
)

// SearchPaths are tried in order when no explicit settings path is given.
var SearchPaths = []string{"/boot/settings.json", "/boot/firmware/settings.json"}

type Settings struct {
	Model               string        `json:"model"`
	UpdateInterval      FlexInt       `json:"update_interval"`
	Orientation         FlexInt       `json:"orientation"`
	InfoSection         FlexBool      `json:"info_section"`
	InfoSectionHeight   FlexInt       `json:"info_section_height"`
	CalibrationHours    []int         `json:"calibration_hours"`
	CalibrationCycles   FlexInt       `json:"calibration_cycles"`
	BorderAroundModules FlexBool      `json:"border_around_modules"`
	ImageHash           FlexBool      `json:"image_hash"`
	UsePiSugar          FlexBool      `json:"use_pi_sugar"`
	ShutdownAfterRun    FlexBool      `json:"shutdown_after_run"`
	Modules             []ModuleEntry `json:"modules"`
}

type ModuleEntry struct {
	Position int             `json:"position"`
	Name     string          `json:"name"`
	Config   json.RawMessage `json:"config"`
}

// Common holds the keys every module config shares.
type Common struct {
	Size     [2]int  `json:"size"`
	PaddingX FlexInt `json:"padding_x"`
	PaddingY FlexInt `json:"padding_y"`
	FontSize FlexInt `json:"fontsize"`
	Language string  `json:"language"`
}

// InnerSize is the drawable area: size minus padding on both sides.
func (c Common) InnerSize() (w, h int) {
	return c.Size[0] - 2*int(c.PaddingX), c.Size[1] - 2*int(c.PaddingY)
}

// ParseCommon extracts the shared module keys and applies defaults.
func ParseCommon(raw json.RawMessage) (Common, error) {
	var c Common
	if err := json.Unmarshal(raw, &c); err != nil {
		return c, err
	}
	if c.FontSize == 0 {
		c.FontSize = 14
	}
	if c.Language == "" {
		c.Language = "en"
	}
	w, h := c.InnerSize()
	if w <= 0 || h <= 0 {
		return c, fmt.Errorf("module size %v too small for padding %d/%d", c.Size, c.PaddingX, c.PaddingY)
	}
	return c, nil
}

// Load reads, defaults and validates a settings file.
func Load(path string) (*Settings, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var s Settings
	if err := json.Unmarshal(data, &s); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	if s.UpdateInterval <= 0 {
		s.UpdateInterval = 60
	}
	if s.InfoSection && s.InfoSectionHeight <= 0 {
		s.InfoSectionHeight = 70
	}
	if s.CalibrationCycles <= 0 {
		s.CalibrationCycles = 3
	}
	if s.Orientation != 0 && s.Orientation != 180 {
		return nil, fmt.Errorf("orientation must be 0 or 180, got %d", s.Orientation)
	}
	if s.Model == "" {
		return nil, errors.New("model is missing")
	}
	sort.SliceStable(s.Modules, func(i, j int) bool { return s.Modules[i].Position < s.Modules[j].Position })
	return &s, nil
}

// FindSettings returns explicit if set, else $INKYCAL_SETTINGS_PATH, else the first existing SearchPaths entry.
func FindSettings(explicit string) (string, error) {
	candidates := []string{}
	if explicit != "" {
		candidates = append(candidates, explicit)
	} else {
		if env := os.Getenv("INKYCAL_SETTINGS_PATH"); env != "" {
			candidates = append(candidates, env)
		}
		candidates = append(candidates, SearchPaths...)
	}
	for _, p := range candidates {
		if st, err := os.Stat(p); err == nil && !st.IsDir() {
			return p, nil
		}
	}
	return "", fmt.Errorf("no settings.json found (tried %v)", candidates)
}
```

- [ ] **Step 5: Tests laufen lassen**

Run: `go test ./internal/config/`
Expected: `ok  	inkycal/internal/config`

- [ ] **Step 6: Commit**

```bash
git add go.mod LICENSE .gitignore internal/config
git commit -m "feat(config): settings.json compatible with Python Inkycal"
```

---

### Task 2: `timefmt` – Arrow-Formatierung und Locales

**Files:**
- Create: `internal/timefmt/locale.go`, `internal/timefmt/format.go`
- Test: `internal/timefmt/format_test.go`

**Interfaces:**
- Consumes: –
- Produces: `timefmt.Format(t time.Time, layout, lang string) string`, `timefmt.HumanizeDays(days int, lang string) string`. Unterstützte Tokens: `YYYY YY MMMM MMM MM M DD D dddd ddd d HH H hh h mm m ss s A a` und `[literal]`. `a` → `am`/`pm`, `A` → `AM`/`PM`. Sprachen `en`, `de` (auch `de_DE`, `de-de`), sonst Englisch.

- [ ] **Step 1: Failing Test schreiben**

`internal/timefmt/format_test.go`:

```go
package timefmt

import (
	"testing"
	"time"
)

func TestFormat(t *testing.T) {
	ts := time.Date(2026, time.March, 5, 14, 7, 9, 0, time.UTC) // Thursday
	cases := []struct{ layout, lang, want string }{
		{"ddd D MMM", "en", "Thu 5 Mar"},
		{"ddd D MMM", "de", "Do 5 Mär"},
		{"dddd, D. MMMM YYYY", "de_DE", "Donnerstag, 5. März 2026"},
		{"HH:mm", "en", "14:07"},
		{"h:mm a", "en", "2:07 pm"},
		{"h A", "en", "2 PM"},
		{"D-MMM-YY", "en", "5-Mar-26"},
		{"DD.MM.YYYY HH:mm:ss", "de", "05.03.2026 14:07:09"},
		{"[Week] d", "en", "Week 4"},
		{"MMMM", "xx", "March"},
	}
	for _, c := range cases {
		if got := Format(ts, c.layout, c.lang); got != c.want {
			t.Errorf("Format(%q,%q) = %q, want %q", c.layout, c.lang, got, c.want)
		}
	}
}

func TestFormatMidnightAndNoon12h(t *testing.T) {
	if got := Format(time.Date(2026, 1, 1, 0, 5, 0, 0, time.UTC), "h:mm a", "en"); got != "12:05 am" {
		t.Errorf("midnight: %q", got)
	}
	if got := Format(time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC), "h a", "en"); got != "12 pm" {
		t.Errorf("noon: %q", got)
	}
}

func TestHumanizeDays(t *testing.T) {
	cases := []struct {
		days       int
		lang, want string
	}{
		{1, "en", "a day"}, {3, "en", "3 days"}, {7, "en", "a week"}, {15, "en", "2 weeks"},
		{1, "de", "ein Tag"}, {3, "de", "3 Tage"}, {8, "de", "eine Woche"}, {21, "de", "3 Wochen"},
	}
	for _, c := range cases {
		if got := HumanizeDays(c.days, c.lang); got != c.want {
			t.Errorf("HumanizeDays(%d,%q) = %q, want %q", c.days, c.lang, got, c.want)
		}
	}
}
```

- [ ] **Step 2: Test laufen lassen – muss fehlschlagen**

Run: `go test ./internal/timefmt/`
Expected: FAIL – `undefined: Format`, `undefined: HumanizeDays`

- [ ] **Step 3: Implementieren**

`internal/timefmt/locale.go`:

```go
package timefmt

import "strings"

type locale struct {
	months, monthsShort [12]string
	days, daysShort     [7]string // Monday first
	day, days2          string    // "a day", "%d days"
	week, weeks         string    // "a week", "%d weeks"
}

var locales = map[string]locale{
	"en": {
		months:      [12]string{"January", "February", "March", "April", "May", "June", "July", "August", "September", "October", "November", "December"},
		monthsShort: [12]string{"Jan", "Feb", "Mar", "Apr", "May", "Jun", "Jul", "Aug", "Sep", "Oct", "Nov", "Dec"},
		days:        [7]string{"Monday", "Tuesday", "Wednesday", "Thursday", "Friday", "Saturday", "Sunday"},
		daysShort:   [7]string{"Mon", "Tue", "Wed", "Thu", "Fri", "Sat", "Sun"},
		day:         "a day", days2: "%d days", week: "a week", weeks: "%d weeks",
	},
	"de": {
		months:      [12]string{"Januar", "Februar", "März", "April", "Mai", "Juni", "Juli", "August", "September", "Oktober", "November", "Dezember"},
		monthsShort: [12]string{"Jan", "Feb", "Mär", "Apr", "Mai", "Jun", "Jul", "Aug", "Sep", "Okt", "Nov", "Dez"},
		days:        [7]string{"Montag", "Dienstag", "Mittwoch", "Donnerstag", "Freitag", "Samstag", "Sonntag"},
		daysShort:   [7]string{"Mo", "Di", "Mi", "Do", "Fr", "Sa", "So"},
		day:         "ein Tag", days2: "%d Tage", week: "eine Woche", weeks: "%d Wochen",
	},
}

// lookup normalises "de_DE"/"de-de"/"DE" to "de" and falls back to English.
func lookup(lang string) locale {
	lang = strings.ToLower(lang)
	if i := strings.IndexAny(lang, "-_"); i >= 0 {
		lang = lang[:i]
	}
	if l, ok := locales[lang]; ok {
		return l
	}
	return locales["en"]
}
```

`internal/timefmt/format.go`:

```go
// Package timefmt formats times with Arrow (Python) tokens, as used in Inkycal's settings.json.
package timefmt

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

// tokens are matched longest first.
var tokens = []string{"YYYY", "YY", "MMMM", "MMM", "MM", "M", "DD", "D", "dddd", "ddd", "d", "HH", "H", "hh", "h", "mm", "m", "ss", "s", "A", "a"}

// Format renders t using Arrow tokens (e.g. "ddd D MMM", "HH:mm"). Text in [brackets] is literal.
func Format(t time.Time, layout, lang string) string {
	l := lookup(lang)
	var b strings.Builder
	for i := 0; i < len(layout); {
		if layout[i] == '[' {
			if end := strings.IndexByte(layout[i:], ']'); end > 0 {
				b.WriteString(layout[i+1 : i+end])
				i += end + 1
				continue
			}
		}
		matched := ""
		for _, tok := range tokens {
			if strings.HasPrefix(layout[i:], tok) {
				matched = tok
				break
			}
		}
		if matched == "" {
			b.WriteByte(layout[i])
			i++
			continue
		}
		b.WriteString(render(t, matched, l))
		i += len(matched)
	}
	return b.String()
}

func render(t time.Time, tok string, l locale) string {
	wd := (int(t.Weekday()) + 6) % 7 // Monday = 0
	h12 := t.Hour() % 12
	if h12 == 0 {
		h12 = 12
	}
	switch tok {
	case "YYYY":
		return fmt.Sprintf("%04d", t.Year())
	case "YY":
		return fmt.Sprintf("%02d", t.Year()%100)
	case "MMMM":
		return l.months[t.Month()-1]
	case "MMM":
		return l.monthsShort[t.Month()-1]
	case "MM":
		return fmt.Sprintf("%02d", int(t.Month()))
	case "M":
		return strconv.Itoa(int(t.Month()))
	case "DD":
		return fmt.Sprintf("%02d", t.Day())
	case "D":
		return strconv.Itoa(t.Day())
	case "dddd":
		return l.days[wd]
	case "ddd":
		return l.daysShort[wd]
	case "d":
		return strconv.Itoa(wd + 1)
	case "HH":
		return fmt.Sprintf("%02d", t.Hour())
	case "H":
		return strconv.Itoa(t.Hour())
	case "hh":
		return fmt.Sprintf("%02d", h12)
	case "h":
		return strconv.Itoa(h12)
	case "mm":
		return fmt.Sprintf("%02d", t.Minute())
	case "m":
		return strconv.Itoa(t.Minute())
	case "ss":
		return fmt.Sprintf("%02d", t.Second())
	case "s":
		return strconv.Itoa(t.Second())
	case "A":
		if t.Hour() < 12 {
			return "AM"
		}
		return "PM"
	case "a":
		if t.Hour() < 12 {
			return "am"
		}
		return "pm"
	}
	return tok
}

// HumanizeDays renders a duration in days like Arrow's humanize(only_distance=True).
func HumanizeDays(days int, lang string) string {
	l := lookup(lang)
	switch {
	case days <= 1:
		return l.day
	case days < 7:
		return fmt.Sprintf(l.days2, days)
	case days < 14:
		return l.week
	default:
		return fmt.Sprintf(l.weeks, days/7)
	}
}
```

- [ ] **Step 4: Tests laufen lassen**

Run: `go test ./internal/timefmt/`
Expected: `ok  	inkycal/internal/timefmt`

- [ ] **Step 5: Commit**

```bash
git add internal/timefmt
git commit -m "feat(timefmt): arrow tokens with en/de locales"
```

---

### Task 3: `render` – Fonts, Canvas, Text, Formen

**Files:**
- Create: `internal/render/fonts/` (Font-Dateien, Lizenzen, README)
- Create: `internal/render/fonts.go`, `internal/render/canvas.go`, `internal/render/text.go`, `internal/render/shapes.go`
- Test: `internal/render/canvas_test.go`

**Interfaces:**
- Consumes: –
- Produces: `render.InkThreshold = 220`; `FontID` (`FontDefault`, `FontRegular`, `FontWeather`, `FontMaterial`); `Ink` (`InkBlack`, `InkColour`, `InkWhite`); `Align` (`AlignCenter`, `AlignLeft`, `AlignRight`); `TextOpts{Align, Ink, Autofit, FillHeight}`; `Canvas{W, H int; Black, Colour *image.Gray}` mit `NewCanvas(w, h int, font FontID, size int) *Canvas`, `SetFont(id, size)`, `Font() (FontID, int)`, `LineHeight() int`, `TextWidth(s) int`, `Wrap(text, maxWidth) []string`, `Write(x, y, w, h int, text string, o TextOpts)`, `Icon(x, y, w, h int, glyph string, font FontID, ink Ink, fillRatio float64)`, `HLine(x0, x1, y int, ink)`, `VLine(x, y0, y1 int, ink)`, `RoundedRect(x, y, w, h, r int, ink)`, `FillCircle(cx, cy, r int, ink)`, `Paste(src *Canvas, x, y int)`, `InkAt(x, y) (black, colour bool)`; `Border(c, x, y, w, h, r int, shrinkX, shrinkY float64, ink)`. `Write`/`Icon` zeichnen bei `w <= 0` oder `h <= 0` nichts. `Autofit` ändert die Canvas-Schrift nicht.

- [ ] **Step 1: Fonts aus Inkycal übernehmen und Abhängigkeit holen**

```bash
git clone --depth 1 https://github.com/aceinnolab/Inkycal /tmp/Inkycal-src
mkdir -p internal/render/fonts
F=/tmp/Inkycal-src/inkycal/fonts
cp $F/NotoSans/NotoSans-SemiCondensedMedium.ttf $F/NotoSans/NotoSans-SemiCondensed.ttf \
   $F/NotoSans/LICENSE_OFL.txt $F/WeatherFont/weathericons-regular-webfont.ttf \
   $F/MaterialIcons/MaterialIcons.ttf internal/render/fonts/
curl -fsSL https://www.apache.org/licenses/LICENSE-2.0.txt -o internal/render/fonts/LICENSE_APACHE-2.0.txt
go get golang.org/x/image@v0.47.0
```

`internal/render/fonts/README.md`:

```markdown
# Eingebettete Fonts

| Datei | Herkunft | Lizenz |
|---|---|---|
| `NotoSans-SemiCondensedMedium.ttf`, `NotoSans-SemiCondensed.ttf` | Google Noto, übernommen aus Inkycal | SIL OFL 1.1 (`LICENSE_OFL.txt`) |
| `weathericons-regular-webfont.ttf` | Weather Icons (Erik Flowers), übernommen aus Inkycal | SIL OFL 1.1 (`LICENSE_OFL.txt`) |
| `MaterialIcons.ttf` | Google Material Icons, übernommen aus Inkycal | Apache 2.0 (`LICENSE_APACHE-2.0.txt`) |
```

- [ ] **Step 2: Failing Tests schreiben**

`internal/render/canvas_test.go`:

```go
package render

import "testing"

// inkIn counts ink pixels of one layer inside a rectangle.
func inkIn(c *Canvas, x0, y0, x1, y1 int, colour bool) int {
	n := 0
	for y := y0; y < y1; y++ {
		for x := x0; x < x1; x++ {
			b, col := c.InkAt(x, y)
			if (!colour && b) || (colour && col) {
				n++
			}
		}
	}
	return n
}

func TestNewCanvasIsWhite(t *testing.T) {
	c := NewCanvas(10, 5, FontDefault, 14)
	if inkIn(c, 0, 0, 10, 5, false)+inkIn(c, 0, 0, 10, 5, true) != 0 {
		t.Fatal("new canvas has ink")
	}
}

func TestWriteAlignment(t *testing.T) {
	c := NewCanvas(300, 40, FontDefault, 14)
	c.Write(0, 0, 300, 40, "Hi", TextOpts{Align: AlignLeft})
	if inkIn(c, 0, 0, 40, 40, false) == 0 || inkIn(c, 150, 0, 300, 40, false) != 0 {
		t.Error("left aligned text not on the left")
	}
	c = NewCanvas(300, 40, FontDefault, 14)
	c.Write(0, 0, 300, 40, "Hi", TextOpts{Align: AlignRight})
	if inkIn(c, 260, 0, 300, 40, false) == 0 || inkIn(c, 0, 0, 150, 40, false) != 0 {
		t.Error("right aligned text not on the right")
	}
	c = NewCanvas(300, 40, FontDefault, 14)
	c.Write(0, 0, 300, 40, "Hi", TextOpts{})
	if inkIn(c, 130, 0, 170, 40, false) == 0 || inkIn(c, 0, 0, 100, 40, false) != 0 {
		t.Error("centred text not centred")
	}
}

func TestWriteColourOnlyTouchesColourLayer(t *testing.T) {
	c := NewCanvas(100, 30, FontDefault, 14)
	c.Write(0, 0, 100, 30, "Red", TextOpts{Ink: InkColour})
	if inkIn(c, 0, 0, 100, 30, true) == 0 || inkIn(c, 0, 0, 100, 30, false) != 0 {
		t.Error("InkColour must draw on colour layer only")
	}
}

func TestWriteClipsToBox(t *testing.T) {
	c := NewCanvas(200, 100, FontDefault, 40)
	c.Write(50, 40, 60, 10, "WWWWWWWW", TextOpts{Align: AlignLeft})
	if inkIn(c, 0, 0, 200, 40, false) != 0 || inkIn(c, 0, 50, 200, 100, false) != 0 || inkIn(c, 110, 0, 200, 100, false) != 0 {
		t.Error("text drawn outside its box")
	}
	if inkIn(c, 50, 40, 110, 50, false) == 0 {
		t.Error("first line must always be drawn, even if taller than the box")
	}
}

func TestWrap(t *testing.T) {
	c := NewCanvas(10, 10, FontDefault, 14)
	lines := c.Wrap("one two three four five six", c.TextWidth("one two three"))
	if len(lines) != 2 || lines[0] != "one two three" || lines[1] != "four five six" {
		t.Errorf("wrap: %q", lines)
	}
	if got := c.Wrap("Supercalifragilistic", 5); len(got) != 1 {
		t.Errorf("long word must stay one line: %q", got)
	}
}

func TestWriteDropsLinesThatDoNotFit(t *testing.T) {
	c := NewCanvas(60, 200, FontDefault, 14)
	lh := c.LineHeight()
	c.Write(0, 0, 60, lh, "aaa bbb ccc ddd eee fff", TextOpts{Align: AlignLeft})
	if inkIn(c, 0, lh, 60, 200, false) != 0 {
		t.Error("lines below the box were drawn")
	}
}

func TestAutofitDoesNotChangeCanvasFont(t *testing.T) {
	c := NewCanvas(400, 100, FontDefault, 14)
	c.Write(0, 0, 400, 100, "March", TextOpts{Autofit: true})
	if id, size := c.Font(); id != FontDefault || size != 14 {
		t.Errorf("font changed to %d/%d", id, size)
	}
	if inkIn(c, 0, 0, 400, 30, false) == 0 || inkIn(c, 0, 70, 400, 100, false) == 0 {
		t.Error("autofit text should span most of the box height")
	}
}

func TestIconCentred(t *testing.T) {
	c := NewCanvas(100, 100, FontDefault, 14)
	c.Icon(0, 0, 100, 100, "", FontWeather, InkColour, 0.9) // day-sunny
	if inkIn(c, 40, 40, 60, 60, true) == 0 {
		t.Error("icon centre has no ink")
	}
	if inkIn(c, 0, 0, 100, 3, true) != 0 || inkIn(c, 0, 97, 100, 100, true) != 0 {
		t.Error("icon touches the box edge despite fill ratio 0.9")
	}
}

func TestInkWhiteOnFilledCircle(t *testing.T) {
	c := NewCanvas(60, 60, FontDefault, 14)
	c.FillCircle(30, 30, 20, InkBlack)
	c.FillCircle(30, 30, 20, InkColour)
	if b, col := c.InkAt(30, 30); !b || !col {
		t.Fatal("circle centre must be ink on both layers")
	}
	c.Write(10, 10, 40, 40, "8", TextOpts{Ink: InkWhite, Autofit: true})
	white := 0
	for y := 10; y < 50; y++ {
		for x := 10; x < 50; x++ {
			if b, _ := c.InkAt(x, y); !b && (x-30)*(x-30)+(y-30)*(y-30) < 400 {
				white++
			}
		}
	}
	if white == 0 {
		t.Error("white text not visible inside circle")
	}
}

func TestRoundedRectAndBorder(t *testing.T) {
	c := NewCanvas(50, 30, FontDefault, 14)
	c.RoundedRect(0, 0, 50, 30, 5, InkBlack)
	if b, _ := c.InkAt(25, 0); !b {
		t.Error("top edge missing")
	}
	if b, _ := c.InkAt(25, 29); !b {
		t.Error("bottom edge missing")
	}
	if b, _ := c.InkAt(0, 0); b {
		t.Error("rounded corner pixel should stay white")
	}
	if b, _ := c.InkAt(25, 15); b {
		t.Error("outline must not be filled")
	}
	d := NewCanvas(100, 100, FontDefault, 14)
	Border(d, 0, 0, 100, 100, 0, 0.1, 0.1, InkColour)
	if _, col := d.InkAt(5, 50); !col {
		t.Error("shrunk border left edge expected at x=5")
	}
	if _, col := d.InkAt(0, 50); col {
		t.Error("border not shrunk")
	}
}

func TestPasteOnlyCopiesInk(t *testing.T) {
	dst := NewCanvas(20, 20, FontDefault, 14)
	dst.FillCircle(10, 10, 9, InkBlack)
	src := NewCanvas(5, 5, FontDefault, 14)
	src.HLine(0, 4, 2, InkColour)
	dst.Paste(src, 8, 8)
	if b, _ := dst.InkAt(9, 9); !b {
		t.Error("paste overwrote destination ink with white")
	}
	if _, col := dst.InkAt(10, 10); !col {
		t.Error("colour ink not pasted")
	}
}

func TestZeroSizedBoxesDrawNothing(t *testing.T) {
	c := NewCanvas(50, 50, FontDefault, 14)
	c.Write(10, 10, 0, 20, "x", TextOpts{})
	c.Write(10, 10, 20, -5, "x", TextOpts{})
	c.Icon(10, 10, -3, 20, "\uf00d", FontWeather, InkBlack, 0.9)
	if inkIn(c, 0, 0, 50, 50, false) != 0 {
		t.Error("drawing into an empty box produced ink")
	}
}
```

- [ ] **Step 3: Tests laufen lassen – müssen fehlschlagen**

Run: `go test ./internal/render/`
Expected: FAIL – `undefined: NewCanvas`, `undefined: FontDefault` …

- [ ] **Step 4: Implementieren**

`internal/render/fonts.go`:

```go
package render

import (
	"embed"
	"fmt"
	"sync"

	"golang.org/x/image/font/opentype"
)

// FontID selects one of the embedded fonts.
type FontID int

const (
	FontDefault  FontID = iota // NotoSans SemiCondensed Medium (Inkycal's default)
	FontRegular                // NotoSans SemiCondensed
	FontWeather                // Weather Icons
	FontMaterial               // Material Icons
)

//go:embed fonts/*.ttf
var fontFS embed.FS

var fontFiles = map[FontID]string{
	FontDefault:  "fonts/NotoSans-SemiCondensedMedium.ttf",
	FontRegular:  "fonts/NotoSans-SemiCondensed.ttf",
	FontWeather:  "fonts/weathericons-regular-webfont.ttf",
	FontMaterial: "fonts/MaterialIcons.ttf",
}

var (
	fontMu    sync.Mutex
	fontCache = map[FontID]*opentype.Font{}
)

// loadFont parses an embedded font once. Parsed fonts are safe for concurrent use; faces are not.
func loadFont(id FontID) *opentype.Font {
	fontMu.Lock()
	defer fontMu.Unlock()
	if f, ok := fontCache[id]; ok {
		return f
	}
	data, err := fontFS.ReadFile(fontFiles[id])
	if err != nil {
		panic(fmt.Sprintf("embedded font %d missing: %v", id, err))
	}
	f, err := opentype.Parse(data)
	if err != nil {
		panic(fmt.Sprintf("embedded font %d broken: %v", id, err))
	}
	fontCache[id] = f
	return f
}
```

`internal/render/canvas.go`:

```go
// Package render draws Inkycal module images on a two-layer (black + colour) canvas.
package render

import (
	"image"
	"image/color"

	"golang.org/x/image/font"
	"golang.org/x/image/font/opentype"
)

// InkThreshold: a pixel value <= InkThreshold counts as ink (same as Python's _optimize_im).
const InkThreshold = 220

// Ink selects the layer(s) a drawing operation affects.
type Ink int

const (
	InkBlack  Ink = iota // black layer only
	InkColour            // colour layer only (shown red)
	InkWhite             // white on both layers
)

type faceKey struct {
	id   FontID
	size int
}

// Canvas holds two greyscale layers; 255 is white (no ink).
type Canvas struct {
	W, H   int
	Black  *image.Gray
	Colour *image.Gray

	fontID   FontID
	fontSize int
	faces    map[faceKey]font.Face
}

// NewCanvas returns a white canvas using the given font as current font.
func NewCanvas(w, h int, id FontID, size int) *Canvas {
	c := &Canvas{
		W: w, H: h,
		Black:  image.NewGray(image.Rect(0, 0, w, h)),
		Colour: image.NewGray(image.Rect(0, 0, w, h)),
		fontID: id, fontSize: size,
		faces: map[faceKey]font.Face{},
	}
	for i := range c.Black.Pix {
		c.Black.Pix[i] = 255
		c.Colour.Pix[i] = 255
	}
	return c
}

// SetFont changes the current font.
func (c *Canvas) SetFont(id FontID, size int) { c.fontID, c.fontSize = id, size }

// Font returns the current font.
func (c *Canvas) Font() (FontID, int) { return c.fontID, c.fontSize }

// face returns a cached face; faces are per canvas because they are not goroutine-safe.
func (c *Canvas) face(id FontID, size int) font.Face {
	k := faceKey{id, size}
	if f, ok := c.faces[k]; ok {
		return f
	}
	f, err := opentype.NewFace(loadFont(id), &opentype.FaceOptions{Size: float64(size), DPI: 72, Hinting: font.HintingNone})
	if err != nil {
		panic(err) // only fails for invalid options
	}
	c.faces[k] = f
	return f
}

// layers returns the images an ink draws on and the colour to draw with.
func (c *Canvas) layers(ink Ink) ([]*image.Gray, color.Gray) {
	switch ink {
	case InkColour:
		return []*image.Gray{c.Colour}, color.Gray{Y: 0}
	case InkWhite:
		return []*image.Gray{c.Black, c.Colour}, color.Gray{Y: 255}
	default:
		return []*image.Gray{c.Black}, color.Gray{Y: 0}
	}
}

func (c *Canvas) set(x, y int, ink Ink) {
	if x < 0 || y < 0 || x >= c.W || y >= c.H {
		return
	}
	imgs, col := c.layers(ink)
	for _, img := range imgs {
		img.SetGray(x, y, col)
	}
}

// InkAt reports whether the pixel carries ink on the black and/or colour layer.
func (c *Canvas) InkAt(x, y int) (black, colour bool) {
	return c.Black.GrayAt(x, y).Y <= InkThreshold, c.Colour.GrayAt(x, y).Y <= InkThreshold
}

// Paste copies the ink (not the white background) of src to (x, y).
func (c *Canvas) Paste(src *Canvas, x, y int) {
	for sy := 0; sy < src.H; sy++ {
		for sx := 0; sx < src.W; sx++ {
			dx, dy := x+sx, y+sy
			if dx < 0 || dy < 0 || dx >= c.W || dy >= c.H {
				continue
			}
			if v := src.Black.GrayAt(sx, sy).Y; v <= InkThreshold {
				c.Black.SetGray(dx, dy, color.Gray{Y: v})
			}
			if v := src.Colour.GrayAt(sx, sy).Y; v <= InkThreshold {
				c.Colour.SetGray(dx, dy, color.Gray{Y: v})
			}
		}
	}
}
```

`internal/render/text.go`:

```go
package render

import (
	"image"
	"strings"

	"golang.org/x/image/font"
	"golang.org/x/image/math/fixed"
)

// Align is the horizontal text alignment inside a box.
type Align int

const (
	AlignCenter Align = iota
	AlignLeft
	AlignRight
)

// TextOpts configures Write.
type TextOpts struct {
	Align      Align
	Ink        Ink
	Autofit    bool    // pick the size whose "Ag" height is ~FillHeight*h (does not change the canvas font)
	FillHeight float64 // 0 means 0.8
}

// LineHeight is ascent+descent of the current font.
func (c *Canvas) LineHeight() int { return lineHeight(c.face(c.fontID, c.fontSize)) }

// TextWidth is the advance width of s in the current font.
func (c *Canvas) TextWidth(s string) int {
	return font.MeasureString(c.face(c.fontID, c.fontSize), s).Ceil()
}

// Wrap splits text on spaces into lines no wider than maxWidth (a single over-long word stays on its own line).
func (c *Canvas) Wrap(text string, maxWidth int) []string {
	return wrap(c.face(c.fontID, c.fontSize), text, maxWidth)
}

func lineHeight(f font.Face) int {
	m := f.Metrics()
	return (m.Ascent + m.Descent).Ceil()
}

func wrap(f font.Face, text string, maxWidth int) []string {
	var lines []string
	cur := ""
	for _, word := range strings.Split(text, " ") {
		proposed := strings.TrimSpace(cur + " " + word)
		if font.MeasureString(f, proposed).Ceil() <= maxWidth {
			cur = proposed
			continue
		}
		if cur == "" {
			lines = append(lines, word)
		} else {
			lines = append(lines, cur)
			cur = word
		}
	}
	if cur != "" {
		lines = append(lines, cur)
	}
	return lines
}

// fitSize returns the largest size (>= 8) whose "Ag" ink height stays below target.
func (c *Canvas) fitSize(id FontID, target int) int {
	for size := 8; size < 400; size++ {
		b, _ := font.BoundString(c.face(id, size), "Ag")
		if (b.Max.Y - b.Min.Y).Ceil() >= target {
			if size > 8 {
				return size - 1
			}
			return 8
		}
	}
	return 400
}

// Write draws (multi-line, wrapped) text centred vertically in the box (x, y, w, h).
// Lines that do not fit are dropped from the bottom, but the first line is always drawn (clipped to the box).
func (c *Canvas) Write(x, y, w, h int, text string, o TextOpts) {
	if w <= 0 || h <= 0 {
		return
	}
	fill := o.FillHeight
	if fill == 0 {
		fill = 0.8
	}
	size := c.fontSize
	if o.Autofit {
		size = c.fitSize(c.fontID, int(float64(h)*fill))
	}
	face := c.face(c.fontID, size)

	var lines []string
	for _, logical := range strings.Split(text, "\n") {
		lines = append(lines, wrap(face, logical, w)...)
	}
	if len(lines) == 0 {
		return
	}
	lh := lineHeight(face)
	spacing := int(float64(size) * 0.2)
	n := len(lines)
	for n > 1 && n*lh+(n-1)*spacing > h {
		n--
	}
	lines = lines[:n]
	total := n*lh + (n-1)*spacing
	top := y + (h-total)/2
	ascent := face.Metrics().Ascent.Ceil()

	clip := image.Rect(x, y, x+w, y+h)
	imgs, col := c.layers(o.Ink)
	for i, line := range lines {
		lw := font.MeasureString(face, line).Ceil()
		px := x + (w-lw)/2
		switch o.Align {
		case AlignLeft:
			px = x
		case AlignRight:
			px = x + w - lw
		}
		baseline := top + i*(lh+spacing) + ascent
		for _, img := range imgs {
			d := font.Drawer{
				Dst:  img.SubImage(clip).(*image.Gray),
				Src:  image.NewUniform(col),
				Face: face,
				Dot:  fixed.P(px, baseline),
			}
			d.DrawString(line)
		}
	}
}

// Icon draws a single glyph scaled so its ink fills fillRatio of the box, centred by its ink bounds.
func (c *Canvas) Icon(x, y, w, h int, glyph string, id FontID, ink Ink, fillRatio float64) {
	if w <= 0 || h <= 0 {
		return
	}
	if fillRatio == 0 {
		fillRatio = 0.9
	}
	size := 8
	for ; size < 400; size++ {
		b, _ := font.BoundString(c.face(id, size), glyph)
		bw, bh := (b.Max.X - b.Min.X).Ceil(), (b.Max.Y - b.Min.Y).Ceil()
		if float64(bw) >= float64(w)*fillRatio || float64(bh) >= float64(h)*fillRatio {
			if size > 8 {
				size--
			}
			break
		}
	}
	face := c.face(id, size)
	b, _ := font.BoundString(face, glyph)
	bw, bh := (b.Max.X - b.Min.X).Ceil(), (b.Max.Y - b.Min.Y).Ceil()
	left := x + (w-bw)/2
	top := y + (h-bh)/2
	dot := fixed.Point26_6{X: fixed.I(left) - b.Min.X, Y: fixed.I(top) - b.Min.Y}
	imgs, col := c.layers(ink)
	for _, img := range imgs {
		d := font.Drawer{Dst: img, Src: image.NewUniform(col), Face: face, Dot: dot}
		d.DrawString(glyph)
	}
}
```

`internal/render/shapes.go`:

```go
package render

import "math"

// HLine draws a 1px horizontal line from x0 to x1 (inclusive).
func (c *Canvas) HLine(x0, x1, y int, ink Ink) {
	if x0 > x1 {
		x0, x1 = x1, x0
	}
	for x := x0; x <= x1; x++ {
		c.set(x, y, ink)
	}
}

// VLine draws a 1px vertical line from y0 to y1 (inclusive).
func (c *Canvas) VLine(x, y0, y1 int, ink Ink) {
	if y0 > y1 {
		y0, y1 = y1, y0
	}
	for y := y0; y <= y1; y++ {
		c.set(x, y, ink)
	}
}

// RoundedRect draws a 1px outline covering pixels x..x+w-1, y..y+h-1 with corner radius r.
func (c *Canvas) RoundedRect(x, y, w, h, r int, ink Ink) {
	if w <= 0 || h <= 0 {
		return
	}
	if max := min(w, h) / 2; r > max {
		r = max
	}
	x1, y1 := x+w-1, y+h-1
	c.HLine(x+r, x1-r, y, ink)
	c.HLine(x+r, x1-r, y1, ink)
	c.VLine(x, y+r, y1-r, ink)
	c.VLine(x1, y+r, y1-r, ink)
	if r == 0 {
		return
	}
	corners := [4][2]int{{x + r, y + r}, {x1 - r, y + r}, {x + r, y1 - r}, {x1 - r, y1 - r}}
	for i, cc := range corners {
		for dy := 0; dy <= r; dy++ {
			for dx := 0; dx <= r; dx++ {
				if math.Abs(math.Hypot(float64(dx), float64(dy))-float64(r)) > 0.5 {
					continue
				}
				px, py := cc[0]-dx, cc[1]-dy
				if i == 1 || i == 3 {
					px = cc[0] + dx
				}
				if i >= 2 {
					py = cc[1] + dy
				}
				c.set(px, py, ink)
			}
		}
	}
}

// FillCircle fills a disc of radius r around (cx, cy).
func (c *Canvas) FillCircle(cx, cy, r int, ink Ink) {
	for dy := -r; dy <= r; dy++ {
		for dx := -r; dx <= r; dx++ {
			if dx*dx+dy*dy <= r*r {
				c.set(cx+dx, cy+dy, ink)
			}
		}
	}
}

// Border is Python's draw_border: the box (w, h) shrunk by shrinkX/shrinkY, centred, with rounded corners.
func Border(c *Canvas, x, y, w, h, r int, shrinkX, shrinkY float64, ink Ink) {
	bw := int(float64(w) * (1 - shrinkX))
	bh := int(float64(h) * (1 - shrinkY))
	c.RoundedRect(x+(w-bw)/2, y+(h-bh)/2, bw+1, bh+1, r, ink)
}
```

- [ ] **Step 5: Tests laufen lassen**

Run: `go mod tidy && go test ./internal/render/`
Expected: `ok  	inkycal/internal/render`

- [ ] **Step 6: Commit**

```bash
git add go.mod go.sum internal/render
git commit -m "feat(render): two-layer canvas with text, icons and shapes"
```

---

### Task 4: `render` – Zusammensetzen, Packen, Hash, Vorschau

**Files:**
- Create: `internal/render/compose.go`, `internal/render/pack.go`
- Test: `internal/render/pack_test.go`

**Interfaces:**
- Consumes: `Canvas`, `NewCanvas`, `InkAt`, `Paste`, `RoundedRect`, `Write`, `set` (Task 3)
- Produces: `render.Part{Img *Canvas; SectionW, SectionH int}`, `render.Assemble(w, h int, parts []Part, border bool, info string, infoHeight int) *Canvas`, `render.Frame{W, H int; Black, Red []byte}` (Black: 1 = weiß; Red: 1 = rot; MSB zuerst, zeilenweise), `render.Pack(c *Canvas, panelW, panelH, orientation int) (Frame, error)`, `Frame.Hash() string`, `Frame.Preview() *image.RGBA`. Ein Hochformat-Canvas (`panelH × panelW`) wird 90° gegen den Uhrzeigersinn gedreht (wie Pythons `getbuffer`), `orientation == 180` dreht vorher um 180°.

- [ ] **Step 1: Failing Tests schreiben**

`internal/render/pack_test.go`:

```go
package render

import (
	"image/color"
	"testing"
)

func bitSet(plane []byte, w, x, y int) bool {
	stride := (w + 7) / 8
	return plane[y*stride+x/8]&(0x80>>(x%8)) != 0
}

func TestPackRotatesPortraitCounterClockwise(t *testing.T) {
	c := NewCanvas(4, 8, FontDefault, 14) // portrait for an 8x4 panel
	c.set(3, 0, InkBlack)                 // top-right of portrait → top-left of panel
	c.set(0, 0, InkColour)                // top-left of portrait → bottom-left of panel
	f, err := Pack(c, 8, 4, 0)
	if err != nil {
		t.Fatal(err)
	}
	if bitSet(f.Black, 8, 0, 0) {
		t.Error("expected black (bit 0) at panel (0,0)")
	}
	if !bitSet(f.Red, 8, 0, 3) {
		t.Error("expected red at panel (0,3)")
	}
	if !bitSet(f.Black, 8, 7, 3) || bitSet(f.Red, 8, 7, 3) {
		t.Error("untouched pixel must be white")
	}
}

func TestPackOrientation180(t *testing.T) {
	c := NewCanvas(8, 4, FontDefault, 14) // already landscape
	c.set(0, 0, InkBlack)
	f, err := Pack(c, 8, 4, 180)
	if err != nil {
		t.Fatal(err)
	}
	if bitSet(f.Black, 8, 7, 3) {
		t.Error("expected black at (7,3) after 180° flip")
	}
	if !bitSet(f.Black, 8, 0, 0) {
		t.Error("(0,0) must be white after flip")
	}
}

func TestPackRejectsWrongSize(t *testing.T) {
	if _, err := Pack(NewCanvas(5, 5, FontDefault, 14), 8, 4, 0); err == nil {
		t.Fatal("expected error")
	}
}

func TestHashChangesWithContent(t *testing.T) {
	c := NewCanvas(8, 4, FontDefault, 14)
	a, _ := Pack(c, 8, 4, 0)
	b, _ := Pack(c, 8, 4, 0)
	if a.Hash() != b.Hash() {
		t.Fatal("same content, different hash")
	}
	c.set(1, 1, InkColour)
	d, _ := Pack(c, 8, 4, 0)
	if a.Hash() == d.Hash() {
		t.Fatal("different content, same hash")
	}
}

func TestPreviewColours(t *testing.T) {
	c := NewCanvas(8, 4, FontDefault, 14)
	c.set(1, 1, InkBlack)
	c.set(2, 1, InkColour)
	f, _ := Pack(c, 8, 4, 0)
	p := f.Preview()
	if p.RGBAAt(1, 1) != (color.RGBA{0, 0, 0, 255}) || p.RGBAAt(2, 1) != (color.RGBA{255, 0, 0, 255}) || p.RGBAAt(0, 0) != (color.RGBA{255, 255, 255, 255}) {
		t.Error("preview colours wrong")
	}
}

func TestAssembleStacksAndCentres(t *testing.T) {
	top := NewCanvas(80, 20, FontDefault, 14)
	top.FillCircle(40, 10, 3, InkBlack)
	bottom := NewCanvas(80, 20, FontDefault, 14)
	bottom.FillCircle(40, 10, 3, InkColour)
	out := Assemble(100, 200, []Part{{top, 100, 40}, {bottom, 100, 40}}, false, "", 0)
	if b, _ := out.InkAt(50, 20); !b { // x: (100-80)/2+40, y: (40-20)/2+10
		t.Error("first module not centred in its section")
	}
	if _, col := out.InkAt(50, 60); !col { // y: 40 + 10 + 10
		t.Error("second module not placed below the first")
	}
}

func TestAssembleBorderAndInfo(t *testing.T) {
	m := NewCanvas(100, 50, FontDefault, 14)
	out := Assemble(100, 120, []Part{{m, 100, 50}}, true, "Battery: 80%", 30)
	if b, _ := out.InkAt(50, 1); !b {
		t.Error("border missing")
	}
	if inkIn(out, 0, 90, 100, 120, false) == 0 {
		t.Error("info text missing")
	}
	if inkIn(out, 0, 55, 100, 90, false) != 0 {
		t.Error("unexpected ink between module and info")
	}
}
```

- [ ] **Step 2: Tests laufen lassen – müssen fehlschlagen**

Run: `go test ./internal/render/`
Expected: FAIL – `undefined: Pack`, `undefined: Assemble`, `undefined: Part`

- [ ] **Step 3: Implementieren**

`internal/render/compose.go`:

```go
package render

// Part is one module image placed in its section (sections are stacked top to bottom).
type Part struct {
	Img                *Canvas
	SectionW, SectionH int
}

// Assemble stacks the parts vertically, centring each image in its section, and writes the
// info line into the bottom infoHeight pixels (skipped when infoHeight is 0).
func Assemble(w, h int, parts []Part, border bool, info string, infoHeight int) *Canvas {
	out := NewCanvas(w, h, FontDefault, 14)
	cursor := 0
	for _, p := range parts {
		if border {
			p.Img.RoundedRect(1, 1, p.Img.W-2, p.Img.H-2, 5, InkBlack)
		}
		x := (p.SectionW - p.Img.W) / 2
		y := cursor + (p.SectionH-p.Img.H)/2
		out.Paste(p.Img, x, y)
		cursor += p.SectionH
	}
	if infoHeight > 0 && info != "" {
		out.Write(0, h-infoHeight, w, infoHeight, info, TextOpts{})
	}
	return out
}
```

`internal/render/pack.go`:

```go
package render

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"image"
	"image/color"
)

// Frame is the display-native bit-plane representation of a canvas.
type Frame struct {
	W, H  int    // native panel size (landscape)
	Black []byte // 1 bit/px, MSB first, 1 = white, 0 = black
	Red   []byte // 1 bit/px, MSB first, 1 = red, 0 = no red
}

// Pack converts the canvas to panel bit planes. A portrait canvas (panelH x panelW) is rotated
// 90° counter-clockwise like Python's getbuffer; orientation 180 flips the canvas first.
func Pack(c *Canvas, panelW, panelH, orientation int) (Frame, error) {
	var rotate bool
	switch {
	case c.W == panelW && c.H == panelH:
	case c.W == panelH && c.H == panelW:
		rotate = true
	default:
		return Frame{}, fmt.Errorf("canvas %dx%d does not fit panel %dx%d", c.W, c.H, panelW, panelH)
	}
	stride := (panelW + 7) / 8
	f := Frame{W: panelW, H: panelH, Black: make([]byte, stride*panelH), Red: make([]byte, stride*panelH)}
	for i := range f.Black {
		f.Black[i] = 0xFF
	}
	for dy := 0; dy < panelH; dy++ {
		for dx := 0; dx < panelW; dx++ {
			sx, sy := dx, dy
			if rotate {
				sx, sy = c.W-1-dy, dx
			}
			if orientation == 180 {
				sx, sy = c.W-1-sx, c.H-1-sy
			}
			black, colour := c.InkAt(sx, sy)
			idx, bit := dy*stride+dx/8, byte(0x80>>(dx%8))
			if black {
				f.Black[idx] &^= bit
			}
			if colour {
				f.Red[idx] |= bit
			}
		}
	}
	return f, nil
}

// Hash identifies the frame content; equal hashes mean no refresh is needed.
func (f Frame) Hash() string {
	h := sha256.New()
	h.Write(f.Black)
	h.Write(f.Red)
	return hex.EncodeToString(h.Sum(nil))
}

// Preview renders the frame as the panel shows it (landscape): white, black and red.
func (f Frame) Preview() *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, f.W, f.H))
	stride := (f.W + 7) / 8
	for y := 0; y < f.H; y++ {
		for x := 0; x < f.W; x++ {
			idx, bit := y*stride+x/8, byte(0x80>>(x%8))
			col := color.RGBA{255, 255, 255, 255}
			if f.Black[idx]&bit == 0 {
				col = color.RGBA{0, 0, 0, 255}
			}
			if f.Red[idx]&bit != 0 {
				col = color.RGBA{255, 0, 0, 255}
			}
			img.SetRGBA(x, y, col)
		}
	}
	return img
}
```

- [ ] **Step 4: Tests laufen lassen**

Run: `go test ./internal/render/`
Expected: `ok  	inkycal/internal/render`

- [ ] **Step 5: Commit**

```bash
git add internal/render
git commit -m "feat(render): assemble sections, pack panel bit planes, hash, preview"
```

---

### Task 5: `epd` – Display-Interface, Treiber 7,5″ V2 Farbe, PNG-Display

**Files:**
- Create: `internal/epd/epd.go`, `internal/epd/epd7in5bv2.go`
- Test: `internal/epd/epd7in5bv2_test.go`

**Interfaces:**
- Consumes: `render.Frame`, `render.Pack`, `render.NewCanvas` (Task 3/4)
- Produces: `epd.Display` (`Size() (w, h int)`, `Show(f render.Frame) error`, `Clear(cycles int) error`), `epd.Bus` (`Open`, `Close`, `SetReset(high bool)`, `Command(c byte)`, `Data(p ...byte)`, `BusyLevel() (int, error)`), `epd.Models map[string][2]int`, `epd.NewPNG(path string, w, h int) Display`, `epd.NewEPD7in5bV2(bus Bus, sleep func(time.Duration)) *EPD7in5bV2` (Feld `BusyTimeout`). Befehlsfolge 1:1 aus Waveshares `epd7in5b_V2.py` (Inkycal `epd_7_in_5_v2_colour.py`); BUSY low = beschäftigt; Polling alle 10 ms; nach jedem Fehler wird der Bus geschlossen (Panel stromlos).

- [ ] **Step 1: Failing Tests schreiben**

`internal/epd/epd7in5bv2_test.go`:

```go
package epd

import (
	"fmt"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"inkycal/internal/render"
)

type fakeBus struct {
	ops        []string
	busyPolls  int // BusyLevel returns 0 this many times, then 1
	data       map[byte][]byte
	lastCmd    byte
	closed     int
	alwaysBusy bool
}

func (b *fakeBus) Open() error  { b.ops = append(b.ops, "open"); return nil }
func (b *fakeBus) Close() error { b.ops = append(b.ops, "close"); b.closed++; return nil }
func (b *fakeBus) SetReset(h bool) error {
	b.ops = append(b.ops, fmt.Sprintf("rst:%v", h))
	return nil
}
func (b *fakeBus) Command(c byte) error {
	b.lastCmd = c
	if c != 0x71 {
		b.ops = append(b.ops, fmt.Sprintf("C%02X", c))
	}
	return nil
}
func (b *fakeBus) Data(p ...byte) error {
	if b.data == nil {
		b.data = map[byte][]byte{}
	}
	b.data[b.lastCmd] = append(b.data[b.lastCmd], p...)
	if len(p) <= 4 {
		b.ops = append(b.ops, fmt.Sprintf("D% X", p))
	} else {
		b.ops = append(b.ops, fmt.Sprintf("D[%d]", len(p)))
	}
	return nil
}
func (b *fakeBus) BusyLevel() (int, error) {
	if b.alwaysBusy {
		return 0, nil
	}
	if b.busyPolls > 0 {
		b.busyPolls--
		return 0, nil
	}
	return 1, nil
}

func noSleep(time.Duration) {}

func TestShowSequence(t *testing.T) {
	bus := &fakeBus{busyPolls: 3}
	d := NewEPD7in5bV2(bus, noSleep)
	c := render.NewCanvas(480, 800, render.FontDefault, 14)
	c.FillCircle(240, 400, 50, render.InkColour)
	f, err := render.Pack(c, 800, 480, 0)
	if err != nil {
		t.Fatal(err)
	}
	if err := d.Show(f); err != nil {
		t.Fatal(err)
	}
	got := strings.Join(bus.ops, " ")
	want := "open rst:true rst:false rst:true C01 D07 07 3F 3F C04 C00 D0F C61 D03 20 01 E0 C15 D00 C50 D11 07 C60 D22 C65 D00 00 00 00 C10 D[48000] C13 D[48000] C12 C02 C07 DA5 close"
	if got != want {
		t.Errorf("ops:\n got %s\nwant %s", got, want)
	}
	if string(bus.data[0x10]) != string(f.Black) || string(bus.data[0x13]) != string(f.Red) {
		t.Error("planes not transferred unchanged")
	}
}

func TestShowRejectsWrongFrame(t *testing.T) {
	d := NewEPD7in5bV2(&fakeBus{}, noSleep)
	if err := d.Show(render.Frame{W: 10, H: 10}); err == nil {
		t.Fatal("expected error")
	}
}

func TestBusyTimeoutClosesBus(t *testing.T) {
	bus := &fakeBus{alwaysBusy: true}
	d := NewEPD7in5bV2(bus, noSleep)
	d.BusyTimeout = 50 * time.Millisecond
	f := render.Frame{W: 800, H: 480, Black: make([]byte, 48000), Red: make([]byte, 48000)}
	if err := d.Show(f); err == nil {
		t.Fatal("expected busy timeout")
	}
	if bus.closed == 0 {
		t.Fatal("bus must be closed after an error so the panel is unpowered")
	}
}

func TestClearCycles(t *testing.T) {
	bus := &fakeBus{}
	d := NewEPD7in5bV2(bus, noSleep)
	if err := d.Clear(2); err != nil {
		t.Fatal(err)
	}
	refreshes := strings.Count(strings.Join(bus.ops, " "), "C12")
	if refreshes != 6 {
		t.Errorf("expected 6 refreshes for 2 cycles, got %d", refreshes)
	}
}

func TestPNGDisplay(t *testing.T) {
	path := filepath.Join(t.TempDir(), "preview.png")
	d := NewPNG(path, 800, 480)
	c := render.NewCanvas(480, 800, render.FontDefault, 14)
	f, _ := render.Pack(c, 800, 480, 0)
	if err := d.Show(f); err != nil {
		t.Fatal(err)
	}
	fh, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer fh.Close()
	img, err := png.Decode(fh)
	if err != nil {
		t.Fatal(err)
	}
	if b := img.Bounds(); b.Dx() != 800 || b.Dy() != 480 {
		t.Errorf("png size %v", b)
	}
}
```

- [ ] **Step 2: Tests laufen lassen – müssen fehlschlagen**

Run: `go test ./internal/epd/`
Expected: FAIL – `undefined: NewEPD7in5bV2`, `undefined: NewPNG`

- [ ] **Step 3: Implementieren**

`internal/epd/epd.go`:

```go
// Package epd drives e-paper panels (and a PNG stand-in for dry runs).
package epd

import (
	"fmt"
	"image/png"
	"os"
	"path/filepath"

	"inkycal/internal/render"
)

// Display shows frames on a panel.
type Display interface {
	Size() (w, h int)          // native panel size (landscape)
	Show(f render.Frame) error // init, transfer, refresh, deep sleep
	Clear(cycles int) error    // calibration: black, colour, white per cycle
}

// Models lists supported panels and their native size.
var Models = map[string][2]int{
	"epd_7_in_5_v2_colour": {800, 480},
}

// Bus is the hardware interface a driver needs (SPI + 4 GPIOs).
type Bus interface {
	Open() error              // power the panel, open SPI
	Close() error             // close SPI, pull RST/DC/PWR low (panel draws no power)
	SetReset(high bool) error // RST pin
	Command(c byte) error     // DC low, write one byte
	Data(p ...byte) error     // DC high, write bytes
	BusyLevel() (int, error)  // raw level of the BUSY pin
}

type pngDisplay struct {
	path string
	w, h int
}

// NewPNG returns a Display that writes each frame as PNG (as the panel shows it, landscape).
func NewPNG(path string, w, h int) Display { return &pngDisplay{path: path, w: w, h: h} }

func (p *pngDisplay) Size() (int, int) { return p.w, p.h }

func (p *pngDisplay) Clear(int) error { return nil }

func (p *pngDisplay) Show(f render.Frame) error {
	if f.W != p.w || f.H != p.h {
		return fmt.Errorf("frame %dx%d, display %dx%d", f.W, f.H, p.w, p.h)
	}
	tmp, err := os.CreateTemp(filepath.Dir(p.path), ".preview-*.png")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if err := png.Encode(tmp, f.Preview()); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), p.path)
}
```

`internal/epd/epd7in5bv2.go`:

```go
package epd

import (
	"errors"
	"fmt"
	"time"

	"inkycal/internal/render"
)

// EPD7in5bV2 drives the Waveshare 7.5" V2 black/white/red panel (Python: epd_7_in_5_v2_colour).
type EPD7in5bV2 struct {
	bus         Bus
	sleep       func(time.Duration)
	BusyTimeout time.Duration // give up waiting for the panel after this long
}

// NewEPD7in5bV2 creates the driver; sleep is time.Sleep in production and a fake in tests.
func NewEPD7in5bV2(bus Bus, sleep func(time.Duration)) *EPD7in5bV2 {
	return &EPD7in5bV2{bus: bus, sleep: sleep, BusyTimeout: 60 * time.Second}
}

func (e *EPD7in5bV2) Size() (int, int) { return 800, 480 }

// waitBusy polls the panel status (BUSY low = busy) every 10 ms.
func (e *EPD7in5bV2) waitBusy() error {
	const step = 10 * time.Millisecond
	for waited := time.Duration(0); ; waited += step {
		if err := e.bus.Command(0x71); err != nil {
			return err
		}
		level, err := e.bus.BusyLevel()
		if err != nil {
			return err
		}
		if level == 1 {
			break
		}
		if waited >= e.BusyTimeout {
			return errors.New("epd: panel stays busy, check wiring and power")
		}
		e.sleep(step)
	}
	e.sleep(200 * time.Millisecond)
	return nil
}

func (e *EPD7in5bV2) cmd(c byte, data ...byte) error {
	if err := e.bus.Command(c); err != nil {
		return err
	}
	if len(data) == 0 {
		return nil
	}
	return e.bus.Data(data...)
}

func (e *EPD7in5bV2) init() error {
	if err := e.bus.Open(); err != nil {
		return err
	}
	for _, step := range []struct {
		high bool
		wait time.Duration
	}{{true, 200 * time.Millisecond}, {false, 4 * time.Millisecond}, {true, 200 * time.Millisecond}} {
		if err := e.bus.SetReset(step.high); err != nil {
			return err
		}
		e.sleep(step.wait)
	}
	if err := e.cmd(0x01, 0x07, 0x07, 0x3f, 0x3f); err != nil { // power setting
		return err
	}
	if err := e.cmd(0x04); err != nil { // power on
		return err
	}
	e.sleep(100 * time.Millisecond)
	if err := e.waitBusy(); err != nil {
		return err
	}
	seq := []struct {
		c    byte
		data []byte
	}{
		{0x00, []byte{0x0F}},                   // panel setting
		{0x61, []byte{0x03, 0x20, 0x01, 0xE0}}, // resolution 800x480
		{0x15, []byte{0x00}},
		{0x50, []byte{0x11, 0x07}}, // VCOM and data interval
		{0x60, []byte{0x22}},       // TCON
		{0x65, []byte{0x00, 0x00, 0x00, 0x00}},
	}
	for _, s := range seq {
		if err := e.cmd(s.c, s.data...); err != nil {
			return err
		}
	}
	return nil
}

func (e *EPD7in5bV2) refresh(black, red []byte) error {
	if err := e.cmd(0x10, black...); err != nil {
		return err
	}
	if err := e.cmd(0x13, red...); err != nil {
		return err
	}
	if err := e.cmd(0x12); err != nil {
		return err
	}
	e.sleep(100 * time.Millisecond)
	return e.waitBusy()
}

func (e *EPD7in5bV2) deepSleep() error {
	if err := e.cmd(0x02); err != nil { // power off
		return err
	}
	if err := e.waitBusy(); err != nil {
		return err
	}
	if err := e.cmd(0x07, 0xA5); err != nil { // deep sleep
		return err
	}
	e.sleep(2 * time.Second)
	return e.bus.Close()
}

// run wraps a panel session so the bus is always closed (panel unpowered) on errors.
func (e *EPD7in5bV2) run(body func() error) error {
	if err := e.init(); err != nil {
		e.bus.Close()
		return err
	}
	if err := body(); err != nil {
		e.bus.Close()
		return err
	}
	return e.deepSleep()
}

// Show transfers the frame and refreshes the panel (~16-20 s).
func (e *EPD7in5bV2) Show(f render.Frame) error {
	if f.W != 800 || f.H != 480 {
		return fmt.Errorf("epd: frame %dx%d, panel is 800x480", f.W, f.H)
	}
	return e.run(func() error { return e.refresh(f.Black, f.Red) })
}

// Clear cycles black → red → white to remove ghosting.
func (e *EPD7in5bV2) Clear(cycles int) error {
	n := 800 / 8 * 480
	fill := func(b byte) []byte {
		p := make([]byte, n)
		for i := range p {
			p[i] = b
		}
		return p
	}
	allBlack, white, noRed, allRed := fill(0x00), fill(0xFF), fill(0x00), fill(0xFF)
	return e.run(func() error {
		for i := 0; i < cycles; i++ {
			for _, step := range [][2][]byte{{allBlack, noRed}, {white, allRed}, {white, noRed}} {
				if err := e.refresh(step[0], step[1]); err != nil {
					return err
				}
			}
		}
		return nil
	})
}
```

- [ ] **Step 4: Tests laufen lassen**

Run: `go test ./internal/epd/`
Expected: `ok  	inkycal/internal/epd`

- [ ] **Step 5: Commit**

```bash
git add internal/epd
git commit -m "feat(epd): 7.5in V2 colour driver and PNG display"
```

---

### Task 6: `epd` – Linux-Hardware-Bus (spidev + gpiocdev)

**Files:**
- Create: `internal/epd/bus_linux.go`, `internal/epd/bus_other.go`

**Interfaces:**
- Consumes: `epd.Bus`, `epd.Display`, `NewEPD7in5bV2` (Task 5)
- Produces: `epd.Open(model string) (Display, error)` – auf Linux echte Hardware für `epd_7_in_5_v2_colour`, sonst Fehler; auf anderen OS immer Fehler.

Dieser Code spricht echte Hardware an und hat keine Unit-Tests; er wird in Task 14 auf dem Pi verifiziert. Hier wird nur geprüft, dass er für beide Pi-Architekturen kompiliert.

- [ ] **Step 1: Abhängigkeiten holen**

```bash
go get github.com/warthog618/go-gpiocdev@v0.9.2 golang.org/x/sys@v0.49.0
```

- [ ] **Step 2: Implementieren**

`internal/epd/bus_linux.go`:

```go
//go:build linux

package epd

import (
	"fmt"
	"os"
	"time"
	"unsafe"

	"github.com/warthog618/go-gpiocdev"
	"golang.org/x/sys/unix"
)

// Waveshare HAT wiring (BCM numbering) and SPI settings, as in Python's epdconfig.py.
const (
	pinRST   = 17
	pinDC    = 25
	pinPWR   = 18
	pinBusy  = 24
	gpioChip = "gpiochip0"
	spiDev   = "/dev/spidev0.0"
	spiSpeed = 4_000_000

	spiIocWrMode        = 0x40016b01
	spiIocWrBitsPerWord = 0x40016b03
	spiIocWrMaxSpeedHz  = 0x40046b04
	spiChunk            = 4096 // spidev default bufsiz
)

type linuxBus struct {
	rst, dc, pwr, busy *gpiocdev.Line
	spi                *os.File
}

func newLinuxBus() (*linuxBus, error) {
	b := &linuxBus{}
	var err error
	if b.rst, err = gpiocdev.RequestLine(gpioChip, pinRST, gpiocdev.AsOutput(0)); err != nil {
		return nil, fmt.Errorf("gpio %d: %w", pinRST, err)
	}
	if b.dc, err = gpiocdev.RequestLine(gpioChip, pinDC, gpiocdev.AsOutput(0)); err != nil {
		return nil, fmt.Errorf("gpio %d: %w", pinDC, err)
	}
	if b.pwr, err = gpiocdev.RequestLine(gpioChip, pinPWR, gpiocdev.AsOutput(0)); err != nil {
		return nil, fmt.Errorf("gpio %d: %w", pinPWR, err)
	}
	if b.busy, err = gpiocdev.RequestLine(gpioChip, pinBusy, gpiocdev.AsInput, gpiocdev.WithPullDown); err != nil {
		return nil, fmt.Errorf("gpio %d: %w", pinBusy, err)
	}
	return b, nil
}

func ioctl(fd uintptr, req uintptr, val unsafe.Pointer) error {
	if _, _, errno := unix.Syscall(unix.SYS_IOCTL, fd, req, uintptr(val)); errno != 0 {
		return errno
	}
	return nil
}

func (b *linuxBus) Open() error {
	if err := b.pwr.SetValue(1); err != nil {
		return err
	}
	time.Sleep(10 * time.Millisecond) // let the HAT's supply settle
	f, err := os.OpenFile(spiDev, os.O_RDWR, 0)
	if err != nil {
		return fmt.Errorf("open %s (is SPI enabled?): %w", spiDev, err)
	}
	mode, bits, speed := uint8(0), uint8(8), uint32(spiSpeed)
	for _, s := range []struct {
		req uintptr
		val unsafe.Pointer
	}{{spiIocWrMode, unsafe.Pointer(&mode)}, {spiIocWrBitsPerWord, unsafe.Pointer(&bits)}, {spiIocWrMaxSpeedHz, unsafe.Pointer(&speed)}} {
		if err := ioctl(f.Fd(), s.req, s.val); err != nil {
			f.Close()
			return fmt.Errorf("spi ioctl %#x: %w", s.req, err)
		}
	}
	b.spi = f
	return nil
}

func (b *linuxBus) Close() error {
	if b.spi != nil {
		b.spi.Close()
		b.spi = nil
	}
	b.rst.SetValue(0)
	b.dc.SetValue(0)
	return b.pwr.SetValue(0)
}

func (b *linuxBus) SetReset(high bool) error {
	v := 0
	if high {
		v = 1
	}
	return b.rst.SetValue(v)
}

func (b *linuxBus) write(dc int, p []byte) error {
	if err := b.dc.SetValue(dc); err != nil {
		return err
	}
	for len(p) > 0 {
		n := min(len(p), spiChunk)
		if _, err := b.spi.Write(p[:n]); err != nil {
			return err
		}
		p = p[n:]
	}
	return nil
}

func (b *linuxBus) Command(c byte) error { return b.write(0, []byte{c}) }

func (b *linuxBus) Data(p ...byte) error { return b.write(1, p) }

func (b *linuxBus) BusyLevel() (int, error) { return b.busy.Value() }

// Open returns the hardware display for a model name from settings.json.
func Open(model string) (Display, error) {
	if model != "epd_7_in_5_v2_colour" {
		return nil, fmt.Errorf("display model %q not supported (supported: epd_7_in_5_v2_colour)", model)
	}
	bus, err := newLinuxBus()
	if err != nil {
		return nil, err
	}
	return NewEPD7in5bV2(bus, time.Sleep), nil
}
```

`internal/epd/bus_other.go`:

```go
//go:build !linux

package epd

import "errors"

// Open is only available on Linux; use NewPNG (dry-run) elsewhere.
func Open(model string) (Display, error) {
	return nil, errors.New("hardware displays are only supported on Linux")
}
```

- [ ] **Step 3: Cross-Compile prüfen**

Run:
```bash
go mod tidy
CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build ./...
CGO_ENABLED=0 GOOS=linux GOARCH=arm GOARM=7 go build ./...
GOOS=darwin go vet ./internal/epd/
go test ./internal/epd/
```
Expected: keine Ausgabe der Builds, `ok  	inkycal/internal/epd`

- [ ] **Step 4: Commit**

```bash
git add go.mod go.sum internal/epd
git commit -m "feat(epd): linux SPI/GPIO bus for the Waveshare HAT"
```

---

### Task 7: `fetch` – HTTP mit Retry und Cache-Fallback

**Files:**
- Create: `internal/fetch/fetch.go`
- Test: `internal/fetch/fetch_test.go`

**Interfaces:**
- Consumes: –
- Produces: `fetch.Client{HTTP *http.Client; CacheDir string; Attempts int; Backoff time.Duration}`, `fetch.New(cacheDir string) *Client` (`""` = ohne Cache; Verzeichnis wird bei Bedarf angelegt), `fetch.Result{Body []byte; Stale bool; FetchedAt time.Time}`, `(*Client).Get(ctx, url string, header http.Header) (Result, error)`, `(*Client).GetJSON(ctx, url string, header http.Header, v any) (stale bool, err error)`, `fetch.StatusError{URL string; Code int; Body string}`. Retry bei Netzwerkfehler/5xx/429, danach Cache; 4xx sofort als Fehler; Query-Strings (API-Keys) werden aus Fehlern/Logs entfernt.

- [ ] **Step 1: Failing Tests schreiben**

`internal/fetch/fetch_test.go`:

```go
package fetch

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func newTestClient(t *testing.T) *Client {
	c := New(t.TempDir())
	c.Backoff = time.Millisecond
	return c
}

func TestGetSuccessAndCacheFallback(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("hello"))
	}))
	c := newTestClient(t)
	res, err := c.Get(context.Background(), srv.URL+"/x", nil)
	if err != nil || string(res.Body) != "hello" || res.Stale {
		t.Fatalf("first get: %q %v %v", res.Body, res.Stale, err)
	}
	srv.Close() // network gone
	res, err = c.Get(context.Background(), srv.URL+"/x", nil)
	if err != nil || string(res.Body) != "hello" || !res.Stale {
		t.Fatalf("fallback: %q stale=%v err=%v", res.Body, res.Stale, err)
	}
}

func TestGetNoCacheReturnsError(t *testing.T) {
	srv := httptest.NewServer(http.NotFoundHandler())
	srv.Close()
	c := newTestClient(t)
	if _, err := c.Get(context.Background(), srv.URL, nil); err == nil {
		t.Fatal("expected error")
	}
}

func TestGet4xxIsNotRetriedAndIgnoresCache(t *testing.T) {
	var calls atomic.Int32
	status := http.StatusOK
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		w.WriteHeader(status)
		w.Write([]byte("body"))
	}))
	defer srv.Close()
	c := newTestClient(t)
	if _, err := c.Get(context.Background(), srv.URL, nil); err != nil {
		t.Fatal(err)
	}
	status = http.StatusUnauthorized
	calls.Store(0)
	_, err := c.Get(context.Background(), srv.URL, nil)
	var se *StatusError
	if !errors.As(err, &se) || se.Code != 401 {
		t.Fatalf("expected 401 StatusError, got %v", err)
	}
	if calls.Load() != 1 {
		t.Errorf("4xx retried: %d calls", calls.Load())
	}
}

func TestGetRetries5xx(t *testing.T) {
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if calls.Add(1) < 3 {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		w.Write([]byte("ok"))
	}))
	defer srv.Close()
	c := newTestClient(t)
	res, err := c.Get(context.Background(), srv.URL, nil)
	if err != nil || string(res.Body) != "ok" || calls.Load() != 3 {
		t.Fatalf("got %q err=%v calls=%d", res.Body, err, calls.Load())
	}
}

func TestHeadersSentAndCacheKeyedByAuth(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(r.Header.Get("Authorization")))
	}))
	c := newTestClient(t)
	a := http.Header{"Authorization": {"Bearer A"}}
	b := http.Header{"Authorization": {"Bearer B"}}
	c.Get(context.Background(), srv.URL, a)
	c.Get(context.Background(), srv.URL, b)
	srv.Close()
	res, _ := c.Get(context.Background(), srv.URL, a)
	if string(res.Body) != "Bearer A" {
		t.Errorf("cache mixed up tokens: %q", res.Body)
	}
}

func TestGetJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"temp": 21.5}`))
	}))
	defer srv.Close()
	var v struct {
		Temp float64 `json:"temp"`
	}
	stale, err := newTestClient(t).GetJSON(context.Background(), srv.URL, nil, &v)
	if err != nil || stale || v.Temp != 21.5 {
		t.Fatalf("%v %v %v", v, stale, err)
	}
}

func TestRedactHidesAPIKey(t *testing.T) {
	if got := redact("https://api.x/weather?appid=SECRET"); strings.Contains(got, "SECRET") {
		t.Errorf("key leaked: %s", got)
	}
}

func TestNetworkErrorDoesNotLeakKey(t *testing.T) {
	srv := httptest.NewServer(http.NotFoundHandler())
	srv.Close()
	c := New("")
	c.Attempts = 1
	_, err := c.Get(context.Background(), srv.URL+"/w?appid=SECRET", nil)
	if err == nil || strings.Contains(err.Error(), "SECRET") {
		t.Fatalf("error leaks key or is nil: %v", err)
	}
}
```

- [ ] **Step 2: Tests laufen lassen – müssen fehlschlagen**

Run: `go test ./internal/fetch/`
Expected: FAIL – `undefined: New`, `undefined: StatusError`, `undefined: redact`

- [ ] **Step 3: Implementieren**

`internal/fetch/fetch.go`:

```go
// Package fetch is the shared HTTP client: timeouts, retries and a disk cache used as fallback
// when the network fails, so the display shows slightly old data instead of an error.
package fetch

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	neturl "net/url"
	"os"
	"path/filepath"
	"time"
)

const maxBody = 20 << 20 // 20 MiB, big calendars included

type Client struct {
	HTTP     *http.Client
	CacheDir string        // "" disables the cache
	Attempts int           // tries per request
	Backoff  time.Duration // wait before the 2nd try, doubled each time
}

type Result struct {
	Body      []byte
	Stale     bool // served from cache because the request failed
	FetchedAt time.Time
}

// StatusError is returned for non-2xx responses.
type StatusError struct {
	URL  string
	Code int
	Body string
}

func (e *StatusError) Error() string {
	return fmt.Sprintf("GET %s: HTTP %d: %s", e.URL, e.Code, e.Body)
}

func (e *StatusError) retryable() bool { return e.Code >= 500 || e.Code == http.StatusTooManyRequests }

func New(cacheDir string) *Client {
	tr := http.DefaultTransport.(*http.Transport).Clone()
	tr.TLSHandshakeTimeout = 10 * time.Second
	tr.ResponseHeaderTimeout = 15 * time.Second
	return &Client{
		HTTP:     &http.Client{Timeout: 30 * time.Second, Transport: tr},
		CacheDir: cacheDir,
		Attempts: 3,
		Backoff:  500 * time.Millisecond,
	}
}

// Get fetches url. On network errors, 5xx or 429 it retries and finally falls back to the cache.
// 4xx errors (wrong key, wrong URL) are returned directly so misconfiguration stays visible.
func (c *Client) Get(ctx context.Context, url string, header http.Header) (Result, error) {
	key := cacheKey(url, header)
	body, err := c.fetch(ctx, url, header)
	if err == nil {
		c.store(key, body)
		return Result{Body: body, FetchedAt: time.Now()}, nil
	}
	var se *StatusError
	if errors.As(err, &se) && !se.retryable() {
		return Result{}, err
	}
	if cached, at, ok := c.load(key); ok {
		slog.Warn("using cached response", "url", redact(url), "cached_at", at, "err", err)
		return Result{Body: cached, Stale: true, FetchedAt: at}, nil
	}
	return Result{}, err
}

// GetJSON is Get followed by json.Unmarshal into v.
func (c *Client) GetJSON(ctx context.Context, url string, header http.Header, v any) (stale bool, err error) {
	res, err := c.Get(ctx, url, header)
	if err != nil {
		return false, err
	}
	if err := json.Unmarshal(res.Body, v); err != nil {
		return res.Stale, fmt.Errorf("GET %s: invalid JSON: %w", redact(url), err)
	}
	return res.Stale, nil
}

func (c *Client) fetch(ctx context.Context, url string, header http.Header) ([]byte, error) {
	attempts := max(c.Attempts, 1)
	wait := c.Backoff
	var lastErr error
	for i := 0; i < attempts; i++ {
		if i > 0 {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(wait):
			}
			wait *= 2
		}
		body, err := c.once(ctx, url, header)
		if err == nil {
			return body, nil
		}
		lastErr = err
		var se *StatusError
		if errors.As(err, &se) && !se.retryable() {
			return nil, err
		}
		if ctx.Err() != nil {
			return nil, err
		}
	}
	return nil, lastErr
}

func (c *Client) once(ctx context.Context, url string, header http.Header) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	for k, vs := range header {
		for _, v := range vs {
			req.Header.Add(k, v)
		}
	}
	req.Header.Set("User-Agent", "inkycal-go")
	resp, err := c.HTTP.Do(req)
	if err != nil {
		var ue *neturl.Error
		if errors.As(err, &ue) {
			err = fmt.Errorf("GET %s: %w", redact(url), ue.Err) // url.Error would log the API key
		}
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxBody))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		snippet := string(body)
		if len(snippet) > 200 {
			snippet = snippet[:200]
		}
		return nil, &StatusError{URL: redact(url), Code: resp.StatusCode, Body: snippet}
	}
	return body, nil
}

func cacheKey(url string, header http.Header) string {
	sum := sha256.Sum256([]byte(url + "\n" + header.Get("Authorization")))
	return hex.EncodeToString(sum[:])
}

func (c *Client) store(key string, body []byte) {
	if c.CacheDir == "" {
		return
	}
	if err := os.MkdirAll(c.CacheDir, 0o700); err != nil {
		slog.Warn("cache dir", "err", err)
		return
	}
	tmp, err := os.CreateTemp(c.CacheDir, ".tmp-*")
	if err != nil {
		slog.Warn("cache write", "err", err)
		return
	}
	_, werr := tmp.Write(body)
	cerr := tmp.Close()
	if werr != nil || cerr != nil {
		os.Remove(tmp.Name())
		return
	}
	if err := os.Rename(tmp.Name(), filepath.Join(c.CacheDir, key)); err != nil {
		os.Remove(tmp.Name())
	}
}

func (c *Client) load(key string) ([]byte, time.Time, bool) {
	if c.CacheDir == "" {
		return nil, time.Time{}, false
	}
	p := filepath.Join(c.CacheDir, key)
	st, err := os.Stat(p)
	if err != nil {
		return nil, time.Time{}, false
	}
	body, err := os.ReadFile(p)
	if err != nil {
		return nil, time.Time{}, false
	}
	return body, st.ModTime(), true
}

// redact drops query strings (API keys!) from URLs before logging.
func redact(url string) string {
	for i := 0; i < len(url); i++ {
		if url[i] == '?' {
			return url[:i] + "?…"
		}
	}
	return url
}
```

- [ ] **Step 4: Tests laufen lassen**

Run: `go test ./internal/fetch/`
Expected: `ok  	inkycal/internal/fetch`

- [ ] **Step 5: Commit**

```bash
git add internal/fetch
git commit -m "feat(fetch): http client with retries and offline cache"
```

---

### Task 8: `state` und `schedule`

**Files:**
- Create: `internal/state/state.go`, `internal/schedule/schedule.go`
- Test: `internal/state/state_test.go`, `internal/schedule/schedule_test.go`

**Interfaces:**
- Consumes: –
- Produces: `state.State{LastHash string; Counter int; LastCalibration time.Time}`, `state.Load(dir string) (State, error)` (fehlende Datei → Nullwert), `state.Save(dir string, s State) error` (atomar), `state.AppendTiming(dir, line string) error`; `schedule.NextWake(now time.Time, intervalMin int, minSleep time.Duration) time.Time`, `schedule.CalibrationDue(now time.Time, hours []int, last time.Time) bool`

- [ ] **Step 1: Failing Tests schreiben**

`internal/state/state_test.go`:

```go
package state

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestLoadMissingIsZero(t *testing.T) {
	s, err := Load(t.TempDir())
	if err != nil || s.LastHash != "" || s.Counter != 0 {
		t.Fatalf("%+v %v", s, err)
	}
}

func TestSaveLoadRoundTrip(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "sub")
	want := State{LastHash: "abc", Counter: 4, LastCalibration: time.Date(2026, 10, 10, 12, 0, 0, 0, time.UTC)}
	if err := Save(dir, want); err != nil {
		t.Fatal(err)
	}
	got, err := Load(dir)
	if err != nil || got.LastHash != want.LastHash || got.Counter != 4 || !got.LastCalibration.Equal(want.LastCalibration) {
		t.Fatalf("%+v %v", got, err)
	}
	entries, _ := os.ReadDir(dir)
	if len(entries) != 1 {
		t.Errorf("temp files left behind: %v", entries)
	}
}

func TestLoadCorruptFileErrors(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "state.json"), []byte("{"), 0o644)
	if _, err := Load(dir); err == nil {
		t.Fatal("expected error")
	}
}

func TestAppendTiming(t *testing.T) {
	dir := t.TempDir()
	AppendTiming(dir, "a")
	AppendTiming(dir, "b")
	data, _ := os.ReadFile(filepath.Join(dir, "timings.csv"))
	if string(data) != "a\nb\n" {
		t.Errorf("%q", data)
	}
}
```

`internal/schedule/schedule_test.go`:

```go
package schedule

import (
	"testing"
	"time"
)

func at(h, m, s int) time.Time { return time.Date(2026, 10, 10, h, m, s, 0, time.UTC) }

func TestNextWake(t *testing.T) {
	cases := []struct {
		now      time.Time
		interval int
		want     time.Time
	}{
		{at(10, 7, 30), 15, at(10, 15, 0)},
		{at(10, 15, 0), 15, at(10, 30, 0)},
		{at(23, 50, 0), 60, time.Date(2026, 10, 11, 0, 0, 0, 0, time.UTC)},
		{at(10, 0, 0), 240, at(12, 0, 0)},   // 4h slots since midnight: 0,4,8,12
		{at(10, 14, 30), 15, at(10, 30, 0)}, // 10:15 is only 30 s away → skip
	}
	for _, c := range cases {
		if got := NextWake(c.now, c.interval, 90*time.Second); !got.Equal(c.want) {
			t.Errorf("NextWake(%s, %d) = %s, want %s", c.now.Format("15:04:05"), c.interval, got, c.want)
		}
	}
}

func TestNextWakeAcrossDST(t *testing.T) {
	berlin, err := time.LoadLocation("Europe/Berlin")
	if err != nil {
		t.Skip("no tzdata")
	}
	now := time.Date(2026, 3, 29, 1, 30, 0, 0, berlin) // clocks jump 02:00 → 03:00
	got := NextWake(now, 60, time.Minute)
	if got.Hour() != 3 || got.Sub(now) != 30*time.Minute {
		t.Errorf("got %s (in %s)", got, got.Sub(now))
	}
}

func TestCalibrationDue(t *testing.T) {
	hours := []int{0, 12}
	if !CalibrationDue(at(12, 5, 0), hours, time.Time{}) {
		t.Error("should calibrate at 12:05 when never calibrated")
	}
	if CalibrationDue(at(12, 35, 0), hours, at(12, 5, 0)) {
		t.Error("already calibrated this hour")
	}
	if !CalibrationDue(at(12, 5, 0), hours, at(12, 5, 0).AddDate(0, 0, -1)) {
		t.Error("yesterday's calibration does not count")
	}
	if CalibrationDue(at(13, 0, 0), hours, time.Time{}) {
		t.Error("13 is not a calibration hour")
	}
}
```

- [ ] **Step 2: Tests laufen lassen – müssen fehlschlagen**

Run: `go test ./internal/state/ ./internal/schedule/`
Expected: FAIL – `undefined: Load`, `undefined: NextWake` …

- [ ] **Step 3: Implementieren**

`internal/state/state.go`:

```go
// Package state persists what must survive a power-off: last frame hash, counters, timings.
package state

import (
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"time"
)

type State struct {
	LastHash        string    `json:"last_hash"`
	Counter         int       `json:"counter"` // successful cycles without module errors
	LastCalibration time.Time `json:"last_calibration"`
}

const fileName = "state.json"

// Load returns the zero State if no state file exists yet.
func Load(dir string) (State, error) {
	var s State
	data, err := os.ReadFile(filepath.Join(dir, fileName))
	if errors.Is(err, fs.ErrNotExist) {
		return s, nil
	}
	if err != nil {
		return s, err
	}
	err = json.Unmarshal(data, &s)
	return s, err
}

// Save writes atomically (tmp file, fsync, rename) so a power cut never leaves a broken file.
func Save(dir string, s State) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, ".state-*")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmp.Name(), filepath.Join(dir, fileName)); err != nil {
		return err
	}
	d, err := os.Open(dir)
	if err != nil {
		return err
	}
	defer d.Close()
	return d.Sync()
}

// AppendTiming appends one CSV line to timings.csv (wake-time measurements).
func AppendTiming(dir, line string) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	f, err := os.OpenFile(filepath.Join(dir, "timings.csv"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	if _, err := f.WriteString(line + "\n"); err != nil {
		f.Close()
		return err
	}
	return f.Close()
}
```

`internal/schedule/schedule.go`:

```go
// Package schedule computes wake-up times and calibration slots.
package schedule

import "time"

// NextWake returns the next wall-clock slot aligned to intervalMin minutes since midnight
// (same as Python's countdown), but at least minSleep in the future so the RTC alarm is
// never set to a time that passes while the Pi is still shutting down.
func NextWake(now time.Time, intervalMin int, minSleep time.Duration) time.Time {
	if intervalMin <= 0 {
		intervalMin = 60
	}
	y, m, d := now.Date()
	since := now.Hour()*60 + now.Minute()
	slot := (since/intervalMin + 1) * intervalMin
	for {
		wake := time.Date(y, m, d, 0, slot, 0, 0, now.Location())
		if wake.Sub(now) >= minSleep {
			return wake
		}
		slot += intervalMin
	}
}

// CalibrationDue is true during a configured hour unless we already calibrated in that hour.
func CalibrationDue(now time.Time, hours []int, last time.Time) bool {
	inHour := false
	for _, h := range hours {
		if h == now.Hour() {
			inHour = true
			break
		}
	}
	if !inHour {
		return false
	}
	l := last.In(now.Location())
	ly, lm, ld := l.Date()
	ny, nm, nd := now.Date()
	return !(ly == ny && lm == nm && ld == nd && l.Hour() == now.Hour())
}
```

- [ ] **Step 4: Tests laufen lassen**

Run: `go test ./internal/state/ ./internal/schedule/`
Expected: beide `ok`

- [ ] **Step 5: Commit**

```bash
git add internal/state internal/schedule
git commit -m "feat: persistent state and wake-up scheduling"
```

---

### Task 9: `pisugar` und `sysinfo`

**Files:**
- Create: `internal/pisugar/pisugar.go`
- Create: `internal/sysinfo/sysinfo.go`, `internal/sysinfo/sysinfo_linux.go`, `internal/sysinfo/sysinfo_other.go`
- Test: `internal/pisugar/pisugar_test.go`, `internal/sysinfo/sysinfo_test.go`

**Interfaces:**
- Consumes: –
- Produces: `pisugar.Client{Addr string; Timeout time.Duration}`, `pisugar.New(addr string) *Client`, Methoden `Battery() (float64, error)`, `PowerPlugged() (bool, error)`, `RTCToPi() error`, `PiToRTC() error`, `SetAlarm(t time.Time, repeat int) error` (sendet `rtc_alarm_set <RFC3339> <repeat>`, erwartet `rtc_alarm_set: done`); `sysinfo.NTPSynced() bool`, `sysinfo.Uptime() (time.Duration, error)`, `sysinfo.PowerOff() error`, `sysinfo.Maintenance() bool`, `var sysinfo.MaintenanceFiles []string`

Protokoll laut `pisugar-power-manager-rs`-README: eine Zeile pro Befehl an TCP `127.0.0.1:8423`, Antwort `<key>: <wert>`; zwischendurch kann der Server Ereigniszeilen (z.B. `single`) senden – die werden übersprungen.

- [ ] **Step 1: Failing Tests schreiben**

`internal/pisugar/pisugar_test.go`:

```go
package pisugar

import (
	"bufio"
	"net"
	"strings"
	"sync"
	"testing"
	"time"
)

// fakeServer answers like pisugar-server; it records received commands.
type fakeServer struct {
	ln      net.Listener
	replies map[string]string // command prefix → reply lines
	mu      sync.Mutex
	got     []string
}

func startFake(t *testing.T, replies map[string]string) *fakeServer {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	s := &fakeServer{ln: ln, replies: replies}
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			go func(conn net.Conn) {
				defer conn.Close()
				line, _ := bufio.NewReader(conn).ReadString('\n')
				line = strings.TrimSpace(line)
				s.mu.Lock()
				s.got = append(s.got, line)
				s.mu.Unlock()
				best := ""
				for prefix := range s.replies { // longest matching prefix wins
					if strings.HasPrefix(line, prefix) && len(prefix) > len(best) {
						best = prefix
					}
				}
				if best != "" {
					conn.Write([]byte(s.replies[best]))
				}
			}(conn)
		}
	}()
	t.Cleanup(func() { ln.Close() })
	return s
}

func client(s *fakeServer) *Client {
	c := New(s.ln.Addr().String())
	c.Timeout = time.Second
	return c
}

func TestBatteryAndPlugged(t *testing.T) {
	s := startFake(t, map[string]string{
		"get battery_power_plugged": "battery_power_plugged: true\n",
		"get battery":               "single\nbattery: 87.5\n", // unrelated event line first
	})
	c := client(s)
	if v, err := c.Battery(); err != nil || v != 87.5 {
		t.Fatalf("battery %v %v", v, err)
	}
	if v, err := c.PowerPlugged(); err != nil || !v {
		t.Fatalf("plugged %v %v", v, err)
	}
}

func TestSetAlarmFormat(t *testing.T) {
	s := startFake(t, map[string]string{"rtc_alarm_set": "rtc_alarm_set: done\n"})
	loc := time.FixedZone("CEST", 2*3600)
	if err := client(s).SetAlarm(time.Date(2026, 10, 10, 14, 30, 0, 123, loc), 127); err != nil {
		t.Fatal(err)
	}
	if s.got[0] != "rtc_alarm_set 2026-10-10T14:30:00+02:00 127" {
		t.Errorf("sent %q", s.got[0])
	}
}

func TestActionFailure(t *testing.T) {
	s := startFake(t, map[string]string{"rtc_pi2rtc": "rtc_pi2rtc: failed\n"})
	if err := client(s).PiToRTC(); err == nil {
		t.Fatal("expected error")
	}
}

func TestServerDown(t *testing.T) {
	ln, _ := net.Listen("tcp", "127.0.0.1:0")
	addr := ln.Addr().String()
	ln.Close()
	c := New(addr)
	c.Timeout = 300 * time.Millisecond
	if _, err := c.Battery(); err == nil {
		t.Fatal("expected error")
	}
}
```

`internal/sysinfo/sysinfo_test.go`:

```go
package sysinfo

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestUptime(t *testing.T) {
	p := filepath.Join(t.TempDir(), "uptime")
	os.WriteFile(p, []byte("12.34 40.00\n"), 0o644)
	old := uptimePath
	uptimePath = p
	defer func() { uptimePath = old }()
	d, err := Uptime()
	if err != nil || d != 12340*time.Millisecond {
		t.Fatalf("%v %v", d, err)
	}
}

func TestMaintenance(t *testing.T) {
	dir := t.TempDir()
	old := MaintenanceFiles
	defer func() { MaintenanceFiles = old }()
	MaintenanceFiles = []string{filepath.Join(dir, "a"), filepath.Join(dir, "b")}
	if Maintenance() {
		t.Fatal("no file yet")
	}
	os.WriteFile(MaintenanceFiles[1], nil, 0o644)
	if !Maintenance() {
		t.Fatal("flag file not detected")
	}
}
```

- [ ] **Step 2: Tests laufen lassen – müssen fehlschlagen**

Run: `go test ./internal/pisugar/ ./internal/sysinfo/`
Expected: FAIL – `undefined: New`, `undefined: Uptime` …

- [ ] **Step 3: Implementieren**

`internal/pisugar/pisugar.go`:

```go
// Package pisugar talks to pisugar-server's TCP interface (default 127.0.0.1:8423).
package pisugar

import (
	"bufio"
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"
)

type Client struct {
	Addr    string
	Timeout time.Duration // per command, including waiting for the server to come up after boot
}

func New(addr string) *Client { return &Client{Addr: addr, Timeout: 5 * time.Second} }

// exec sends one command and returns the value of the "<key>: <value>" reply line.
func (c *Client) exec(cmd string) (string, error) {
	fields := strings.Fields(cmd)
	key := fields[0]
	if key == "get" && len(fields) > 1 {
		key = fields[1]
	}
	deadline := time.Now().Add(c.Timeout)
	var conn net.Conn
	var err error
	for {
		conn, err = net.DialTimeout("tcp", c.Addr, time.Second)
		if err == nil || time.Now().After(deadline) {
			break
		}
		time.Sleep(200 * time.Millisecond) // server may still be starting
	}
	if err != nil {
		return "", fmt.Errorf("pisugar: %w", err)
	}
	defer conn.Close()
	conn.SetDeadline(deadline)
	if _, err := conn.Write([]byte(cmd + "\n")); err != nil {
		return "", fmt.Errorf("pisugar %q: %w", cmd, err)
	}
	sc := bufio.NewScanner(conn)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if v, ok := strings.CutPrefix(line, key+":"); ok {
			return strings.TrimSpace(v), nil
		}
	}
	if err := sc.Err(); err != nil {
		return "", fmt.Errorf("pisugar %q: %w", cmd, err)
	}
	return "", fmt.Errorf("pisugar %q: no reply", cmd)
}

func (c *Client) action(cmd string) error {
	v, err := c.exec(cmd)
	if err != nil {
		return err
	}
	if v != "done" {
		return fmt.Errorf("pisugar %q: %s", cmd, v)
	}
	return nil
}

// Battery returns the charge level in percent.
func (c *Client) Battery() (float64, error) {
	v, err := c.exec("get battery")
	if err != nil {
		return 0, err
	}
	return strconv.ParseFloat(v, 64)
}

// PowerPlugged reports whether USB power is connected (PiSugar 3).
func (c *Client) PowerPlugged() (bool, error) {
	v, err := c.exec("get battery_power_plugged")
	if err != nil {
		return false, err
	}
	return strconv.ParseBool(v)
}

// RTCToPi sets the system clock from the PiSugar RTC.
func (c *Client) RTCToPi() error { return c.action("rtc_rtc2pi") }

// PiToRTC writes the (NTP-synchronised) system clock into the RTC.
func (c *Client) PiToRTC() error { return c.action("rtc_pi2rtc") }

// SetAlarm programs the wake-up; repeat is a weekday bitmask (127 = every day).
func (c *Client) SetAlarm(t time.Time, repeat int) error {
	return c.action(fmt.Sprintf("rtc_alarm_set %s %d", t.Truncate(time.Second).Format(time.RFC3339), repeat))
}
```

`internal/sysinfo/sysinfo.go`:

```go
// Package sysinfo wraps the few OS facts the app needs (Linux; stubs elsewhere).
package sysinfo

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

var (
	uptimePath = "/proc/uptime"
	// MaintenanceFiles: if one exists, the device never powers off (so you can SSH in).
	MaintenanceFiles = []string{"/boot/firmware/inkycal-maintenance", "/boot/inkycal-maintenance"}
)

// Uptime is the time since the kernel booted (= how long the OS took before the app started).
func Uptime() (time.Duration, error) {
	data, err := os.ReadFile(uptimePath)
	if err != nil {
		return 0, err
	}
	fields := strings.Fields(string(data))
	if len(fields) == 0 {
		return 0, fmt.Errorf("empty %s", uptimePath)
	}
	secs, err := strconv.ParseFloat(fields[0], 64)
	if err != nil {
		return 0, err
	}
	return time.Duration(secs * float64(time.Second)), nil
}

// Maintenance reports whether a maintenance flag file exists.
func Maintenance() bool {
	for _, p := range MaintenanceFiles {
		if _, err := os.Stat(p); err == nil {
			return true
		}
	}
	return false
}
```

`internal/sysinfo/sysinfo_linux.go`:

```go
//go:build linux

package sysinfo

import (
	"os/exec"

	"golang.org/x/sys/unix"
)

// NTPSynced reports the kernel's clock-synchronised flag (what timedatectl shows as "System clock synchronized").
func NTPSynced() bool {
	var tx unix.Timex
	if _, err := unix.Adjtimex(&tx); err != nil {
		return false
	}
	return tx.Status&unix.STA_UNSYNC == 0
}

// PowerOff asks systemd to power off (the PiSugar alarm must be set before).
func PowerOff() error { return exec.Command("systemctl", "poweroff").Run() }
```

`internal/sysinfo/sysinfo_other.go`:

```go
//go:build !linux

package sysinfo

import "errors"

func NTPSynced() bool { return false }

func PowerOff() error { return errors.New("power off is only supported on Linux") }
```

- [ ] **Step 4: Tests laufen lassen**

Run: `go test ./internal/pisugar/ ./internal/sysinfo/ && CGO_ENABLED=0 GOOS=linux GOARCH=arm GOARM=7 go build ./...`
Expected: beide `ok`, Build ohne Ausgabe

- [ ] **Step 5: Commit**

```bash
git add internal/pisugar internal/sysinfo
git commit -m "feat: pisugar-server client and system helpers"
```

---

### Task 10: `ical` – Kalender laden und expandieren

**Files:**
- Create: `internal/ical/ical.go`
- Test: `internal/ical/ical_test.go`

**Interfaces:**
- Consumes: `fetch.Client.Get` (Task 7)
- Produces: `ical.Event{Title string; Start, End time.Time; AllDay bool}` (End exklusiv, Zeiten in `loc`), `ical.Parse(r io.Reader, from, to time.Time, loc *time.Location) ([]Event, error)` (überlappend mit `[from, to)`, sortiert), `ical.Load(ctx, fc *fetch.Client, urls, files []string, from, to time.Time, loc *time.Location) ([]Event, bool, error)` (URLs parallel, `webcal://` → `https://`, leere Listen → keine Events, sortiert; bool = irgendeine Quelle aus dem Cache)

Bekannte gocal-Eigenheiten, die `Parse` ausgleicht: Bereichsprüfung ist an den Rändern exklusiv (wir erweitern um ±24 h und filtern selbst), Ganztags-Enden kommen als `23:59:59.999` (wir runden auf die exklusive Mitternacht), unbekannte TZIDs werden UTC (wir mappen Windows-Zonen), ein kaputtes Event bricht ohne `StrictModeFailEvent` den ganzen Feed ab.

- [ ] **Step 1: Abhängigkeit holen**

```bash
go get github.com/apognu/gocal@v0.9.1
```

- [ ] **Step 2: Failing Tests schreiben**

`internal/ical/ical_test.go`:

```go
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
```

- [ ] **Step 3: Tests laufen lassen – müssen fehlschlagen**

Run: `go test ./internal/ical/`
Expected: FAIL – `undefined: Parse`, `undefined: Load`

- [ ] **Step 4: Implementieren**

`internal/ical/ical.go`:

```go
// Package ical loads iCalendar feeds/files and expands recurring events (via gocal).
package ical

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/apognu/gocal"

	"inkycal/internal/fetch"
)

type Event struct {
	Title      string
	Start, End time.Time // in the requested location; End is exclusive
	AllDay     bool
}

// windowsZones maps Outlook/Exchange TZIDs to IANA names (gocal would silently fall back to UTC).
var windowsZones = map[string]string{
	"W. Europe Standard Time":        "Europe/Berlin",
	"Central Europe Standard Time":   "Europe/Budapest",
	"Central European Standard Time": "Europe/Warsaw",
	"Romance Standard Time":          "Europe/Paris",
	"GMT Standard Time":              "Europe/London",
	"E. Europe Standard Time":        "Europe/Chisinau",
	"FLE Standard Time":              "Europe/Kiev",
	"Eastern Standard Time":          "America/New_York",
	"Central Standard Time":          "America/Chicago",
	"Mountain Standard Time":         "America/Denver",
	"Pacific Standard Time":          "America/Los_Angeles",
	"UTC":                            "UTC",
}

func init() {
	gocal.SetTZMapper(func(s string) (*time.Location, error) {
		if name, ok := windowsZones[s]; ok {
			return time.LoadLocation(name)
		}
		return nil, fmt.Errorf("no windows mapping for %q", s)
	})
}

func midnight(t time.Time) time.Time {
	y, m, d := t.Date()
	return time.Date(y, m, d, 0, 0, 0, 0, t.Location())
}

// Parse returns events overlapping [from, to), sorted by start.
func Parse(r io.Reader, from, to time.Time, loc *time.Location) ([]Event, error) {
	// gocal's own range check excludes events starting exactly at Start, so widen and filter ourselves.
	start, end := from.Add(-24*time.Hour), to.Add(24*time.Hour)
	p := gocal.NewParser(r)
	p.Start, p.End = &start, &end
	p.AllDayEventsTZ = loc
	p.Strict.Mode = gocal.StrictModeFailEvent // one broken event must not hide the whole calendar
	if err := p.Parse(); err != nil {
		return nil, err
	}
	var out []Event
	for _, e := range p.Events {
		if e.Start == nil {
			continue
		}
		s := e.Start.In(loc)
		allDay := e.RawStart.Params["VALUE"] == "DATE" || len(e.RawStart.Value) == 8
		en := s
		if e.End != nil {
			en = e.End.In(loc)
		}
		if allDay {
			// gocal reports all-day ends as 23:59:59.999 of the last day; make them exclusive midnights.
			en = midnight(en.Add(time.Millisecond))
			if !en.After(s) {
				en = s.AddDate(0, 0, 1)
			}
		} else if s.Equal(midnight(s)) && en.Equal(midnight(en)) && en.Sub(s) >= 24*time.Hour {
			allDay = true // same rule as Python's iCalendar.all_day
		}
		overlaps := s.Before(to) && (en.After(from) || (en.Equal(s) && !s.Before(from)))
		if !overlaps {
			continue
		}
		out = append(out, Event{Title: strings.TrimSpace(e.Summary), Start: s, End: en, AllDay: allDay})
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Start.Before(out[j].Start) })
	return out, nil
}

// Load fetches all URLs in parallel (webcal:// is treated as https://), reads all files,
// and returns the merged, sorted events. stale is true if any URL came from the cache.
func Load(ctx context.Context, fc *fetch.Client, urls, files []string, from, to time.Time, loc *time.Location) ([]Event, bool, error) {
	type result struct {
		events []Event
		stale  bool
		err    error
	}
	results := make([]result, len(urls))
	var wg sync.WaitGroup
	for i, u := range urls {
		wg.Add(1)
		go func(i int, u string) {
			defer wg.Done()
			if rest, ok := strings.CutPrefix(u, "webcal://"); ok {
				u = "https://" + rest
			}
			res, err := fc.Get(ctx, u, nil)
			if err != nil {
				results[i].err = fmt.Errorf("calendar %d: %w", i+1, err)
				return
			}
			ev, err := Parse(bytes.NewReader(res.Body), from, to, loc)
			if err != nil {
				results[i].err = fmt.Errorf("calendar %d: %w", i+1, err)
				return
			}
			results[i] = result{events: ev, stale: res.Stale}
		}(i, u)
	}
	wg.Wait()

	var all []Event
	stale := false
	for _, r := range results {
		if r.err != nil {
			return nil, false, r.err
		}
		all = append(all, r.events...)
		stale = stale || r.stale
	}
	for _, path := range files {
		f, err := os.Open(path)
		if err != nil {
			return nil, false, err
		}
		ev, err := Parse(f, from, to, loc)
		f.Close()
		if err != nil {
			return nil, false, fmt.Errorf("%s: %w", path, err)
		}
		all = append(all, ev...)
	}
	sort.SliceStable(all, func(i, j int) bool { return all[i].Start.Before(all[j].Start) })
	return all, stale, nil
}
```

- [ ] **Step 5: Tests laufen lassen**

Run: `go mod tidy && go test ./internal/ical/`
Expected: `ok  	inkycal/internal/ical`

- [ ] **Step 6: Commit**

```bash
git add go.mod go.sum internal/ical
git commit -m "feat(ical): load and expand calendars via gocal"
```

---

### Task 11: `modules` und Agenda-Modul

**Files:**
- Create: `internal/modules/module.go`, `internal/modules/agenda/agenda.go`
- Test: `internal/modules/agenda/agenda_test.go`

**Interfaces:**
- Consumes: `config.Common`, `config.StringList`, `config.FlexInt` (Task 1), `timefmt.Format` (Task 2), `render.*` (Task 3), `fetch.Client` (Task 7), `ical.Event`, `ical.Load` (Task 10)
- Produces: `modules.Env{Now time.Time; Loc *time.Location; Fetch *fetch.Client; Stale bool}`, `modules.Module` (`Generate(ctx context.Context, env *Env) (*render.Canvas, error)`), `modules.Factory func(c config.Common, raw json.RawMessage) (Module, error)`; `agenda.New(c config.Common, raw json.RawMessage) (modules.Module, error)`

Port von `inkycal_agenda.py`: ein Datumskopf pro Tag (rote Trennlinie), darunter Termine mit Uhrzeit rechtsbündig bzw. Kalender-Icon bei Ganztagsterminen, Titel umbrochen; 1 oder 2 Spalten; ein Datumskopf bleibt nie allein am Spaltenende; beginnt eine Spalte mit einem Termin, wird der Datumskopf wiederholt.

- [ ] **Step 1: Failing Tests schreiben**

`internal/modules/agenda/agenda_test.go`:

```go
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
```

- [ ] **Step 2: Tests laufen lassen – müssen fehlschlagen**

Run: `go test ./internal/modules/...`
Expected: FAIL – `undefined: buildItems`, `undefined: layout`, `undefined: New`; Paket `inkycal/internal/modules` fehlt

- [ ] **Step 3: Implementieren**

`internal/modules/module.go`:

```go
// Package modules defines what a dashboard module is.
package modules

import (
	"context"
	"encoding/json"
	"time"

	"inkycal/internal/config"
	"inkycal/internal/fetch"
	"inkycal/internal/render"
)

// Env is passed to every module run. Each module gets its own copy.
type Env struct {
	Now   time.Time // already in Loc
	Loc   *time.Location
	Fetch *fetch.Client
	Stale bool // set by the module when it rendered cached data
}

// Module renders one section of the dashboard.
type Module interface {
	// Generate returns a canvas of the module's inner size (config size minus padding).
	Generate(ctx context.Context, env *Env) (*render.Canvas, error)
}

// Factory builds a module from the shared keys and its raw JSON config.
type Factory func(c config.Common, raw json.RawMessage) (Module, error)
```

`internal/modules/agenda/agenda.go`:

```go
// Package agenda lists upcoming iCalendar events grouped by day (port of inkycal_agenda.py).
package agenda

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"time"

	"inkycal/internal/config"
	"inkycal/internal/ical"
	"inkycal/internal/modules"
	"inkycal/internal/render"
	"inkycal/internal/timefmt"
)

type settings struct {
	ICalURLs   config.StringList `json:"ical_urls"`
	ICalFiles  config.StringList `json:"ical_files"`
	DateFormat string            `json:"date_format"`
	TimeFormat string            `json:"time_format"`
	Columns    config.FlexInt    `json:"columns"`
}

type Agenda struct {
	common config.Common
	s      settings
}

// New validates the config; defaults match the Python module.
func New(c config.Common, raw json.RawMessage) (modules.Module, error) {
	var s settings
	if err := json.Unmarshal(raw, &s); err != nil {
		return nil, fmt.Errorf("agenda: %w", err)
	}
	if s.DateFormat == "" {
		s.DateFormat = "ddd D MMM"
	}
	if s.TimeFormat == "" {
		s.TimeFormat = "HH:mm"
	}
	if s.Columns == 0 {
		s.Columns = 1
	}
	if s.Columns < 1 || s.Columns > 2 {
		return nil, fmt.Errorf("agenda: columns must be 1 or 2, got %d", s.Columns)
	}
	return &Agenda{common: c, s: s}, nil
}

type item struct {
	isDate bool
	title  string
	start  time.Time
	allDay bool
}

// buildItems merges one header per day with the events; events that started before today
// sort under today's header, headers come before events of the same time.
func buildItems(today time.Time, days int, events []ical.Event, dateFmt, lang string) []item {
	var items []item
	for i := 0; i < days; i++ {
		d := today.AddDate(0, 0, i)
		items = append(items, item{isDate: true, title: timefmt.Format(d, dateFmt, lang), start: d})
	}
	for _, e := range events {
		items = append(items, item{title: e.Title, start: e.Start, allDay: e.AllDay})
	}
	key := func(it item) time.Time {
		if it.start.Before(today) {
			return today
		}
		return it.start
	}
	sort.SliceStable(items, func(i, j int) bool {
		ki, kj := key(items[i]), key(items[j])
		if !ki.Equal(kj) {
			return ki.Before(kj)
		}
		return items[i].isDate && !items[j].isDate
	})
	return items
}

type row struct {
	isDate bool
	text   string
	start  time.Time
	allDay bool
	col    int
	y, h   int
}

// layout places items into columns: a date header is never left alone at the bottom of a
// column, and a column that starts with an event repeats the current date header.
func layout(items []item, cv *render.Canvas, lh, colH, cols, titleWidth int) []row {
	height := func(it item) int {
		if it.isDate {
			return lh
		}
		return max(1, len(cv.Wrap(it.title, titleWidth))) * lh
	}
	var rows []row
	col, y := 0, 0
	lastDate := ""
	for i, it := range items {
		h := height(it)
		force := it.isDate && i+1 < len(items) && !items[i+1].isDate && y+h+height(items[i+1]) > colH
		if force || y+h > colH {
			col++
			y = 0
			if col >= cols {
				break
			}
			if !it.isDate && lastDate != "" {
				rows = append(rows, row{isDate: true, text: lastDate, col: col, y: 0, h: lh})
				y = lh
			}
		}
		if it.isDate {
			lastDate = it.title
		}
		rows = append(rows, row{isDate: it.isDate, text: it.title, start: it.start, allDay: it.allDay, col: col, y: y, h: h})
		y += h
	}
	return rows
}

func (a *Agenda) Generate(ctx context.Context, env *modules.Env) (*render.Canvas, error) {
	w, h := a.common.InnerSize()
	lang := a.common.Language
	cv := render.NewCanvas(w, h, render.FontDefault, int(a.common.FontSize))
	lh := cv.LineHeight() + 1
	cols := int(a.s.Columns)
	gutter := 0
	if cols > 1 {
		gutter = 10
	}
	colW := (w - (cols-1)*gutter) / cols
	days := (h / lh) * cols

	y, m, d := env.Now.Date()
	today := time.Date(y, m, d, 0, 0, 0, 0, env.Loc)
	events, stale, err := ical.Load(ctx, env.Fetch, a.s.ICalURLs, a.s.ICalFiles, today, today.AddDate(0, 0, days-1), env.Loc)
	if err != nil {
		return nil, err
	}
	env.Stale = stale

	items := buildItems(today, days, events, a.s.DateFormat, lang)
	dateWidth := 0
	for _, it := range items {
		if it.isDate {
			dateWidth = max(dateWidth, cv.TextWidth(it.title))
		}
	}
	dateWidth = min(dateWidth, colW)
	timeWidth := 0
	for _, e := range events {
		timeWidth = max(timeWidth, cv.TextWidth(timefmt.Format(e.Start, a.s.TimeFormat, lang)))
	}
	timeWidth += 10
	xTime := dateWidth / 3
	xEvent := xTime + timeWidth
	const bullet = " • "
	bulletWidth := cv.TextWidth(bullet)
	titleWidth := colW - xEvent - bulletWidth

	left := render.TextOpts{Align: render.AlignLeft}
	for _, r := range layout(items, cv, lh, h, cols, titleWidth) {
		x0 := r.col * (colW + gutter)
		if r.isDate {
			cv.HLine(x0, x0+colW-1, r.y, render.InkColour)
			cv.Write(x0, r.y, dateWidth, r.h, r.text, left)
			continue
		}
		if r.allDay {
			cv.Icon(x0+xTime+timeWidth-lh, r.y, lh, lh, "", render.FontMaterial, render.InkBlack, 0.8)
		} else {
			cv.Write(x0+xTime, r.y, timeWidth, lh, timefmt.Format(r.start, a.s.TimeFormat, lang), render.TextOpts{Align: render.AlignRight})
		}
		cv.Write(x0+xEvent, r.y, bulletWidth, lh, bullet, left)
		cv.Write(x0+xEvent+bulletWidth, r.y, titleWidth, r.h, r.text, left)
	}
	return cv, nil
}
```

- [ ] **Step 4: Tests laufen lassen**

Run: `go test ./internal/modules/...`
Expected: `ok  	inkycal/internal/modules/agenda`

- [ ] **Step 5: Commit**

```bash
git add internal/modules
git commit -m "feat(modules): module interface and agenda module"
```

---

### Task 12: `app` – Zyklus, Loop, Registry

**Files:**
- Create: `internal/app/registry.go`, `internal/app/app.go`
- Test: `internal/app/app_test.go`

**Interfaces:**
- Consumes: alles aus Task 1–11 (`config`, `epd.Display`, `fetch.Client`, `modules.*`, `render.Assemble/Pack`, `schedule.*`, `state.*`, `timefmt.Format`, `agenda.New`)
- Produces: `app.PiSugar`, `app.System` (Interfaces), `app.Options{Settings, StateDir, Display, PiSugar, System, Fetch, Loc, Now, Registry}`, `app.Timings{Boot, Modules, Render, Display, Total time.Duration}`, `app.Result{Refreshed bool; ModuleErrors []string; Wake time.Time; PowerOff bool; Timings Timings}`, `app.RunCycle(ctx, o *Options) (Result, error)`, `app.Loop(ctx, o *Options) error`, `var app.Registry = map[string]modules.Factory{"Agenda": agenda.New}` (Plan 2 ersetzt die Datei durch ein größeres Map-Literal)

Ablauf siehe Spec §4: RTC→Systemzeit (falls NTP nicht synchron) → Module parallel (60 s Deadline, Fehler = leere Section + „im N: X“) → Assemble/Pack → Refresh nur bei neuem Hash bzw. Kalibrierung, **in einer Goroutine** → währenddessen `rtc_pi2rtc` (nur wenn NTP synchron) und Wecker → warten → State + `timings.csv` → Ausschalt-Entscheidung.

- [ ] **Step 1: Failing Tests schreiben**

`internal/app/app_test.go`:

```go
package app

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"inkycal/internal/config"
	"inkycal/internal/fetch"
	"inkycal/internal/modules"
	"inkycal/internal/render"
	"inkycal/internal/state"
)

type fakeDisplay struct {
	shows, clears int
	err           error
}

func (d *fakeDisplay) Size() (int, int)        { return 800, 480 }
func (d *fakeDisplay) Show(render.Frame) error { d.shows++; return d.err }
func (d *fakeDisplay) Clear(cycles int) error  { d.clears += cycles; return nil }

type fakePiSugar struct {
	alarms           []time.Time
	alarmErr         error
	plugged          bool
	rtcToPi, piToRTC int
}

func (p *fakePiSugar) Battery() (float64, error)   { return 81, nil }
func (p *fakePiSugar) PowerPlugged() (bool, error) { return p.plugged, nil }
func (p *fakePiSugar) RTCToPi() error              { p.rtcToPi++; return nil }
func (p *fakePiSugar) PiToRTC() error              { p.piToRTC++; return nil }
func (p *fakePiSugar) SetAlarm(t time.Time, r int) error {
	p.alarms = append(p.alarms, t)
	return p.alarmErr
}

type fakeSystem struct {
	synced, maintenance bool
	powerOffs           int
}

func (s *fakeSystem) NTPSynced() bool       { return s.synced }
func (s *fakeSystem) Uptime() time.Duration { return 12 * time.Second }
func (s *fakeSystem) PowerOff() error       { s.powerOffs++; return nil }
func (s *fakeSystem) Maintenance() bool     { return s.maintenance }

// dotModule draws a dot; failModule always fails.
type dotModule struct{ c config.Common }

func (m dotModule) Generate(ctx context.Context, env *modules.Env) (*render.Canvas, error) {
	w, h := m.c.InnerSize()
	cv := render.NewCanvas(w, h, render.FontDefault, 14)
	cv.FillCircle(w/2, h/2, 5, render.InkColour)
	return cv, nil
}

type failModule struct{}

func (failModule) Generate(context.Context, *modules.Env) (*render.Canvas, error) {
	return nil, errors.New("boom")
}

var testRegistry = map[string]modules.Factory{
	"Dot":  func(c config.Common, _ json.RawMessage) (modules.Module, error) { return dotModule{c}, nil },
	"Fail": func(config.Common, json.RawMessage) (modules.Module, error) { return failModule{}, nil },
}

func testOptions(t *testing.T, names ...string) (*Options, *fakeDisplay, *fakePiSugar, *fakeSystem) {
	t.Helper()
	s := &config.Settings{
		Model: "epd_7_in_5_v2_colour", UpdateInterval: 60, InfoSection: true, InfoSectionHeight: 40,
		CalibrationCycles: 1, ImageHash: true, UsePiSugar: true, ShutdownAfterRun: true,
	}
	for i, n := range names {
		s.Modules = append(s.Modules, config.ModuleEntry{Position: i + 1, Name: n,
			Config: json.RawMessage(`{"size":[480,200],"padding_x":10,"padding_y":10}`)})
	}
	d, p, sys := &fakeDisplay{}, &fakePiSugar{}, &fakeSystem{}
	now := time.Date(2026, 10, 10, 10, 7, 0, 0, time.UTC)
	return &Options{
		Settings: s, StateDir: t.TempDir(), Display: d, PiSugar: p, System: sys,
		Fetch: fetch.New(""), Loc: time.UTC, Now: func() time.Time { return now }, Registry: testRegistry,
	}, d, p, sys
}

func TestRunCycleRefreshesSetsAlarmAndPowersOff(t *testing.T) {
	o, d, p, _ := testOptions(t, "Dot")
	res, err := RunCycle(context.Background(), o)
	if err != nil {
		t.Fatal(err)
	}
	if d.shows != 1 || !res.Refreshed {
		t.Errorf("shows=%d refreshed=%v", d.shows, res.Refreshed)
	}
	want := time.Date(2026, 10, 10, 11, 0, 0, 0, time.UTC)
	if len(p.alarms) != 1 || !p.alarms[0].Equal(want) || !res.Wake.Equal(want) {
		t.Errorf("alarm %v wake %v", p.alarms, res.Wake)
	}
	if !res.PowerOff {
		t.Error("expected power off")
	}
	st, _ := state.Load(o.StateDir)
	if st.LastHash == "" || st.Counter != 1 {
		t.Errorf("state %+v", st)
	}
}

func TestRunCycleSkipsUnchangedImage(t *testing.T) {
	o, d, _, _ := testOptions(t, "Dot")
	RunCycle(context.Background(), o)
	res, _ := RunCycle(context.Background(), o)
	if d.shows != 1 || res.Refreshed {
		t.Errorf("second identical cycle refreshed: shows=%d", d.shows)
	}
	o.Settings.ImageHash = false
	RunCycle(context.Background(), o)
	if d.shows != 2 {
		t.Error("without image_hash every cycle must refresh")
	}
}

func TestRunCycleModuleErrorStillShows(t *testing.T) {
	o, d, _, _ := testOptions(t, "Dot", "Fail", "Missing")
	res, err := RunCycle(context.Background(), o)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.ModuleErrors) != 2 || d.shows != 1 || !res.PowerOff {
		t.Errorf("errors=%v shows=%d poweroff=%v", res.ModuleErrors, d.shows, res.PowerOff)
	}
	st, _ := state.Load(o.StateDir)
	if st.Counter != 0 {
		t.Errorf("counter must reset on errors, got %d", st.Counter)
	}
}

func TestPowerOffGuards(t *testing.T) {
	cases := map[string]func(*Options, *fakePiSugar, *fakeSystem){
		"maintenance": func(o *Options, p *fakePiSugar, s *fakeSystem) { s.maintenance = true },
		"usb power":   func(o *Options, p *fakePiSugar, s *fakeSystem) { p.plugged = true },
		"alarm fails": func(o *Options, p *fakePiSugar, s *fakeSystem) { p.alarmErr = errors.New("i2c") },
		"disabled":    func(o *Options, p *fakePiSugar, s *fakeSystem) { o.Settings.ShutdownAfterRun = false },
		"no pisugar":  func(o *Options, p *fakePiSugar, s *fakeSystem) { o.PiSugar = nil },
	}
	for name, mutate := range cases {
		o, _, p, s := testOptions(t, "Dot")
		mutate(o, p, s)
		res, _ := RunCycle(context.Background(), o)
		if res.PowerOff {
			t.Errorf("%s: must not power off", name)
		}
	}
}

func TestClockSync(t *testing.T) {
	o, _, p, s := testOptions(t, "Dot")
	RunCycle(context.Background(), o)
	if p.rtcToPi != 1 || p.piToRTC != 0 {
		t.Errorf("unsynced: rtcToPi=%d piToRTC=%d", p.rtcToPi, p.piToRTC)
	}
	s.synced = true
	RunCycle(context.Background(), o)
	if p.rtcToPi != 1 || p.piToRTC != 1 {
		t.Errorf("synced: rtcToPi=%d piToRTC=%d", p.rtcToPi, p.piToRTC)
	}
}

func TestCalibration(t *testing.T) {
	o, d, _, _ := testOptions(t, "Dot")
	o.Settings.CalibrationHours = []int{10}
	RunCycle(context.Background(), o)
	RunCycle(context.Background(), o)
	if d.clears != 1 {
		t.Errorf("calibration must run once per hour, ran %d", d.clears)
	}
}

func TestInfoText(t *testing.T) {
	now := time.Date(2026, 10, 10, 9, 5, 0, 0, time.UTC)
	oc := []moduleOutcome{{}, {err: errors.New("x")}, {stale: true}}
	got := infoText(now, false, oc, 15, true)
	if got != "10 Oct @ 09:05  im 2: X  im 3: cache  Low battery! (15%)" {
		t.Errorf("%q", got)
	}
	if got := infoText(now, true, nil, 80, true); got != "Battery: 80%" {
		t.Errorf("%q", got)
	}
}

func TestLoopPowersOff(t *testing.T) {
	o, _, _, s := testOptions(t, "Dot")
	if err := Loop(context.Background(), o); err != nil {
		t.Fatal(err)
	}
	if s.powerOffs != 1 {
		t.Error("PowerOff not called")
	}
}

func TestLoopStopsOnCancel(t *testing.T) {
	o, _, _, _ := testOptions(t, "Dot")
	o.Settings.ShutdownAfterRun = false
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error)
	go func() { done <- Loop(ctx, o) }()
	time.Sleep(50 * time.Millisecond)
	cancel()
	if err := <-done; !errors.Is(err, context.Canceled) {
		t.Errorf("got %v", err)
	}
}

func TestTimingsWritten(t *testing.T) {
	o, _, _, _ := testOptions(t, "Dot")
	res, _ := RunCycle(context.Background(), o)
	if res.Timings.Boot != 12*time.Second {
		t.Errorf("boot %v", res.Timings.Boot)
	}
	data, err := os.ReadFile(filepath.Join(o.StateDir, "timings.csv"))
	if err != nil || !strings.HasPrefix(string(data), "2026-10-10T10:07:00Z,12.00,") {
		t.Errorf("timings.csv: %q %v", data, err)
	}
}
```

- [ ] **Step 2: Tests laufen lassen – müssen fehlschlagen**

Run: `go test ./internal/app/`
Expected: FAIL – `undefined: Options`, `undefined: RunCycle`, `undefined: Loop` …

- [ ] **Step 3: Implementieren**

`internal/app/registry.go`:

```go
package app

import (
	"inkycal/internal/modules"
	"inkycal/internal/modules/agenda"
)

// Registry maps settings.json module names (as in Python's InkycalModuleImporter) to factories.
var Registry = map[string]modules.Factory{
	"Agenda": agenda.New,
}
```

`internal/app/app.go`:

```go
// Package app runs one dashboard cycle: modules → image → display → RTC alarm → power decision.
package app

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"inkycal/internal/config"
	"inkycal/internal/epd"
	"inkycal/internal/fetch"
	"inkycal/internal/modules"
	"inkycal/internal/render"
	"inkycal/internal/schedule"
	"inkycal/internal/state"
	"inkycal/internal/timefmt"
)

const (
	moduleTimeout = 60 * time.Second
	minSleep      = 90 * time.Second // the RTC alarm must not pass while we shut down
	alarmEveryDay = 127              // PiSugar weekday bitmask
)

type PiSugar interface {
	Battery() (float64, error)
	PowerPlugged() (bool, error)
	RTCToPi() error
	PiToRTC() error
	SetAlarm(t time.Time, repeat int) error
}

type System interface {
	NTPSynced() bool
	Uptime() time.Duration
	PowerOff() error
	Maintenance() bool
}

type Options struct {
	Settings *config.Settings
	StateDir string
	Display  epd.Display
	PiSugar  PiSugar // nil = disabled
	System   System
	Fetch    *fetch.Client
	Loc      *time.Location
	Now      func() time.Time
	Registry map[string]modules.Factory
}

type Timings struct {
	Boot, Modules, Render, Display, Total time.Duration
}

type Result struct {
	Refreshed    bool
	ModuleErrors []string
	Wake         time.Time
	PowerOff     bool
	Timings      Timings
}

type moduleOutcome struct {
	part  render.Part
	err   error
	stale bool
}

// runModules runs all modules concurrently; a failed module leaves its section blank.
func runModules(ctx context.Context, o *Options, now time.Time) []moduleOutcome {
	ctx, cancel := context.WithTimeout(ctx, moduleTimeout)
	defer cancel()
	out := make([]moduleOutcome, len(o.Settings.Modules))
	var wg sync.WaitGroup
	for i, entry := range o.Settings.Modules {
		common, err := config.ParseCommon(entry.Config)
		if err != nil {
			out[i].err = fmt.Errorf("%s: %w", entry.Name, err)
			continue
		}
		w, h := common.InnerSize()
		out[i].part = render.Part{Img: render.NewCanvas(w, h, render.FontDefault, 14), SectionW: common.Size[0], SectionH: common.Size[1]}
		factory, ok := o.Registry[entry.Name]
		if !ok {
			out[i].err = fmt.Errorf("unknown module %q", entry.Name)
			continue
		}
		wg.Add(1)
		go func(i int, entry config.ModuleEntry, common config.Common) {
			defer wg.Done()
			defer func() {
				if r := recover(); r != nil {
					out[i].err = fmt.Errorf("%s panicked: %v", entry.Name, r)
				}
			}()
			m, err := factory(common, entry.Config)
			if err != nil {
				out[i].err = err
				return
			}
			env := &modules.Env{Now: now, Loc: o.Loc, Fetch: o.Fetch}
			cv, err := m.Generate(ctx, env)
			if err != nil {
				out[i].err = fmt.Errorf("%s: %w", entry.Name, err)
				return
			}
			out[i].part.Img = cv
			out[i].stale = env.Stale
		}(i, entry, common)
	}
	wg.Wait()
	return out
}

// infoText reproduces Python's info line (timestamp only without image_hash, which would defeat the hash).
func infoText(now time.Time, imageHash bool, outcomes []moduleOutcome, battery float64, hasBattery bool) string {
	var b strings.Builder
	if !imageHash {
		b.WriteString(timefmt.Format(now, "D MMM @ HH:mm", "en") + "  ")
	}
	for i, oc := range outcomes {
		switch {
		case oc.err != nil:
			fmt.Fprintf(&b, "im %d: X  ", i+1)
		case oc.stale:
			fmt.Fprintf(&b, "im %d: cache  ", i+1)
		}
	}
	if hasBattery {
		if battery < 20 {
			fmt.Fprintf(&b, "Low battery! (%.0f%%) ", battery)
		} else {
			fmt.Fprintf(&b, "Battery: %.0f%% ", battery)
		}
	}
	return strings.TrimSpace(b.String())
}

func shouldPowerOff(s *config.Settings, hasPiSugar, alarmOK, maintenance, plugged bool) bool {
	return hasPiSugar && bool(s.ShutdownAfterRun) && alarmOK && !maintenance && !plugged
}

// RunCycle renders and shows one update. The returned Result is valid even when err != nil.
func RunCycle(ctx context.Context, o *Options) (Result, error) {
	start := time.Now()
	s := o.Settings
	res := Result{Timings: Timings{Boot: o.System.Uptime()}}

	if o.PiSugar != nil && !o.System.NTPSynced() {
		if err := o.PiSugar.RTCToPi(); err != nil {
			slog.Warn("rtc → system clock failed", "err", err)
		}
	}
	now := o.Now().In(o.Loc)
	res.Wake = schedule.NextWake(now, int(s.UpdateInterval), minSleep)
	st, err := state.Load(o.StateDir)
	if err != nil {
		slog.Warn("state unreadable, starting fresh", "err", err)
		st = state.State{}
	}

	outcomes := runModules(ctx, o, now)
	parts := make([]render.Part, len(outcomes))
	for i, oc := range outcomes {
		parts[i] = oc.part
		if oc.err != nil {
			res.ModuleErrors = append(res.ModuleErrors, oc.err.Error())
			slog.Error("module failed", "position", i+1, "err", oc.err)
		}
	}
	res.Timings.Modules = time.Since(start)

	var battery float64
	hasBattery := false
	if o.PiSugar != nil {
		if b, err := o.PiSugar.Battery(); err == nil {
			battery, hasBattery = b, true
		} else {
			slog.Warn("battery level unavailable", "err", err)
		}
	}
	info, infoHeight := "", 0
	if s.InfoSection {
		info, infoHeight = infoText(now, bool(s.ImageHash), outcomes, battery, hasBattery), int(s.InfoSectionHeight)
	}
	panelW, panelH := o.Display.Size()
	canvas := render.Assemble(panelH, panelW, parts, bool(s.BorderAroundModules), info, infoHeight)
	frame, err := render.Pack(canvas, panelW, panelH, int(s.Orientation))
	if err != nil {
		return res, err
	}
	res.Timings.Render = time.Since(start) - res.Timings.Modules

	calibrate := schedule.CalibrationDue(now, s.CalibrationHours, st.LastCalibration)
	res.Refreshed = calibrate || !bool(s.ImageHash) || frame.Hash() != st.LastHash
	displayDone := make(chan error, 1)
	if res.Refreshed {
		go func() {
			if calibrate {
				if err := o.Display.Clear(int(s.CalibrationCycles)); err != nil {
					displayDone <- fmt.Errorf("calibration: %w", err)
					return
				}
			}
			displayDone <- o.Display.Show(frame)
		}()
	} else {
		slog.Info("image unchanged, skipping refresh")
		displayDone <- nil
	}

	// While the panel refreshes (~20 s): RTC housekeeping.
	alarmOK, plugged := false, false
	if o.PiSugar != nil {
		if o.System.NTPSynced() {
			if err := o.PiSugar.PiToRTC(); err != nil {
				slog.Warn("system clock → rtc failed", "err", err)
			}
		}
		if err := o.PiSugar.SetAlarm(res.Wake, alarmEveryDay); err != nil {
			slog.Error("setting wake-up alarm failed, staying on", "err", err)
		} else {
			alarmOK = true
		}
		plugged, _ = o.PiSugar.PowerPlugged() // unsupported on old models → false
	}

	displayErr := <-displayDone
	res.Timings.Display = time.Since(start) - res.Timings.Modules - res.Timings.Render
	if displayErr == nil && res.Refreshed {
		st.LastHash = frame.Hash()
		if calibrate {
			st.LastCalibration = now
		}
	}
	if len(res.ModuleErrors) == 0 {
		st.Counter++
	} else {
		st.Counter = 0
	}
	if err := state.Save(o.StateDir, st); err != nil {
		slog.Warn("saving state failed", "err", err)
	}

	res.PowerOff = shouldPowerOff(s, o.PiSugar != nil, alarmOK, o.System.Maintenance(), plugged)
	res.Timings.Total = time.Since(start)
	t := res.Timings
	slog.Info("cycle done", "boot", t.Boot, "modules", t.Modules, "render", t.Render, "display", t.Display,
		"total", t.Total, "refreshed", res.Refreshed, "wake", res.Wake.Format(time.RFC3339), "power_off", res.PowerOff)
	line := fmt.Sprintf("%s,%.2f,%.2f,%.2f,%.2f,%.2f,%v", now.Format(time.RFC3339),
		t.Boot.Seconds(), t.Modules.Seconds(), t.Render.Seconds(), t.Display.Seconds(), t.Total.Seconds(), res.Refreshed)
	if err := state.AppendTiming(o.StateDir, line); err != nil {
		slog.Warn("writing timings failed", "err", err)
	}
	return res, displayErr
}

// Loop runs cycles until the device powers off or ctx is cancelled.
func Loop(ctx context.Context, o *Options) error {
	for {
		res, err := RunCycle(ctx, o)
		if err != nil {
			slog.Error("cycle failed", "err", err)
		}
		if res.PowerOff {
			slog.Info("powering off", "wake", res.Wake.Format(time.RFC3339))
			return o.System.PowerOff()
		}
		wait := res.Wake.Sub(o.Now())
		slog.Info("sleeping until next update", "wait", wait.Round(time.Second))
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(wait):
		}
	}
}
```

- [ ] **Step 4: Tests laufen lassen (mit Race-Detector – Display und RTC laufen parallel)**

Run: `go test -race ./internal/app/`
Expected: `ok  	inkycal/internal/app`

- [ ] **Step 5: Commit**

```bash
git add internal/app
git commit -m "feat(app): concurrent cycle, hash skip, rtc alarm, guarded power-off"
```

---

### Task 13: `cmd/inkycal` – CLI und End-to-End-Dry-Run

**Files:**
- Create: `cmd/inkycal/main.go`
- Test: `cmd/inkycal/main_test.go`

**Interfaces:**
- Consumes: `app.*`, `config.FindSettings/Load`, `epd.Models/Open/NewPNG`, `fetch.New`, `pisugar.New`, `sysinfo.*`
- Produces: Binary `inkycal` mit Flags `--settings`, `--mode run|once|dry-run|clear`, `--out`, `--state-dir`, `--pisugar`. Exit-Codes: 0 ok, 1 Laufzeitfehler, 2 Aufruffehler. PiSugar wird nur im Modus `run` und nur bei `use_pi_sugar: true` verwendet (`once`/`dry-run` stellen keinen Wecker und schalten nie aus). State-Verzeichnis: `--state-dir`, sonst `$STATE_DIRECTORY` (systemd), sonst bei `dry-run` ein Temp-Verzeichnis, sonst `/var/lib/inkycal`.

- [ ] **Step 1: Failing Test schreiben**

`cmd/inkycal/main_test.go`:

```go
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
```

- [ ] **Step 2: Test laufen lassen – muss fehlschlagen**

Run: `go test ./cmd/inkycal/`
Expected: FAIL – `undefined: run`

- [ ] **Step 3: Implementieren**

`cmd/inkycal/main.go`:

```go
// Command inkycal renders the e-paper dashboard described by settings.json.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"
	_ "time/tzdata" // iCal TZIDs must resolve even on minimal images

	"inkycal/internal/app"
	"inkycal/internal/config"
	"inkycal/internal/epd"
	"inkycal/internal/fetch"
	"inkycal/internal/pisugar"
	"inkycal/internal/sysinfo"
)

type osSystem struct{}

func (osSystem) NTPSynced() bool { return sysinfo.NTPSynced() }
func (osSystem) PowerOff() error { return sysinfo.PowerOff() }
func (osSystem) Maintenance() bool {
	return sysinfo.Maintenance()
}
func (osSystem) Uptime() time.Duration {
	d, _ := sysinfo.Uptime()
	return d
}

func main() { os.Exit(run(os.Args[1:])) }

func run(args []string) int {
	fs := flag.NewFlagSet("inkycal", flag.ContinueOnError)
	settingsPath := fs.String("settings", "", "settings.json (default: $INKYCAL_SETTINGS_PATH, /boot/settings.json, /boot/firmware/settings.json)")
	mode := fs.String("mode", "run", "run | once | dry-run | clear")
	out := fs.String("out", "preview.png", "dry-run: PNG output path")
	stateDir := fs.String("state-dir", "", "state + HTTP cache (default: $STATE_DIRECTORY or /var/lib/inkycal)")
	pisugarAddr := fs.String("pisugar", "127.0.0.1:8423", "pisugar-server TCP address")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	switch *mode {
	case "run", "once", "dry-run", "clear":
	default:
		fmt.Fprintf(os.Stderr, "unknown mode %q\n", *mode)
		return 2
	}
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, nil)))
	if err := runMode(*mode, *settingsPath, *out, resolveStateDir(*stateDir, *mode), *pisugarAddr); err != nil {
		slog.Error("inkycal failed", "err", err)
		return 1
	}
	return 0
}

func resolveStateDir(flagValue, mode string) string {
	switch {
	case flagValue != "":
		return flagValue
	case os.Getenv("STATE_DIRECTORY") != "": // set by systemd StateDirectory=
		return os.Getenv("STATE_DIRECTORY")
	case mode == "dry-run":
		return filepath.Join(os.TempDir(), "inkycal-dry-run")
	default:
		return "/var/lib/inkycal"
	}
}

func runMode(mode, settingsPath, out, stateDir, pisugarAddr string) error {
	path, err := config.FindSettings(settingsPath)
	if err != nil {
		return err
	}
	s, err := config.Load(path)
	if err != nil {
		return err
	}
	size, ok := epd.Models[s.Model]
	if !ok {
		return fmt.Errorf("display model %q not supported", s.Model)
	}
	var display epd.Display
	if mode == "dry-run" {
		display = epd.NewPNG(out, size[0], size[1])
	} else if display, err = epd.Open(s.Model); err != nil {
		return err
	}
	if mode == "clear" {
		return display.Clear(int(s.CalibrationCycles))
	}

	o := &app.Options{
		Settings: s,
		StateDir: stateDir,
		Display:  display,
		System:   osSystem{},
		Fetch:    fetch.New(filepath.Join(stateDir, "http")),
		Loc:      time.Local,
		Now:      time.Now,
		Registry: app.Registry,
	}
	if mode == "run" && s.UsePiSugar {
		o.PiSugar = pisugar.New(pisugarAddr) // assign only when enabled: a typed nil would not be == nil
	}
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	if mode == "run" {
		if err := app.Loop(ctx, o); err != nil && !errors.Is(err, context.Canceled) {
			return err
		}
		return nil
	}
	res, err := app.RunCycle(ctx, o)
	if err != nil {
		return err
	}
	if mode == "dry-run" {
		slog.Info("preview written", "path", out, "module_errors", len(res.ModuleErrors))
	}
	return nil
}
```

- [ ] **Step 4: Alle Tests und ein manueller Dry-Run**

Run:
```bash
go vet ./... && go test ./...
cat > /tmp/settings.json <<'JSON'
{"model":"epd_7_in_5_v2_colour","update_interval":60,"orientation":0,"info_section":true,"info_section_height":40,
 "calibration_hours":[],"border_around_modules":true,
 "modules":[{"position":1,"name":"Agenda","config":{"size":[480,760],
   "ical_urls":"https://www.officeholidays.com/ics-fed/usa","ical_files":null,
   "date_format":"ddd D MMM","time_format":"HH:mm","padding_x":10,"padding_y":10,"fontsize":14,"language":"de"}}]}
JSON
go run ./cmd/inkycal --mode dry-run --settings /tmp/settings.json --out /tmp/preview.png
```
Expected: alle Pakete `ok`; Log `preview written path=/tmp/preview.png`. `/tmp/preview.png` öffnen: 800×480 (Panel-Sicht, also um 90° gedreht), rote Datumslinien, Feiertage als Termine.

- [ ] **Step 5: Commit**

```bash
git add cmd
git commit -m "feat(cmd): inkycal CLI with run/once/dry-run/clear"
```

---

### Task 14: Deployment, Hardware-Verifikation, Wachzeit-Messung

**Files:**
- Create: `Makefile`, `deploy/inkycal-go.service`, `deploy/README.md`

**Interfaces:**
- Consumes: Binary aus Task 13
- Produces: `make test`, `make build-arm64`, `make build-armv7`, `make deploy PI=… ARCH=arm64|armv7`; systemd-Unit `inkycal-go.service` (`StateDirectory=inkycal` → `/var/lib/inkycal`)

- [ ] **Step 1: Dateien anlegen**

`Makefile` (Einrückung mit Tabs!):

```make
BIN     := inkycal
LDFLAGS := -s -w
PI      ?= pi@inkycal.local
ARCH    ?= arm64

.PHONY: test build-arm64 build-armv7 deploy

test:
	go vet ./...
	go test ./...

build-arm64:
	CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -trimpath -ldflags "$(LDFLAGS)" -o dist/$(BIN)-arm64 ./cmd/inkycal

build-armv7:
	CGO_ENABLED=0 GOOS=linux GOARCH=arm GOARM=7 go build -trimpath -ldflags "$(LDFLAGS)" -o dist/$(BIN)-armv7 ./cmd/inkycal

# make deploy PI=pi@192.168.1.50 ARCH=armv7
deploy: build-$(ARCH)
	scp dist/$(BIN)-$(ARCH) $(PI):/tmp/inkycal
	scp deploy/inkycal-go.service $(PI):/tmp/inkycal-go.service
	ssh $(PI) 'sudo install -m 0755 /tmp/inkycal /usr/local/bin/inkycal && sudo install -m 0644 /tmp/inkycal-go.service /etc/systemd/system/inkycal-go.service && sudo systemctl daemon-reload'
```

`deploy/inkycal-go.service`:

```ini
[Unit]
Description=Inkycal e-paper dashboard (Go)
Wants=network-online.target
After=network-online.target pisugar-server.service

[Service]
Type=simple
ExecStart=/usr/local/bin/inkycal --mode run
StateDirectory=inkycal
Restart=on-failure
RestartSec=30

[Install]
WantedBy=multi-user.target
```

`deploy/README.md`:

````markdown
# Inkycal-Go auf dem Pi installieren

Voraussetzungen: Pi Zero 2 W mit Raspberry Pi OS (Lite empfohlen), PiSugar mit laufendem `pisugar-server`, Waveshare 7,5″ V2 (Farbe) am HAT, SPI aktiviert.

## 0. Baseline messen (Python, vorher!)

Auf dem Pi mit dem alten Python-Inkycal ein paar Zyklen laufen lassen, dann:

```bash
systemd-analyze                              # Boot-Dauer bis multi-user
journalctl -b -1 -o short-monotonic | grep -iE "inkycal|reached target|power-off" | head -40
```

Daraus die Zeit „Boot → Inkycal-Start → Power-off“ in die Tabelle unten eintragen. Das ist der Vergleichswert.

| Messung | Python | Go |
|---|---|---|
| Boot bis zum App-Start | | |
| App-Start bis Power-off | | |
| Wachzeit gesamt | | |

## 1. Architektur prüfen

```bash
uname -m   # aarch64 → ARCH=arm64, armv7l → ARCH=armv7
```

## 2. Altes Inkycal stoppen

```bash
systemctl list-units --all | grep -i inky   # Namen des Python-Dienstes finden
sudo systemctl disable --now <name>.service
```

Sonst blockieren sich beide Programme gegenseitig die GPIO-Pins.

## 3. SPI aktivieren (falls nicht schon an)

```bash
sudo raspi-config nonint do_spi 0
ls /dev/spidev0.0
```

## 4. Bauen und kopieren (auf dem Entwicklungsrechner)

```bash
make deploy PI=pi@<ip-des-pi> ARCH=arm64
```

## 5. settings.json ergänzen

Die bestehende `/boot/firmware/settings.json` (oder `/boot/settings.json`) bleibt. Diese Schlüssel kommen dazu bzw. werden gesetzt:

```json
"use_pi_sugar": true,
"shutdown_after_run": true,
"image_hash": true
```

## 6. Erst testen, dann scharf schalten

```bash
sudo touch /boot/firmware/inkycal-maintenance              # Notausgang: nie ausschalten
inkycal --mode dry-run --out /tmp/preview.png              # ohne Display
sudo inkycal --mode once                                   # einmal aufs Display
sudo systemctl enable --now inkycal-go.service
journalctl -u inkycal-go -f                                # Logs ansehen
sudo rm /boot/firmware/inkycal-maintenance                 # ab jetzt Akku-Modus
```

## Notausgang

Der Pi ist nur noch Sekunden wach. Um wieder per SSH dranzukommen:

- SD-Karte in den PC stecken und auf der Boot-Partition eine leere Datei `inkycal-maintenance` anlegen, **oder**
- USB-Strom am PiSugar anstecken. Dann schaltet sich der Pi nicht aus und aktualisiert im Intervall weiter.

## Messwerte

Jeder Zyklus hängt eine Zeile an `/var/lib/inkycal/timings.csv` an:

```
zeit,boot_s,module_s,render_s,display_s,gesamt_s,refreshed
```

`boot_s` ist die Zeit vom Kernel-Start bis zum App-Start. Mit `journalctl -b -1 -o short-monotonic | tail` siehst du, wann der Pi sich abgeschaltet hat.
````

- [ ] **Step 2: Builds prüfen**

Run: `make test && make build-arm64 build-armv7 && ls -la dist/`
Expected: Tests `ok`, zwei Binaries `inkycal-arm64` und `inkycal-armv7` (je ~9 MB)

- [ ] **Step 3: Commit**

```bash
git add Makefile deploy
git commit -m "build: cross-compile, systemd unit and install guide"
```

- [ ] **Step 4: Baseline messen (Python) – auf dem Pi, mit dem Menschen**

Abschnitt „0. Baseline messen“ aus `deploy/README.md` ausführen und die Python-Werte in die Tabelle eintragen. Ohne diese Zahl ist der Erfolg des Rewrites nicht belegbar.

- [ ] **Step 5: Installation und Hardware-Test – auf dem Pi, mit dem Menschen**

Abschnitte 1–6 aus `deploy/README.md` befolgen. Verifizieren und Ergebnis notieren:
- `sudo inkycal --mode once` → Display zeigt die Agenda mit roten Linien; Log enthält `cycle done … refreshed=true`.
- zweites `sudo inkycal --mode once` direkt danach (mit `"image_hash": true`) → Log `image unchanged, skipping refresh`, Display bleibt unverändert.
- `sudo inkycal --mode clear` → Display flackert schwarz/rot/weiß.
- Dienst aktiv, Wartungsdatei entfernt → Pi schaltet sich nach dem Refresh ab und wacht zur nächsten Intervallgrenze von selbst wieder auf (`get rtc_alarm_time` über `echo "get rtc_alarm_time" | nc -q 0 127.0.0.1 8423` vor dem Abschalten prüfen).

- [ ] **Step 6: Go-Wachzeit messen und eintragen**

Nach mindestens drei automatischen Zyklen: `cat /var/lib/inkycal/timings.csv` und `journalctl -b -1 -o short-monotonic | tail -20`. Go-Spalte der Tabelle in `deploy/README.md` füllen, committen:

```bash
git add deploy/README.md
git commit -m "docs: measured wake time python vs go"
```

Ergebnis an den Menschen berichten: Boot-Anteil vs. App-Anteil. Ist der Boot-Anteil > 10 s, ist Phase 3 (OS verschlanken) der nächste Hebel – nicht weiterer Go-Code.

