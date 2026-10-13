// Package app runs one dashboard cycle: modules → image → display → RTC alarm → power decision.
package app

import (
	"context"
	"fmt"
	"log/slog"
	"math"
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
	networkWait   = 30 * time.Second // then the modules run on their cache
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

// Network is the radio side of a cycle. The service no longer waits for
// network-online.target: the cycle waits itself, after starting the panel, and
// switches the radio off for the refresh when the device will power off.
type Network interface {
	Wait(ctx context.Context, timeout time.Duration) bool
	RadioOff() error
	RadioOn() error
}

type Options struct {
	Settings *config.Settings
	StateDir string
	Display  epd.Display
	PiSugar  PiSugar // nil = disabled
	System   System
	Network  Network // nil = no waiting, radio untouched
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
		w, h := common.InnerSize()
		// A failed module keeps its section blank, so it needs a canvas even with a broken size.
		out[i].part = render.Part{
			Img:      render.NewCanvas(max(w, 1), max(h, 1), render.FontDefault, 14),
			SectionW: common.Size[0], SectionH: common.Size[1],
		}
		if err != nil {
			out[i].err = fmt.Errorf("%s: %w", entry.Name, err)
			continue
		}
		factory, ok := o.Registry[entry.Name]
		if !ok {
			out[i].err = fmt.Errorf("unknown module %q", entry.Name)
			drawHint(out[i].part.Img, errorHint(entry.Name, out[i].err, common.Language))
			continue
		}
		wg.Add(1)
		go func(i int, entry config.ModuleEntry, common config.Common) {
			defer wg.Done()
			defer func() {
				if r := recover(); r != nil {
					out[i].err = fmt.Errorf("%s panicked: %v", entry.Name, r)
					// The module may have panicked mid-drawing: hint on a fresh canvas.
					img := out[i].part.Img
					out[i].part.Img = render.NewCanvas(img.W, img.H, render.FontDefault, 14)
					drawHint(out[i].part.Img, errorHint(entry.Name, out[i].err, common.Language))
				}
			}()
			m, err := factory(common, entry.Config)
			if err != nil {
				out[i].err = err
				drawHint(out[i].part.Img, errorHint(entry.Name, err, common.Language))
				return
			}
			env := &modules.Env{Now: now, Loc: o.Loc, Fetch: o.Fetch}
			cv, err := m.Generate(ctx, env)
			if err != nil {
				out[i].err = fmt.Errorf("%s: %w", entry.Name, err)
				drawHint(out[i].part.Img, errorHint(entry.Name, err, common.Language))
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
		if imageHash {
			battery = math.Floor(battery/10) * 10 // every 1 % would change the hash and force a refresh
		}
		if battery < 20 {
			fmt.Fprintf(&b, "Low battery! (%.0f%%) ", battery)
		} else {
			fmt.Fprintf(&b, "Battery: %.0f%% ", battery)
		}
	}
	return strings.TrimSpace(b.String())
}

// nextWake uses the fixed update_times when configured, else the update_interval grid.
func nextWake(s *config.Settings, now time.Time) time.Time {
	if times, err := schedule.ParseTimes(s.UpdateTimes); err == nil && len(times) > 0 {
		return schedule.NextWakeAt(now, times, minSleep)
	}
	return schedule.NextWake(now, int(s.UpdateInterval), minSleep)
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
	res.Wake = nextWake(s, now)
	st, err := state.Load(o.StateDir)
	if err != nil {
		slog.Warn("state unreadable, starting fresh", "err", err)
		st = state.State{}
	}

	// Wake and initialise the panel while the network comes up and the modules fetch.
	prepared := make(chan struct{})
	prep, canPrepare := o.Display.(epd.Preparer)
	if canPrepare {
		go func() {
			defer close(prepared)
			if err := prep.Prepare(); err != nil {
				slog.Warn("panel init ahead of the refresh failed, retrying at refresh", "err", err)
			}
		}()
	} else {
		close(prepared)
	}
	if o.Network != nil {
		// A cycle that died after switching the radio off (crash, failed power-off)
		// must not leave the device unreachable: make sure it is on.
		if err := o.Network.RadioOn(); err != nil {
			slog.Debug("switching the radio on failed", "err", err)
		}
		t0 := time.Now()
		if o.Network.Wait(ctx, networkWait) {
			slog.Info("network up", "waited", time.Since(t0).Round(time.Millisecond))
		} else {
			slog.Warn("no network, modules use their cache", "waited", time.Since(t0).Round(time.Millisecond))
		}
	}

	outcomes := runModules(ctx, o, now)
	parts := make([]render.Part, len(outcomes), max(len(outcomes), 1))
	for i, oc := range outcomes {
		parts[i] = oc.part
		if oc.err != nil {
			res.ModuleErrors = append(res.ModuleErrors, oc.err.Error())
			slog.Error("module failed", "position", i+1, "err", oc.err)
		}
	}
	res.Timings.Modules = time.Since(start)
	o.Fetch.Prune(14 * 24 * time.Hour)

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
	if len(parts) == 0 { // upstream shows a message instead of a blank panel
		c := render.NewCanvas(panelH-80, 120, render.FontDefault, 22)
		drawHint(c, "settings.json has no modules")
		parts = append(parts, render.Part{Img: c, SectionW: panelH, SectionH: 120})
	}
	canvas := render.Assemble(panelH, panelW, parts, render.Layout{
		Border: bool(s.BorderAroundModules), Dividers: bool(s.Dividers), Info: info, InfoHeight: infoHeight})
	frame, err := render.Pack(canvas, panelW, panelH, int(s.Orientation))
	if err != nil {
		return res, err
	}
	res.Timings.Render = time.Since(start) - res.Timings.Modules

	calibrate := !bool(s.DisableCalibration) && schedule.CalibrationDue(now, s.CalibrationHours, st.LastCalibration)
	res.Refreshed = calibrate || !bool(s.ImageHash) || frame.Hash() != st.LastHash
	displayDone := make(chan error, 1)
	if res.Refreshed {
		go func() {
			<-prepared
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
		go func() {
			<-prepared
			if canPrepare {
				if err := prep.Release(); err != nil {
					slog.Warn("putting the prepared panel to sleep failed", "err", err)
				}
			}
			displayDone <- nil
		}()
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
	// Nothing needs the network any more if we power off afterwards: switch the
	// radio off for the refresh (~50 s on the 12.48"), it costs battery for nothing.
	radioOff := false
	if o.Network != nil && shouldPowerOff(s, o.PiSugar != nil, alarmOK, o.System.Maintenance(), plugged) {
		if err := o.Network.RadioOff(); err != nil {
			slog.Warn("switching the radio off failed", "err", err)
		} else {
			slog.Info("radio off for the refresh, powering off afterwards")
		}
		radioOff = true
	}

	displayErr := <-displayDone
	res.Timings.Display = time.Since(start) - res.Timings.Modules - res.Timings.Render

	// Modules, calibration or a cold panel can take longer than minSleep: never power off
	// with an alarm that has passed (repeat=127 would only wake us the next day).
	if alarmOK && res.Wake.Sub(o.Now()) < minSleep {
		res.Wake = nextWake(s, o.Now().In(o.Loc))
		if err := o.PiSugar.SetAlarm(res.Wake, alarmEveryDay); err != nil {
			slog.Error("re-setting wake-up alarm failed, staying on", "err", err)
			alarmOK = false
		}
	}
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
	if radioOff && !res.PowerOff { // staying on after all: keep the device reachable
		if err := o.Network.RadioOn(); err != nil {
			slog.Error("switching the radio back on failed", "err", err)
		}
	}
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

// Calibrate runs the panel's clean cycles and forgets the last frame hash, so the next cycle
// redraws the (now white) panel even if the content did not change.
func Calibrate(o *Options) error {
	if err := o.Display.Clear(int(o.Settings.CalibrationCycles)); err != nil {
		return err
	}
	st, err := state.Load(o.StateDir)
	if err != nil {
		st = state.State{}
	}
	st.LastHash = ""
	st.LastCalibration = o.Now()
	return state.Save(o.StateDir, st)
}
