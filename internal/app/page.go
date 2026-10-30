package app

import (
	"encoding/json"
	"fmt"

	"inkycal/internal/config"
	"inkycal/internal/layout"
	"inkycal/internal/render"
)

// tile is a tile of settings.json resolved against the page: where it goes, how it looks
// and whether it can run.
type tile struct {
	name   string
	config json.RawMessage
	common config.Common // sized to its section
	rect   layout.Rect   // grid: its section on the page
	grid   bool
	placed bool // false: unplaced on a grid, only reported
	theme  *render.Theme
	inks   *render.InkMap
	err    error // why it cannot run, with its name
}

// resolveTiles places and styles every tile on a w×h page area (without the info line).
// It is lenient like config.Tiles: a tile that cannot run gets err and the others still
// resolve; the error is for a page-wide problem only. grid tells which layout the page has.
func resolveTiles(s *config.Settings, w, h int) (tiles []tile, grid bool, err error) {
	g, specs, err := s.Tiles()
	if err != nil {
		return nil, false, err
	}
	tiles = make([]tile, len(specs))
	for i, sp := range specs {
		t := &tiles[i]
		t.name, t.config, t.common, t.grid = sp.Name, sp.Config, sp.Common, g != nil
		t.placed = g == nil || sp.Cell != nil
		if !t.placed {
			t.err = fmt.Errorf("%s: %w", t.name, sp.Err)
			continue
		}
		t.theme, t.inks = themeFor(sp.Look, s.Dir()), inksFor(sp.Look)
		err := sp.Err
		if g != nil {
			t.rect = g.Rect(*sp.Cell, w, h)
			if err == nil {
				t.common, err = t.common.WithSize(t.rect.W, t.rect.H)
			}
			t.common.Size = [2]int{t.rect.W, t.rect.H} // keep the section even when the config failed
		}
		if err != nil {
			t.err = fmt.Errorf("%s: %w", t.name, err)
		}
	}
	return tiles, g != nil, nil
}

// canvas is a blank canvas for the tile in its look: the module's inner area, or for a
// tile that cannot run the area its hint goes on. On a grid that is the whole section (a
// shrinking cell can outgrow the padding without the tile being touched), and so it is
// for a stacked tile whose padding leaves no inner area. An unplaced tile has no area.
func (t tile) canvas() *render.Canvas {
	if !t.placed {
		return render.NewCanvas(1, 1, render.FontDefault, 14)
	}
	w, h := t.common.InnerSize()
	if t.err != nil && (t.grid || w <= 0 || h <= 0) {
		w, h = t.common.Size[0], t.common.Size[1]
	}
	return render.NewThemedCanvas(max(w, 1), max(h, 1), render.FontDefault, 14, t.theme)
}
