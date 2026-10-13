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
	canvas := render.Assemble(panelH, panelW, parts, render.Layout{
		Border: bool(s.BorderAroundModules), Dividers: bool(s.Dividers), Info: info, InfoHeight: infoHeight})
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

	// Modules, calibration or a cold panel can take longer than minSleep: never power off
	// with an alarm that has passed (repeat=127 would only wake us the next day).
	if alarmOK && res.Wake.Sub(o.Now()) < minSleep {
		res.Wake = schedule.NextWake(o.Now().In(o.Loc), int(s.UpdateInterval), minSleep)
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
