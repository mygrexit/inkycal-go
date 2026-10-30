package app

import (
	"context"
	"fmt"
	"log/slog"
	"math"
	"strings"
	"sync"
	"time"

	"inkycal/internal/modules"
	"inkycal/internal/problem"
	"inkycal/internal/render"
	"inkycal/internal/timefmt"
)

// ModuleStatus is how one module of settings.json fared, in settings order.
type ModuleStatus struct {
	Err   error
	Stale bool // rendered from cached data
}

// Composed is the laid-out page before it is packed for the panel.
type Composed struct {
	Canvas  *render.Canvas
	Modules []ModuleStatus
	Took    time.Duration // modules only
}

// Compose runs the modules and lays out the page (portrait canvas: panel height × width),
// including the info line. The battery level comes from o.PiSugar when set.
func Compose(ctx context.Context, o *Options, now time.Time) (Composed, error) {
	s := o.Settings
	panelW, panelH := o.Display.Size()
	pageW, pageH := panelH, panelW // portrait page, as Assemble lays it out
	infoHeight := 0
	if s.InfoSection {
		infoHeight = int(s.InfoSectionHeight)
	}
	tiles, grid, err := resolveTiles(s, pageW, pageH-infoHeight)
	if err != nil {
		return Composed{}, err
	}
	start := time.Now()
	outcomes := runModules(ctx, o, now, tiles)
	took := time.Since(start)
	statuses := make([]ModuleStatus, len(outcomes))
	parts := make([]render.Part, len(outcomes), max(len(outcomes), 1))
	for i, oc := range outcomes {
		statuses[i] = ModuleStatus{Err: oc.err, Stale: oc.stale}
		parts[i] = oc.part
	}
	var battery float64
	hasBattery := false
	if o.PiSugar != nil {
		if b, err := o.PiSugar.Battery(); err == nil {
			battery, hasBattery = b, true
		} else {
			slog.Warn("battery level unavailable", "err", err)
		}
	}
	info := ""
	if s.InfoSection {
		info = infoText(now, bool(s.ImageHash), statuses, battery, hasBattery)
	}
	look := render.Layout{Border: bool(s.BorderAroundModules), Dividers: bool(s.Dividers), Info: info, InfoHeight: infoHeight}
	if grid {
		placed := make([]render.Placed, 0, len(outcomes))
		for i, oc := range outcomes {
			if t := tiles[i]; t.placed {
				placed = append(placed, render.Placed{Img: oc.part.Img, X: t.rect.X, Y: t.rect.Y, W: t.rect.W, H: t.rect.H, Inks: oc.part.Inks})
			}
		}
		if len(placed) == 0 && len(outcomes) > 0 { // every tile was left out: say so
			c := render.NewCanvas(pageW-80, 120, render.FontDefault, 22)
			drawHint(c, "settings.json: no tile fits the grid – fix it with the builder")
			placed = append(placed, render.Placed{Img: c, X: 0, Y: 0, W: pageW, H: 120})
		}
		return Composed{Canvas: render.AssembleGrid(pageW, pageH, placed, look), Modules: statuses, Took: took}, nil
	}
	if len(parts) == 0 { // upstream shows a message instead of a blank panel
		c := render.NewCanvas(panelH-80, 120, render.FontDefault, 22)
		drawHint(c, "settings.json has no modules")
		parts = append(parts, render.Part{Img: c, SectionW: panelH, SectionH: 120})
	}
	return Composed{Canvas: render.Assemble(pageW, pageH, parts, look), Modules: statuses, Took: took}, nil
}

type moduleOutcome struct {
	part  render.Part
	err   error
	stale bool
}

// fail records a module's error and writes the matching hint into its image.
func (oc *moduleOutcome) fail(err error, module, lang string) {
	oc.err = err
	drawHint(oc.part.Img, errorHint(module, err, lang))
}

// runModules runs the tiles' modules concurrently; a tile that cannot run shows its hint.
func runModules(ctx context.Context, o *Options, now time.Time, tiles []tile) []moduleOutcome {
	ctx, cancel := context.WithTimeout(ctx, moduleTimeout)
	defer cancel()
	out := make([]moduleOutcome, len(tiles))
	var wg sync.WaitGroup
	for i, t := range tiles {
		out[i].part = render.Part{Img: t.canvas(), SectionW: t.common.Size[0], SectionH: t.common.Size[1]}
		if !t.placed {
			out[i].err = t.err // no section: nothing to draw, only the error to report
			continue
		}
		if t.err != nil {
			out[i].fail(t.err, t.name, t.common.Language)
			continue
		}
		factory, ok := o.Registry[t.name]
		if !ok {
			out[i].fail(problem.Mark(problem.UnknownModule, fmt.Errorf("unknown module %q", t.name)), t.name, t.common.Language)
			continue
		}
		wg.Go(func() {
			defer func() {
				if r := recover(); r != nil {
					// The module may have panicked mid-drawing: hint on a fresh canvas.
					out[i].part.Img = t.canvas()
					out[i].fail(fmt.Errorf("%s panicked: %v", t.name, r), t.name, t.common.Language)
				}
			}()
			m, err := factory(t.common, t.config)
			if err != nil {
				out[i].fail(err, t.name, t.common.Language)
				return
			}
			env := &modules.Env{Now: now, Loc: o.Loc, Fetch: o.Fetch, Theme: t.theme}
			cv, err := m.Generate(ctx, env)
			if err != nil {
				out[i].fail(fmt.Errorf("%s: %w", t.name, err), t.name, t.common.Language)
				return
			}
			out[i].part.Img = cv
			out[i].stale = env.Stale
		})
	}
	wg.Wait()
	// Own colours are applied to the finished image, so modules need not know them.
	for i, t := range tiles {
		if t.inks != nil {
			out[i].part.Img.Recolor(*t.inks)
			out[i].part.Inks = t.inks
		}
	}
	return out
}

// infoText reproduces Python's info line (timestamp only without image_hash, which would defeat the hash).
func infoText(now time.Time, imageHash bool, mods []ModuleStatus, battery float64, hasBattery bool) string {
	var b strings.Builder
	if !imageHash {
		b.WriteString(timefmt.Format(now, "D MMM @ HH:mm", "en") + "  ")
	}
	for i, m := range mods {
		switch {
		case m.Err != nil:
			fmt.Fprintf(&b, "im %d: X  ", i+1)
		case m.Stale:
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
