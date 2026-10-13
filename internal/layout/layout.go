// Package layout places modules on a grid of cells, the model the builder edits.
package layout

import "fmt"

// Grid divides the area above the info line into Columns × Rows cells.
type Grid struct {
	Columns int `json:"columns"`
	Rows    int `json:"rows"`
}

// Default is used when modules have cells but settings.json has no "grid".
var Default = Grid{Columns: 12, Rows: 16}

// Cell is [col, row, colSpan, rowSpan], 0-based.
type Cell [4]int

// Rect is a pixel rectangle.
type Rect struct{ X, Y, W, H int }

// Rect returns the pixel rectangle of c on a w×h area. Edges are col*w/Columns, so
// neighbouring cells share their edges and the grid tiles the area exactly.
func (g Grid) Rect(c Cell, w, h int) Rect {
	x0, x1 := c[0]*w/g.Columns, (c[0]+c[2])*w/g.Columns
	y0, y1 := c[1]*h/g.Rows, (c[1]+c[3])*h/g.Rows
	return Rect{X: x0, Y: y0, W: x1 - x0, H: y1 - y0}
}

// Validate checks that every cell lies inside the grid and no two overlap.
// Modules are numbered from 1 in messages, like positions in settings.json.
func (g Grid) Validate(cells []Cell) error {
	if g.Columns < 1 || g.Rows < 1 || g.Columns > 48 || g.Rows > 64 {
		return fmt.Errorf("grid %dx%d: columns must be 1–48, rows 1–64", g.Columns, g.Rows)
	}
	for i, c := range cells {
		if c[0] < 0 || c[1] < 0 || c[2] < 1 || c[3] < 1 || c[0]+c[2] > g.Columns || c[1]+c[3] > g.Rows {
			return fmt.Errorf("module %d: cell %v is outside the %dx%d grid", i+1, c, g.Columns, g.Rows)
		}
		for j := 0; j < i; j++ {
			if Overlaps(cells[j], c) {
				return fmt.Errorf("modules %d and %d overlap", j+1, i+1)
			}
		}
	}
	return nil
}

// Overlaps reports whether two cells share at least one grid cell (touching edges do not count).
func Overlaps(a, b Cell) bool {
	return a[0] < b[0]+b[2] && b[0] < a[0]+a[2] && a[1] < b[1]+b[3] && b[1] < a[1]+a[3]
}
