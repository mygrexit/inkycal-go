package layout

import (
	"strings"
	"testing"
)

func TestRectTilesTheAreaExactly(t *testing.T) {
	g := Grid{Columns: 12, Rows: 16}
	// 1304 is not divisible by 16 and 981 not by 12: neighbours must share edges.
	w, h := 981, 1304
	for col := 0; col < g.Columns; col++ {
		a := g.Rect(Cell{col, 0, 1, 1}, w, h)
		if col+1 < g.Columns {
			b := g.Rect(Cell{col + 1, 0, 1, 1}, w, h)
			if a.X+a.W != b.X {
				t.Fatalf("column %d ends at %d, next starts at %d", col, a.X+a.W, b.X)
			}
		} else if a.X+a.W != w {
			t.Fatalf("last column ends at %d, area is %d wide", a.X+a.W, w)
		}
	}
	full := g.Rect(Cell{0, 0, 12, 16}, w, h)
	if full != (Rect{0, 0, w, h}) {
		t.Errorf("full cell = %+v", full)
	}
	if r := g.Rect(Cell{6, 8, 6, 8}, 984, 1304); r != (Rect{492, 652, 492, 652}) {
		t.Errorf("bottom-right quarter = %+v", r)
	}
}

func TestValidate(t *testing.T) {
	g := Default
	ok := []Cell{{0, 0, 12, 4}, {0, 4, 6, 12}, {6, 4, 6, 12}}
	if err := g.Validate(ok); err != nil {
		t.Fatalf("valid layout: %v", err)
	}
	cases := []struct {
		cells []Cell
		want  string
	}{
		{[]Cell{{0, 0, 0, 4}}, "outside"},
		{[]Cell{{-1, 0, 2, 2}}, "outside"},
		{[]Cell{{10, 0, 3, 2}}, "outside"},
		{[]Cell{{0, 15, 1, 2}}, "outside"},
		{[]Cell{{0, 0, 6, 6}, {5, 5, 2, 2}}, "modules 1 and 2 overlap"},
	}
	for _, c := range cases {
		err := g.Validate(c.cells)
		if err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("%v: err = %v, want %q", c.cells, err, c.want)
		}
	}
	if err := (Grid{0, 16}).Validate(nil); err == nil {
		t.Error("grid with 0 columns accepted")
	}
}

func TestOverlapsIsTouchFree(t *testing.T) {
	if Overlaps(Cell{0, 0, 6, 4}, Cell{6, 0, 6, 4}) {
		t.Error("side by side cells overlap")
	}
	if !Overlaps(Cell{0, 0, 6, 4}, Cell{5, 3, 6, 4}) {
		t.Error("overlapping corner not detected")
	}
}
