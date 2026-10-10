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
