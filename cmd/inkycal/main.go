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

// osNetwork waits for a default route and switches the WLAN radios.
type osNetwork struct{}

func (osNetwork) Wait(ctx context.Context, timeout time.Duration) bool {
	return sysinfo.WaitNetwork(ctx, timeout)
}
func (osNetwork) RadioOff() error { return setRadios(false) }
func (osNetwork) RadioOn() error  { return setRadios(true) }

func setRadios(up bool) error {
	var errs []error
	for _, name := range sysinfo.WirelessInterfaces() {
		errs = append(errs, sysinfo.SetLink(name, up))
	}
	return errors.Join(errs...)
}

func main() { os.Exit(run(os.Args[1:])) }

func run(args []string) int {
	fs := flag.NewFlagSet("inkycal", flag.ContinueOnError)
	settingsPath := fs.String("settings", "", "settings.json (default: $INKYCAL_SETTINGS_PATH, /boot/settings.json, /boot/firmware/settings.json)")
	mode := fs.String("mode", "run", "run | once | dry-run | clear | builder")
	out := fs.String("out", "preview.png", "dry-run: PNG output path")
	stateDir := fs.String("state-dir", "", "state + HTTP cache (default: $STATE_DIRECTORY or /var/lib/inkycal)")
	pisugarAddr := fs.String("pisugar", "127.0.0.1:8423", "pisugar-server TCP address")
	listen := fs.String("listen", "127.0.0.1:8080", "builder: address to serve on (0.0.0.0:8080 to reach it from other devices)")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	switch *mode {
	case "run", "once", "dry-run", "clear", "builder":
	default:
		fmt.Fprintf(os.Stderr, "unknown mode %q\n", *mode)
		return 2
	}
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, nil)))
	if *mode == "builder" {
		if err := runBuilder(*settingsPath, *stateDir, *listen); err != nil {
			slog.Error("builder failed", "err", err)
			return 1
		}
		return 0
	}
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
	for _, w := range s.Warnings {
		slog.Warn("settings.json", "problem", w)
	}
	size, ok := epd.Models[s.Model]
	if !ok {
		return fmt.Errorf("display model %q not supported", s.Model)
	}
	var display epd.Display
	if mode == "dry-run" {
		display = epd.NewPNG(out, size[0], size[1])
	} else {
		epd.SettingsPath = path // panel-specific keys such as the IT8951 "vcom"
		if display, err = epd.Open(s.Model); err != nil {
			return err
		}
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
	if mode == "clear" {
		return app.Calibrate(o)
	}
	if mode == "run" || mode == "once" {
		o.Network = osNetwork{} // the service starts before the network is up
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
