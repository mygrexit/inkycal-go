package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"

	"inkycal/internal/layout"
	"inkycal/internal/problem"
)

// Tile is one entry of settings.json's modules with its config decoded once.
type Tile struct {
	Name   string
	Config json.RawMessage // for the module, which reads its own keys from it
	Common Common          // shared keys with defaults, font size scaled by the look; as far as it got when the config does not decode
	Cell   *layout.Cell    // its cell on a grid page; nil when stacked or unplaced
	Look   Style           // its style over the page theme; the theme alone when its style is broken
	Err    error           // why it cannot run: placement, style, config or padding

	rejected error // what Parse refuses: a placement or style problem
}

// Tiles decodes every tile of the page, in settings order. It is lenient: a tile that
// cannot run gets Err (unplaced on a grid: also no Cell) and the others still resolve.
// grid is nil for a stacked page; a grid page has cells on its tiles (a tile without
// one is unplaced). err is for a page-wide problem only: a grid that holds no cell.
// Tiles reads Modules on every call and keeps nothing.
func (s *Settings) Tiles() (grid *layout.Grid, tiles []Tile, err error) {
	tiles = make([]Tile, len(s.Modules))
	cells := make([]*layout.Cell, len(s.Modules))
	other := make([]error, len(s.Modules)) // config or padding problem, for a tile with a section
	anyCell := false
	for i, m := range s.Modules {
		t := &tiles[i]
		t.Name, t.Config = m.Name, m.Config
		c, err := decodeCommon(m.Config)
		// checked is the style to validate; the look still takes c.Style as far as it decoded.
		cell, checked := c.Cell, c.Style
		if err != nil {
			// One field of the wrong type must not cost the tile its cell or hide a broken
			// style: read those two on their own (apart, so one cannot spoil the other).
			cell, checked = nil, nil
			var part struct {
				Cell *layout.Cell `json:"cell"`
			}
			if json.Unmarshal(m.Config, &part) == nil {
				cell = part.Cell
			}
			var st struct {
				Style *Style `json:"style"`
			}
			if json.Unmarshal(m.Config, &st) == nil {
				checked = st.Style
			}
		} else if cell == nil {
			err = c.checkSize()
		}
		own := c.Style
		if e := checked.validate(); e != nil {
			t.rejected = problem.Mark(problem.Style, fmt.Errorf("style: %w", e))
			own = nil
		}
		t.Look = effective(s.Theme, own)
		c.FontSize = scaled(c.FontSize, t.Look.Scale)
		t.Common, cells[i], other[i] = c, cell, err
		anyCell = anyCell || cell != nil
	}
	if anyCell {
		g := layout.Default
		if s.Grid != nil {
			g = *s.Grid
		}
		if err := g.Validate(nil); err != nil {
			return nil, nil, problem.Mark(problem.Placement, fmt.Errorf("grid layout: %w", err))
		}
		grid = &g
		place(g, tiles, cells)
	}
	for i := range tiles {
		t := &tiles[i]
		switch {
		case t.rejected != nil:
			t.Err = t.rejected
		case other[i] != nil: // an unplaced tile is always rejected, so this one has a section
			t.Err = other[i]
		}
	}
	return grid, tiles, nil
}

// place gives each tile its cell, first come first served: a tile without a cell, with
// one outside the grid or overlapping an earlier tile stays unplaced.
func place(g layout.Grid, tiles []Tile, cells []*layout.Cell) {
	var placed []int
	for i, c := range cells {
		var cellErr error
		switch {
		case c == nil:
			cellErr = errors.New("grid layout: no cell")
		case g.Validate([]layout.Cell{*c}) != nil:
			cellErr = fmt.Errorf("grid layout: cell %v is outside the %dx%d grid", *c, g.Columns, g.Rows)
		default:
			for _, j := range placed {
				if layout.Overlaps(*cells[j], *c) {
					cellErr = fmt.Errorf("grid layout: cell %v overlaps module %d", *c, j+1)
					break
				}
			}
		}
		if cellErr != nil {
			// keep a style problem too: both are reported at once, not one per wake
			tiles[i].rejected = errors.Join(problem.Mark(problem.Placement, cellErr), tiles[i].rejected)
			continue
		}
		placed = append(placed, i)
		tiles[i].Cell = c
	}
}

// scaled applies a look's size factor to a font size.
func scaled(size FlexInt, scale float64) FlexInt {
	if scale == 0 {
		return size
	}
	return FlexInt(max(6, int(math.Round(float64(size)*scale))))
}
