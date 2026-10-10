# Inkycal-Go – Design

**Datum:** 2026-10-10
**Status:** Entwurf zur Freigabe

## 1. Ziel

Ein schlanker Go-Rewrite von [Inkycal](https://github.com/aceinnolab/Inkycal) für **einen** Aufbau:

- Raspberry Pi Zero 2 W mit PiSugar (RTC-Wecker, Akku)
- Waveshare 7,5″ V2 dreifarbig (`epd_7_in_5_v2_colour`, 800×480, Schwarz/Weiß/Rot)
- Module: **Agenda**, **Calendar**, **Weather**, **Todoist**, **Feeds**

Das einzige Optimierungsziel ist die **Wachzeit pro Update**, also die Zeit vom Einschalten durch die RTC bis zum Abschalten. Laut Inkycal-Doku liegt sie heute bei etwa 3,5 Minuten. Das Ziel sind unter 45 s auf einem Standard-Raspberry-Pi-OS-Lite und unter 30 s nach der OS-Optimierung (Phase 3).

## 2. Warum der Rewrite überhaupt Wachzeit spart

| Ursache im Python-Code | Kosten | Lösung in Go |
|---|---|---|
| Interpreter-Start plus Imports (PIL, numpy, arrow, requests, icalendar …) | 10–40 s | Statisches Binary, Start < 50 ms |
| `internet_available()` pro Modul: bis zu 3 Requests an google.com mit `sleep(5)` (`inkycal/utils/functions.py:69`) | 1–15 s | Entfällt. Echte Fehler führen zu Retry und Cache-Fallback. |
| Module laufen nacheinander | Summe aller Netzwerk-Latenzen | Alle Module laufen parallel (Goroutinen) |
| `time.sleep(5)` vor dem Shutdown plus nochmal `sleep(5)` in `_shutdown_system` (`inkycal/main.py:455`, `:621`) | 10 s | Entfällt |
| Bild-Hashes werden bei jedem Start gelöscht (`_remove_hashes` im Konstruktor). Nach einem Reboot wird also **immer** refresht, auch wenn sich nichts geändert hat. | 16–20 s Refresh plus Display-Strom | Der Hash bleibt im State-File; „unverändert“ heißt kein Refresh |
| `ReadBusy` pollt ohne Pause | CPU zu 100 % während des Refreshs | Pollt im 10-ms-Takt |
| `rtc_pi2rtc` wird beim Start immer aufgerufen, auch wenn die Systemzeit noch nicht per NTP synchronisiert ist. Eine falsche Zeit kann in die RTC wandern und damit einen falschen Wecker setzen. | Korrektheit | Beim Start `rtc_rtc2pi`; `rtc_pi2rtc` nur, wenn der Kernel „NTP synchronisiert“ meldet (adjtimex) |

Fix bleibt: Linux-Boot (Phase 3) und der Refresh des Farb-Panels (~16–20 s, Physik).

## 3. Nicht-Ziele

- Keine Web-UI. Konfiguriert wird über `settings.json` auf der Boot-Partition, kompatibel zum Python-Format.
- Keine weiteren Displays und keine weiteren Module (Stocks, Webshot, xkcd, Fullweather …). Die Architektur erlaubt sie später.
- Die Ausgabe muss nicht pixelgleich zu Python sein. Ziel ist dasselbe Layout und dieselben Inhalte.
- Kein Startup-Splash. Er kostet einen kompletten Refresh.

## 4. Architektur

```
cmd/inkycal/main.go           CLI: --mode run|once|dry-run|clear
internal/config/              settings.json (Python-kompatibel), Flex-Typen
internal/timefmt/             Arrow-Token-Formatierung + Locales (en, de)
internal/render/              Canvas (2 Ebenen), Text, Icons, Formen, Assemble, Pack
internal/render/fonts/        eingebettete Fonts (go:embed)
internal/epd/                 Display-Interface, 7.5" V2 colour Treiber, PNG-Display, Linux-Bus
internal/fetch/               HTTP-Client mit Retry + Disk-Cache-Fallback
internal/ical/                iCal laden/expandieren (gocal-Wrapper)
internal/modules/             Module-Interface + Env
internal/modules/agenda/      (Plan 1)
internal/modules/weather/     (Plan 2)
internal/modules/calendar/    (Plan 2)
internal/modules/todoist/     (Plan 2)
internal/modules/feeds/       (Plan 2)
internal/state/               state.json (Hash, Counter, letzte Kalibrierung)
internal/schedule/            nächster Weckzeitpunkt, Kalibrierungs-Fälligkeit
internal/pisugar/             TCP-Client für pisugar-server (127.0.0.1:8423)
internal/sysinfo/             NTP-Status, Uptime, PowerOff, Wartungs-Flag (Linux)
internal/app/                 Zyklus-Orchestrierung, Registry, Timings
deploy/                       systemd-Unit, Installationsanleitung
```

### Ablauf eines Zyklus (`app.RunCycle`)

1. `t0`: Uptime lesen (Zeit vom Boot bis zum Start der App) und loggen.
2. PiSugar: Ist NTP nicht synchronisiert, `rtc_rtc2pi` aufrufen. Akkustand lesen.
3. Alle Module **parallel** ausführen (`Generate`, jedes mit eigenem Canvas). Gesamt-Deadline 60 s; Modulfehler werden gesammelt.
4. Assemble: Module untereinander, jeweils zentriert in ihrer Section, plus Info-Zeile unten.
5. Pack: ins Panel-Format drehen, in Bit-Ebenen packen, Hash bilden.
6. Ist `image_hash` aktiv und der Hash gleich dem letzten, wird **nicht refresht**. Sonst Kalibrierung (falls fällig), dann `Display.Show` **in einer Goroutine**.
7. Parallel zum Refresh: nächste Weckzeit berechnen, `rtc_pi2rtc` (nur wenn NTP synchronisiert), `rtc_alarm_set`, State speichern.
8. Auf das Display warten, Timings loggen und in `timings.csv` anhängen.
9. Ausschalten nur, wenn **alle** Bedingungen gelten: PiSugar aktiv, `shutdown_after_run` an, Wecker erfolgreich gesetzt, kein Wartungs-Flag, kein USB-Strom. Sonst schläft der Prozess bis zur nächsten Weckzeit und macht im Loop weiter.

### Notausgang (wichtig bei kurzer Wachzeit)

Bei ~30 s Wachzeit kommt man nicht mehr per SSH rein. Deshalb gibt es **kein Ausschalten**, wenn:
- die Datei `/boot/firmware/inkycal-maintenance` oder `/boot/inkycal-maintenance` existiert (am PC auf die SD-Karte legbar), oder
- PiSugar `battery_power_plugged: true` meldet, also USB-Strom angeschlossen ist.

## 5. Verbindliche Schnittstellen

Alle Pläne halten sich exakt an diese Signaturen.

### `internal/config`

```go
type FlexInt int        // JSON: 5, "5", null(→0)
type FlexBool bool      // JSON: true, false, "true", "True", "false", "False", null(→false)
type StringList []string // JSON: "a, b" (split ",", trim, leere raus), ["a","b"], null

type Settings struct {
	Model               string        `json:"model"`
	UpdateInterval      FlexInt       `json:"update_interval"`     // Minuten, Default 60
	Orientation         FlexInt       `json:"orientation"`         // 0 oder 180
	InfoSection         FlexBool      `json:"info_section"`
	InfoSectionHeight   FlexInt       `json:"info_section_height"` // Default 70 wenn InfoSection
	CalibrationHours    []int         `json:"calibration_hours"`
	CalibrationCycles   FlexInt       `json:"calibration_cycles"`  // Default 3 (neu, optional)
	BorderAroundModules FlexBool      `json:"border_around_modules"`
	ImageHash           FlexBool      `json:"image_hash"`
	UsePiSugar          FlexBool      `json:"use_pi_sugar"`
	ShutdownAfterRun    FlexBool      `json:"shutdown_after_run"`
	Modules             []ModuleEntry `json:"modules"` // nach Load: sortiert nach Position
}

type ModuleEntry struct {
	Position int             `json:"position"`
	Name     string          `json:"name"`
	Config   json.RawMessage `json:"config"`
}

type Common struct {
	Size     [2]int  `json:"size"`
	PaddingX FlexInt `json:"padding_x"`
	PaddingY FlexInt `json:"padding_y"`
	FontSize FlexInt `json:"fontsize"` // Default 14
	Language string  `json:"language"` // Default "en"
}

func (c Common) InnerSize() (w, h int)          // Size - 2*Padding
func ParseCommon(raw json.RawMessage) (Common, error) // inkl. Defaults
func Load(path string) (*Settings, error)        // inkl. Defaults + Validierung
func FindSettings(explicit string) (string, error)
```

### `internal/timefmt`

```go
// Format formatiert t mit Arrow-Tokens (YYYY YY MMMM MMM MM M DD D dddd ddd HH H hh h mm m ss s A a, [literal]).
func Format(t time.Time, layout, lang string) string
// HumanizeDays: "3 days"/"3 Tage", "a week"/"eine Woche", "2 weeks"/"2 Wochen".
func HumanizeDays(days int, lang string) string
```

### `internal/render`

```go
const InkThreshold = 220 // Pixelwert <= 220 gilt als Tinte (wie Pythons _optimize_im)

type FontID int
const (
	FontDefault  FontID = iota // NotoSans-SemiCondensedMedium
	FontRegular                // NotoSans-SemiCondensed
	FontWeather                // weathericons-regular-webfont
	FontMaterial               // MaterialIcons
)

type Ink int
const (
	InkBlack  Ink = iota // nur Ebene Black
	InkColour            // nur Ebene Colour (wird rot)
	InkWhite             // weiß auf beiden Ebenen (z.B. Zahl im "heute"-Kreis)
)

type Align int
const (
	AlignCenter Align = iota
	AlignLeft
	AlignRight
)

type TextOpts struct {
	Align      Align
	Ink        Ink
	Autofit    bool    // Schriftgröße so wählen, dass "Ag" ~FillHeight*h hoch ist; ändert NICHT die Canvas-Schrift
	FillHeight float64 // 0 → 0.8
}

type Canvas struct {
	W, H   int
	Black  *image.Gray // 255 = weiß
	Colour *image.Gray // 255 = weiß
	// unexportiert: aktuelle Schrift, Face-Cache (Faces sind nicht thread-safe → pro Canvas)
}

func NewCanvas(w, h int, font FontID, size int) *Canvas
func (c *Canvas) SetFont(id FontID, size int)
func (c *Canvas) Font() (FontID, int)
func (c *Canvas) LineHeight() int                 // ascent+descent der aktuellen Schrift
func (c *Canvas) TextWidth(s string) int          // Advance-Breite
func (c *Canvas) Wrap(text string, maxWidth int) []string
func (c *Canvas) Write(x, y, w, h int, text string, o TextOpts)                    // w<=0 || h<=0 → no-op
func (c *Canvas) Icon(x, y, w, h int, glyph string, font FontID, ink Ink, fillRatio float64) // w<=0 || h<=0 → no-op
func (c *Canvas) HLine(x0, x1, y int, ink Ink)
func (c *Canvas) VLine(x, y0, y1 int, ink Ink)
func (c *Canvas) RoundedRect(x, y, w, h, r int, ink Ink) // Kontur, 1px
func (c *Canvas) FillCircle(cx, cy, r int, ink Ink)
func (c *Canvas) Paste(src *Canvas, x, y int)            // überträgt nur Tinte (<= InkThreshold) beider Ebenen
func (c *Canvas) InkAt(x, y int) (black, colour bool)
// Border ist Pythons draw_border: Rechteck (w,h) um shrinkX/shrinkY verkleinert, zentriert, Ecken mit Radius r.
func Border(c *Canvas, x, y, w, h, r int, shrinkX, shrinkY float64, ink Ink)

type Part struct {
	Img                *Canvas
	SectionW, SectionH int
}
func Assemble(w, h int, parts []Part, border bool, info string, infoHeight int) *Canvas

type Frame struct {
	W, H  int    // native Panelgröße (Querformat)
	Black []byte // 1 bit/px, MSB zuerst, 1 = weiß, 0 = schwarz
	Red   []byte // 1 bit/px, MSB zuerst, 1 = rot,  0 = kein rot
}
func Pack(c *Canvas, panelW, panelH, orientation int) (Frame, error)
func (f Frame) Hash() string
func (f Frame) Preview() *image.RGBA
```

### `internal/epd`

```go
type Display interface {
	Size() (w, h int)          // native Panelgröße
	Show(f render.Frame) error // init → übertragen → refresh → deep sleep
	Clear(cycles int) error    // Kalibrierung
}
func Open(model string) (Display, error)        // Hardware (linux), sonst Fehler
func NewPNG(path string, w, h int) Display      // dry-run: schreibt Preview-PNG
var Models = map[string][2]int{"epd_7_in_5_v2_colour": {800, 480}}
```

### `internal/fetch`

```go
type Client struct {
	HTTP     *http.Client
	CacheDir string        // "" = kein Cache
	Attempts int           // Default 3
	Backoff  time.Duration // Default 500ms, verdoppelt sich
}
type Result struct {
	Body      []byte
	Stale     bool // aus Cache, weil Abruf fehlschlug
	FetchedAt time.Time
}
func New(cacheDir string) *Client
func (c *Client) Get(ctx context.Context, url string, header http.Header) (Result, error)
func (c *Client) GetJSON(ctx context.Context, url string, header http.Header, v any) (stale bool, err error)
```
Retry gibt es bei Netzwerkfehlern und 5xx/429. Bei 4xx wird sofort ein Fehler gemeldet, ohne Cache-Fallback, weil das ein Konfigurationsfehler ist.

### `internal/ical`

```go
type Event struct {
	Title      string
	Start, End time.Time // in loc
	AllDay     bool
}
func Parse(r io.Reader, from, to time.Time, loc *time.Location) ([]Event, error) // überlappend [from,to), sortiert
func Load(ctx context.Context, fc *fetch.Client, urls, files []string, from, to time.Time, loc *time.Location) (events []Event, stale bool, err error)
// Load: URLs parallel, webcal:// → https://, leere Listen → keine Events, Ergebnis sortiert; Ganztags-Termine in loc.
```

### `internal/modules`

```go
type Env struct {
	Now   time.Time      // bereits in Loc
	Loc   *time.Location
	Fetch *fetch.Client
	Stale bool           // Modul setzt true, wenn es Cache-Daten verwendet hat
}
type Module interface {
	Generate(ctx context.Context, env *Env) (*render.Canvas, error) // Canvas hat InnerSize
}
type Factory func(c config.Common, raw json.RawMessage) (Module, error)
```
Die Registry liegt in `internal/app/registry.go` als `var Registry = map[string]modules.Factory{...}`. Die Schlüssel entsprechen den Python-Namen: `"Agenda"`, `"Calendar"`, `"Weather"`, `"Todoist"`, `"Feeds"`.

### `internal/state`, `internal/schedule`, `internal/pisugar`, `internal/sysinfo`, `internal/app`

```go
// state
type State struct {
	LastHash        string    `json:"last_hash"`
	Counter         int       `json:"counter"`
	LastCalibration time.Time `json:"last_calibration"`
}
func Load(dir string) (State, error) // fehlt → Nullwert
func Save(dir string, s State) error // atomar (tmp + fsync + rename)
func AppendTiming(dir string, line string) error

// schedule
func NextWake(now time.Time, intervalMin int, minSleep time.Duration) time.Time
func CalibrationDue(now time.Time, hours []int, last time.Time) bool

// pisugar
type Client struct { Addr string; Timeout time.Duration }
func New(addr string) *Client
func (c *Client) Battery() (float64, error)
func (c *Client) PowerPlugged() (bool, error)
func (c *Client) RTCToPi() error
func (c *Client) PiToRTC() error
func (c *Client) SetAlarm(t time.Time, repeat int) error

// sysinfo (linux; andere OS: Stubs)
func NTPSynced() bool
func Uptime() (time.Duration, error)
func PowerOff() error
func Maintenance() bool

// app
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
	PiSugar  PiSugar // nil = deaktiviert
	System   System
	Fetch    *fetch.Client
	Loc      *time.Location
	Now      func() time.Time
	Registry map[string]modules.Factory
}
type Result struct {
	Refreshed    bool
	ModuleErrors []string
	Wake         time.Time
	PowerOff     bool
	Timings      Timings
}
func RunCycle(ctx context.Context, o *Options) (Result, error)
func Loop(ctx context.Context, o *Options) error
```

## 6. Globale Vorgaben

- Go 1.26 (`go 1.26` in go.mod; `golang.org/x/image v0.47.0` verlangt ≥ 1.26), Modulpfad `inkycal`
- Lizenz **GPL-3.0-only**, weil es ein abgeleitetes Werk von Inkycal (GPL-3.0) ist; die Fonts behalten ihre Lizenzen (OFL 1.1, Apache 2.0)
- `CGO_ENABLED=0`; Ziele `linux/arm64` (64-bit-OS) und `linux/arm` mit `GOARM=7` (32-bit-OS)
- Erlaubte Abhängigkeiten: `golang.org/x/image v0.47.0`, `golang.org/x/sys v0.49.0`, `github.com/warthog618/go-gpiocdev v0.9.2`, `github.com/apognu/gocal v0.9.1`, `github.com/mmcdole/gofeed v1.5.0` (nur Feeds)
- Tests nur mit der Standardbibliothek; kein Netzwerk in Tests (`httptest`)
- Logging mit `log/slog`, Text-Handler auf stderr (landet im journald)
- Pins wie Waveshare: RST=17, DC=25, CS=CE0 (spidev0.0), BUSY=24, PWR=18; SPI 4 MHz, Mode 0

## 7. Phasen

| Phase | Inhalt | Plan |
|---|---|---|
| 0 | Python-Baseline messen (`systemd-analyze`, Journal-Zeitstempel). Die Zahlen kommen ins README. | Teil von Plan 1, Task 15 |
| 1 | Kern, Display, PiSugar, Agenda: ein lauffähiges Gerät | `plans/2026-10-10-inkycal-go-plan1-core.md` |
| 2 | Weather, Calendar, Todoist, Feeds | `plans/2026-10-10-inkycal-go-plan2-modules.md` |
| 3 | OS schlank machen: Lite, Dienste weg, statische IP, Bluetooth und HDMI aus, Read-only-Root, eigene Unit früh im Boot | eigener Plan nach Messung |
| 4 (optional) | Buildroot-Image mit dem Go-Binary als einzigem Dienst | eigener Plan |

## 8. Wachzeit-Budget (Ziel, auf dem Gerät zu verifizieren)

| Abschnitt | Phase 1/2 (Lite-OS) | nach Phase 3 |
|---|---|---|
| Boot bis zum App-Start (`/proc/uptime`) | 15–25 s | 5–8 s |
| Module (parallel, Netzwerk) | ≤ 3 s | ≤ 3 s |
| Rendern + Packen | ≤ 0,5 s | ≤ 0,5 s |
| Display-Refresh (Farbe) + Deep Sleep | 18–22 s (0 bei unverändertem Hash) | gleich |
| Shutdown | 3–5 s | 1–2 s |
