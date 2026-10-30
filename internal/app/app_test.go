package app

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
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
	last          render.Frame
}

func (d *fakeDisplay) Size() (int, int)          { return 800, 480 }
func (d *fakeDisplay) Show(f render.Frame) error { d.shows++; d.last = f; return d.err }
func (d *fakeDisplay) Clear(cycles int) error    { d.clears += cycles; return nil }

type fakePiSugar struct {
	alarms           []time.Time
	alarmErr         error
	failAfter        int // SetAlarm fails once more than failAfter alarms were set (0 = never)
	plugged          bool
	rtcToPi, piToRTC int
}

func (p *fakePiSugar) Battery() (float64, error)   { return 81, nil }
func (p *fakePiSugar) PowerPlugged() (bool, error) { return p.plugged, nil }
func (p *fakePiSugar) RTCToPi() error              { p.rtcToPi++; return nil }
func (p *fakePiSugar) PiToRTC() error              { p.piToRTC++; return nil }
func (p *fakePiSugar) SetAlarm(t time.Time, r int) error {
	p.alarms = append(p.alarms, t)
	if p.failAfter > 0 && len(p.alarms) > p.failAfter {
		return errors.New("i2c timeout")
	}
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
	"Dot":   func(c config.Common, _ json.RawMessage) (modules.Module, error) { return dotModule{c}, nil },
	"Fail":  func(config.Common, json.RawMessage) (modules.Module, error) { return failModule{}, nil },
	"Panic": func(config.Common, json.RawMessage) (modules.Module, error) { panic("broken module") },
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

func TestNoModulesShowsAHint(t *testing.T) {
	o, d, _, _ := testOptions(t, "Dot")
	o.Settings.Modules = nil
	o.Settings.InfoSection = false // only the hint may ink the panel
	RunCycle(context.Background(), o)
	inked := false
	for _, b := range d.last.Black {
		inked = inked || b != 0xFF
	}
	if d.shows != 1 || !inked {
		t.Errorf("shows=%d inked=%v: an empty settings.json must not leave a blank panel", d.shows, inked)
	}
}

func TestDisableCalibration(t *testing.T) {
	o, d, _, _ := testOptions(t, "Dot")
	o.Settings.CalibrationHours = []int{10}
	o.Settings.DisableCalibration = true
	RunCycle(context.Background(), o)
	if d.clears != 0 {
		t.Errorf("disable_calibration must skip calibration, ran %d", d.clears)
	}
}

func TestInfoText(t *testing.T) {
	now := time.Date(2026, 10, 10, 9, 5, 0, 0, time.UTC)
	oc := []ModuleStatus{{}, {Err: errors.New("x")}, {Stale: true}}
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

func TestRunCycleBadModuleSizeDoesNotPanic(t *testing.T) {
	o, d, _, _ := testOptions(t, "Dot", "Dot")
	o.Settings.Modules[1].Config = json.RawMessage(`{"size":[480,10],"padding_x":10,"padding_y":10}`)
	o.Settings.Modules = append(o.Settings.Modules, config.ModuleEntry{Position: 3, Name: "Dot", Config: json.RawMessage(`{"padding_x":10}`)})
	res, err := RunCycle(context.Background(), o)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.ModuleErrors) != 2 || d.shows != 1 || !res.PowerOff {
		t.Errorf("errors=%v shows=%d poweroff=%v", res.ModuleErrors, d.shows, res.PowerOff)
	}
}

// testClock is a goroutine-safe fake clock (the display runs in its own goroutine).
type testClock struct {
	mu sync.Mutex
	t  time.Time
}

func (c *testClock) now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.t
}

func (c *testClock) advance(d time.Duration) {
	c.mu.Lock()
	c.t = c.t.Add(d)
	c.mu.Unlock()
}

// slowDisplay advances the clock while "refreshing", like a calibration run.
type slowDisplay struct {
	*fakeDisplay
	clock *testClock
	took  time.Duration
}

func (d *slowDisplay) Show(f render.Frame) error {
	d.clock.advance(d.took)
	return d.fakeDisplay.Show(f)
}

func TestAlarmStillInFutureAfterSlowDisplay(t *testing.T) {
	o, _, p, _ := testOptions(t, "Dot")
	clk := &testClock{t: time.Date(2026, 10, 10, 12, 58, 0, 0, time.UTC)}
	o.Now = clk.now
	o.Display = &slowDisplay{fakeDisplay: &fakeDisplay{}, clock: clk, took: 3 * time.Minute}
	res, err := RunCycle(context.Background(), o)
	if err != nil {
		t.Fatal(err)
	}
	last := p.alarms[len(p.alarms)-1]
	if last.Sub(clk.now()) < minSleep {
		t.Errorf("alarm %s is not %s after the cycle end %s", last, minSleep, clk.now())
	}
	if !res.Wake.Equal(last) || !res.PowerOff {
		t.Errorf("wake=%s alarm=%s poweroff=%v", res.Wake, last, res.PowerOff)
	}
}

func TestAlarmResetFailureKeepsDeviceOn(t *testing.T) {
	o, _, p, _ := testOptions(t, "Dot")
	clk := &testClock{t: time.Date(2026, 10, 10, 12, 58, 0, 0, time.UTC)}
	o.Now = clk.now
	o.Display = &slowDisplay{fakeDisplay: &fakeDisplay{}, clock: clk, took: 3 * time.Minute}
	p.failAfter = 1 // first alarm succeeds, the corrected one fails
	res, _ := RunCycle(context.Background(), o)
	if res.PowerOff {
		t.Error("must not power off when the corrected alarm could not be set")
	}
}

func TestCalibrateForcesNextRefresh(t *testing.T) {
	o, d, _, _ := testOptions(t, "Dot")
	RunCycle(context.Background(), o)
	if err := Calibrate(o); err != nil {
		t.Fatal(err)
	}
	res, _ := RunCycle(context.Background(), o)
	if !res.Refreshed || d.shows != 2 || d.clears != 1 {
		t.Errorf("refreshed=%v shows=%d clears=%d", res.Refreshed, d.shows, d.clears)
	}
}

func TestInfoTextRoundsBatteryWithImageHash(t *testing.T) {
	now := time.Date(2026, 10, 10, 9, 5, 0, 0, time.UTC)
	if got := infoText(now, true, nil, 87.4, true); got != "Battery: 80%" {
		t.Errorf("image_hash on: %q", got)
	}
	if got := infoText(now, false, nil, 87.4, true); got != "10 Oct @ 09:05  Battery: 87%" {
		t.Errorf("image_hash off: %q", got)
	}
}

func TestErrorHint(t *testing.T) {
	for _, c := range []struct {
		name, err, lang, want string
	}{
		{"Todoist", `todoist: projects: GET https://api.todoist.com/…: HTTP 401: {"error":"Unauthorized"}`, "de", "Todoist: Zugang abgelehnt – API-Schlüssel in settings.json prüfen"},
		{"Weather", `weather: current: Get "https://api.openweathermap.org/…": dial tcp: lookup api.openweathermap.org: no such host`, "de", "Wetter: keine Verbindung"},
		{"Calendar", "calendar: calendar 2: boom", "de", "Kalender: Fehler – Details mit journalctl -u inkycal-go"},
		{"Calendar", "boom", "en", "Calendar: error – see journalctl -u inkycal-go"},
		{"Weather", "weather: latitude and longitude are missing (search the place)", "de", "Wetter: noch nicht eingerichtet – im Builder ausfüllen"},
		{"Agenda", "agenda: ical_urls is missing", "en", "Agenda: not set up yet – fill it in with the builder"},
		{"Widget", `unknown module "Widget"`, "de", "Widget: dieses Modul gibt es nicht mehr – Kachel entfernen"},
		{"Calendar", "calendar: https://x/basic.ics: not an iCalendar file (got HTML)", "de", "Kalender: Adresse liefert eine Webseite statt eines Kalenders – iCal-Adresse verwenden"},
		{"Calendar", "calendar: https://x/basic.ics: not an iCalendar file (got HTML)", "en", "Calendar: the address shows a web page, not a calendar – use the iCal address"},
		{"Weather", "weather: inner size 120x40 too small (min 200x100)", "de", "Wetter: Kachel zu klein – im Builder vergrößern"},
		{"Weather", "weather: inner size 120x40 too small (min 200x100)", "en", "Weather: tile too small – make it bigger in the builder"},
		{"Weather", "Weather: style: scale 3: want 0.5–2", "de", "Wetter: ungültiger Stil – im Builder neu wählen"},
		{"Weather", "Weather: style: scale 3: want 0.5–2", "en", "Weather: invalid style – choose it again in the builder"},
	} {
		if got := errorHint(c.name, errors.New(c.err), c.lang); got != c.want {
			t.Errorf("%s %q:\n got %q\nwant %q", c.name, c.err, got, c.want)
		}
	}
}

func TestFailedModuleShowsHint(t *testing.T) {
	o, _, _, _ := testOptions(t, "Dot", "Fail", "Missing", "Panic")
	out := runModules(context.Background(), o, o.Now(), nil)
	for i := 1; i < len(out); i++ {
		img := out[i].part.Img
		inked := false
		for y := 0; y < img.H && !inked; y++ {
			for x := 0; x < img.W; x++ {
				if b, _ := img.InkAt(x, y); b {
					inked = true
					break
				}
			}
		}
		if out[i].err == nil || !inked {
			t.Errorf("module %d: err=%v, hint drawn=%v", i, out[i].err, inked)
		}
	}
}

func TestRunCycleUsesUpdateTimes(t *testing.T) {
	o, _, p, _ := testOptions(t, "Dot") // now = 10:07
	o.Settings.UpdateTimes = config.StringList{"06:00", "12:00"}
	res, err := RunCycle(context.Background(), o)
	if err != nil {
		t.Fatal(err)
	}
	want := time.Date(2026, 10, 10, 12, 0, 0, 0, time.UTC)
	if !res.Wake.Equal(want) || len(p.alarms) != 1 || !p.alarms[0].Equal(want) {
		t.Errorf("wake %v alarms %v, want %v", res.Wake, p.alarms, want)
	}
}

// preparingDisplay records the order of driver calls.
type preparingDisplay struct {
	*fakeDisplay
	mu  sync.Mutex
	log []string
}

func (d *preparingDisplay) add(s string) { d.mu.Lock(); d.log = append(d.log, s); d.mu.Unlock() }
func (d *preparingDisplay) Prepare() error {
	d.add("prepare")
	return nil
}
func (d *preparingDisplay) Release() error { d.add("release"); return nil }
func (d *preparingDisplay) Show(f render.Frame) error {
	d.add("show")
	return d.fakeDisplay.Show(f)
}

func TestRunCyclePreparesPanelDuringFetch(t *testing.T) {
	o, _, _, _ := testOptions(t, "Dot")
	d := &preparingDisplay{fakeDisplay: &fakeDisplay{}}
	o.Display = d
	RunCycle(context.Background(), o)
	if strings.Join(d.log, ",") != "prepare,show" {
		t.Errorf("first cycle: %v", d.log)
	}
	// Unchanged image: the prepared panel goes back to sleep.
	d.log = nil
	RunCycle(context.Background(), o)
	if strings.Join(d.log, ",") != "prepare,release" {
		t.Errorf("unchanged cycle: %v", d.log)
	}
}

type fakeNetwork struct {
	up             bool
	waits, off, on int
	offErr         error
}

func (n *fakeNetwork) Wait(context.Context, time.Duration) bool { n.waits++; return n.up }
func (n *fakeNetwork) RadioOff() error                          { n.off++; return n.offErr }
func (n *fakeNetwork) RadioOn() error                           { n.on++; return nil }

func TestRunCycleWaitsForNetworkAndSwitchesRadioOffBeforePowerOff(t *testing.T) {
	o, d, _, _ := testOptions(t, "Dot")
	n := &fakeNetwork{up: true}
	o.Network = n
	res, _ := RunCycle(context.Background(), o)
	if n.waits != 1 || n.off != 1 || n.on != 1 || !res.PowerOff || d.shows != 1 { // on: at the start
		t.Errorf("waits=%d off=%d on=%d poweroff=%v shows=%d", n.waits, n.off, n.on, res.PowerOff, d.shows)
	}
	// No network: the cycle still runs (modules fall back to their cache).
	n2 := &fakeNetwork{}
	o.Network = n2
	o.Settings.ImageHash = false
	if res, _ := RunCycle(context.Background(), o); !res.Refreshed {
		t.Error("cycle without network did not refresh")
	}
}

func TestRadioStaysOnWhenDeviceStaysOn(t *testing.T) {
	cases := map[string]func(*Options, *fakePiSugar, *fakeSystem){
		"maintenance": func(o *Options, p *fakePiSugar, s *fakeSystem) { s.maintenance = true },
		"usb power":   func(o *Options, p *fakePiSugar, s *fakeSystem) { p.plugged = true },
		"alarm fails": func(o *Options, p *fakePiSugar, s *fakeSystem) { p.alarmErr = errors.New("i2c") },
		"disabled":    func(o *Options, p *fakePiSugar, s *fakeSystem) { o.Settings.ShutdownAfterRun = false },
		"no pisugar":  func(o *Options, p *fakePiSugar, s *fakeSystem) { o.PiSugar = nil },
	}
	for name, mutate := range cases {
		o, _, p, s := testOptions(t, "Dot")
		n := &fakeNetwork{up: true}
		o.Network = n
		mutate(o, p, s)
		RunCycle(context.Background(), o)
		if n.off != 0 {
			t.Errorf("%s: radio switched off", name)
		}
	}
	// Decided off during the refresh, then the corrected alarm fails: radio back on.
	o, _, p, _ := testOptions(t, "Dot")
	n := &fakeNetwork{up: true}
	o.Network = n
	clk := &testClock{t: time.Date(2026, 10, 10, 12, 58, 0, 0, time.UTC)}
	o.Now = clk.now
	o.Display = &slowDisplay{fakeDisplay: &fakeDisplay{}, clock: clk, took: 3 * time.Minute}
	p.failAfter = 1
	if res, _ := RunCycle(context.Background(), o); res.PowerOff || n.off != 1 || n.on != 2 {
		t.Errorf("poweroff=%v off=%d on=%d", res.PowerOff, n.off, n.on)
	}
}

// A cycle that fails after the panel was woken ahead of time must put it back to sleep:
// otherwise it stays powered, its bus open, until the next wake.
func TestRunCycleReleasesPreparedPanelOnError(t *testing.T) {
	o, _, _, _ := testOptions(t, "Dot", "Dot")
	o.Settings.Modules[0].Config = json.RawMessage(`{"size":[480,200],"cell":[0,0,2,2]}`) // the second has none
	d := &preparingDisplay{fakeDisplay: &fakeDisplay{}}
	o.Display = d
	if _, err := RunCycle(context.Background(), o); err == nil {
		t.Fatal("want a grid error")
	}
	if got := strings.Join(d.log, ","); got != "prepare,release" {
		t.Errorf("driver calls %q, want prepare,release", got)
	}
}

// A tile Load could not use fails on its own: the rest of the page is still shown.
func TestBrokenTileDoesNotStopThePanel(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "settings.json")
	os.WriteFile(p, []byte(`{"model":"epd_7_in_5_v2_colour","modules":[
		{"position":1,"name":"Dot","config":{"cell":[0,0,12,4],"padding_x":10,"padding_y":10}},
		{"position":2,"name":"Dot","config":{"cell":[0,4,12,4],"padding_x":10,"padding_y":10,"style":{"scale":3}}},
		{"position":3,"name":"Dot","config":{"cell":[0,2,12,4],"padding_x":10,"padding_y":10}}]}`), 0o600)
	s, err := config.Load(p)
	if err != nil {
		t.Fatal(err)
	}
	o, d, _, _ := testOptions(t)
	o.Settings = s
	res, err := RunCycle(context.Background(), o)
	if err != nil {
		t.Fatalf("cycle failed: %v", err)
	}
	if d.shows != 1 || len(res.ModuleErrors) != 2 {
		t.Fatalf("shows %d, module errors %q", d.shows, res.ModuleErrors)
	}
	if !strings.Contains(res.ModuleErrors[0], "style") || !strings.Contains(res.ModuleErrors[1], "overlaps") {
		t.Errorf("module errors %q", res.ModuleErrors)
	}
}

// Nothing placeable: the panel says so instead of going blank.
func TestNoPlaceableTileShowsHint(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "settings.json")
	os.WriteFile(p, []byte(`{"model":"epd_7_in_5_v2_colour","modules":[
		{"position":1,"name":"Dot","config":{"cell":[0,0,99,4]}}]}`), 0o600)
	s, err := config.Load(p)
	if err != nil {
		t.Fatal(err)
	}
	o, _, _, _ := testOptions(t)
	o.Settings = s
	c, err := Compose(context.Background(), o, o.Now())
	if err != nil {
		t.Fatal(err)
	}
	if b, _ := modulestestInk(c.Canvas); !b {
		t.Error("blank page: want a hint")
	}
}

// A broken style is not applied to the error tile: its colours could paint the tile red.
func TestBrokenStyleIsNotApplied(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "settings.json")
	os.WriteFile(p, []byte(`{"model":"epd_7_in_5_v2_colour","modules":[
		{"position":1,"name":"Dot","config":{"cell":[0,0,12,8],"padding_x":10,"padding_y":10,"style":{"font":"comic","colors":{"paper":"colour"}}}}]}`), 0o600)
	s, err := config.Load(p)
	if err != nil {
		t.Fatal(err)
	}
	o, _, _, _ := testOptions(t)
	o.Settings = s
	c, err := Compose(context.Background(), o, o.Now())
	if err != nil {
		t.Fatal(err)
	}
	if _, colour := modulestestInk(c.Canvas); colour {
		t.Error("the broken style's colour paper was applied")
	}
}

func modulestestInk(c *render.Canvas) (black, colour bool) {
	for y := 0; y < c.H; y++ {
		for x := 0; x < c.W; x++ {
			b, col := c.InkAt(x, y)
			black, colour = black || b, colour || col
		}
	}
	return black, colour
}
