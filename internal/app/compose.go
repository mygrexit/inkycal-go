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
	"inkycal/internal/layout"
	"inkycal/internal/modules"
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
	g, cells, grid, err := config.GridCells(s)
	if err != nil {
		return Composed{}, err
	}
	var rects []layout.Rect
	if grid {
		rects = make([]layout.Rect, len(cells))
		for i, c := range cells {
			rects[i] = g.Rect(c, pageW, pageH-infoHeight)
		}
	}
	start := time.Now()
	outcomes := runModules(ctx, o, now, rects)
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
		placed := make([]render.Placed, len(outcomes))
		for i, oc := range outcomes {
			placed[i] = render.Placed{Img: oc.part.Img, X: oc.rect.X, Y: oc.rect.Y, W: oc.rect.W, H: oc.rect.H, Inks: oc.part.Inks}
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
	rect  layout.Rect // grid layouts: the module's section on the page
	err   error
	stale bool
}

// fail records a module's error and writes the matching hint into its image.
func (oc *moduleOutcome) fail(err error, module, lang string) {
	oc.err = err
	drawHint(oc.part.Img, errorHint(module, err, lang))
}

// runModules runs all modules concurrently; a failed module leaves its section blank.
// rects (grid layouts) give each module's section; nil takes the size from its config.
func runModules(ctx context.Context, o *Options, now time.Time, rects []layout.Rect) []moduleOutcome {
	ctx, cancel := context.WithTimeout(ctx, moduleTimeout)
	defer cancel()
	out := make([]moduleOutcome, len(o.Settings.Modules))
	inks := make([]*render.InkMap, len(o.Settings.Modules))
	var wg sync.WaitGroup
	for i, entry := range o.Settings.Modules {
		common, err := config.ParseCommon(entry.Config)
		style := config.Effective(o.Settings.Theme, common.Style)
		common.FontSize = scaled(common.FontSize, style.Scale)
		theme := themeFor(style, o.Settings.Dir())
		inks[i] = inksFor(style)
		if rects != nil {
			out[i].rect = rects[i]
			if err == nil {
				common, err = common.WithSize(rects[i].W, rects[i].H)
			}
			common.Size = [2]int{rects[i].W, rects[i].H} // keep the blank section even when the config failed
		}
		w, h := common.InnerSize()
		// A failed module keeps its section blank, so it needs a canvas even with a broken size.
		out[i].part = render.Part{
			Img:      render.NewThemedCanvas(max(w, 1), max(h, 1), render.FontDefault, 14, theme),
			SectionW: common.Size[0], SectionH: common.Size[1],
		}
		if err != nil {
			out[i].err = fmt.Errorf("%s: %w", entry.Name, err)
			if rects != nil {
				// On a grid a shrinking cell can outgrow the padding without the module
				// being touched: say so across the whole cell instead of leaving it blank.
				out[i].part.Img = render.NewThemedCanvas(rects[i].W, rects[i].H, render.FontDefault, 14, theme)
				drawHint(out[i].part.Img, errorHint(entry.Name, out[i].err, common.Language))
			}
			continue
		}
		factory, ok := o.Registry[entry.Name]
		if !ok {
			out[i].fail(fmt.Errorf("unknown module %q", entry.Name), entry.Name, common.Language)
			continue
		}
		wg.Add(1)
		go func(i int, entry config.ModuleEntry, common config.Common, theme *render.Theme) {
			defer wg.Done()
			defer func() {
				if r := recover(); r != nil {
					// The module may have panicked mid-drawing: hint on a fresh canvas.
					img := out[i].part.Img
					out[i].part.Img = render.NewThemedCanvas(img.W, img.H, render.FontDefault, 14, theme)
					out[i].fail(fmt.Errorf("%s panicked: %v", entry.Name, r), entry.Name, common.Language)
				}
			}()
			m, err := factory(common, entry.Config)
			if err != nil {
				out[i].fail(err, entry.Name, common.Language)
				return
			}
			env := &modules.Env{Now: now, Loc: o.Loc, Fetch: o.Fetch, Theme: theme}
			cv, err := m.Generate(ctx, env)
			if err != nil {
				out[i].fail(fmt.Errorf("%s: %w", entry.Name, err), entry.Name, common.Language)
				return
			}
			out[i].part.Img = cv
			out[i].stale = env.Stale
		}(i, entry, common, theme)
	}
	wg.Wait()
	// Own colours are applied to the finished image, so modules need not know them.
	for i := range out {
		if inks[i] != nil {
			out[i].part.Img.Recolor(*inks[i])
			out[i].part.Inks = inks[i]
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
