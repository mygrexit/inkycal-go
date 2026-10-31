package app

import (
	"context"
	"encoding/json"
	"sort"
	"sync"
	"testing"

	"inkycal/internal/config"
	"inkycal/internal/layout"
	"inkycal/internal/modules"
)

func gridOptions(t *testing.T, cells ...string) *Options {
	t.Helper()
	o, _, _, _ := testOptions(t)
	o.Settings.Grid = &layout.Grid{Columns: 2, Rows: 2}
	o.Settings.Dividers = true
	names := []string{"Dot", "Dot", "Fail"}
	for i, c := range cells {
		o.Settings.Modules = append(o.Settings.Modules, config.ModuleEntry{Position: i + 1, Name: names[i],
			Config: json.RawMessage(`{"cell":` + c + `,"padding_x":10,"padding_y":10}`)})
	}
	return o
}

// fakeDisplay is 800×480 → page 480 wide, 800 high; info line 40 → grid area 480×760.
func TestComposeGridPlacesModulesSideBySide(t *testing.T) {
	o := gridOptions(t, "[0,0,1,2]", "[1,0,1,1]", "[1,1,1,1]")
	got, err := Compose(context.Background(), o.composeOptions(), o.Now())
	if err != nil {
		t.Fatal(err)
	}
	c := got.Canvas
	// Dots sit in the centre of each module's rectangle.
	for _, p := range [][2]int{{120, 380}, {360, 190}} {
		if _, col := c.InkAt(p[0], p[1]); !col {
			t.Errorf("no dot at %v", p)
		}
	}
	// The failed module keeps its place: no dot in the bottom right, an error status.
	if _, col := c.InkAt(360, 570); col {
		t.Error("failed module drew a dot")
	}
	if got.Modules[2].Err == nil {
		t.Error("no error for the failing module")
	}
	if b, _ := c.InkAt(240, 100); !b {
		t.Error("no vertical divider between the columns")
	}
	// Module ink (the red dots) never reaches the info line: the grid ends at y=760.
	for y := 760; y < 800; y++ {
		for x := range 480 {
			if _, col := c.InkAt(x, y); col {
				t.Fatalf("module ink in the info area at %d,%d", x, y)
			}
		}
	}
}

func TestComposeGridSizesModulesFromCells(t *testing.T) {
	var (
		mu    sync.Mutex
		sizes [][2]int
	)
	o := gridOptions(t, "[0,0,1,2]", "[1,0,1,1]")
	o.Registry = map[string]modules.Factory{"Dot": func(c config.Common, raw json.RawMessage) (modules.Module, error) {
		mu.Lock()
		sizes = append(sizes, c.Size)
		mu.Unlock()
		return dotModule{c}, nil
	}}
	if _, err := Compose(context.Background(), o.composeOptions(), o.Now()); err != nil {
		t.Fatal(err)
	}
	sort.Slice(sizes, func(i, j int) bool { return sizes[i][1] > sizes[j][1] })
	if len(sizes) != 2 || sizes[0] != [2]int{240, 760} || sizes[1] != [2]int{240, 380} {
		t.Errorf("sizes = %v", sizes)
	}
}

// On a grid a module whose padding no longer fits its cell (or whose config is broken)
// shows its hint across the whole cell instead of staying blank.
func TestComposeGridBrokenConfigShowsHint(t *testing.T) {
	o, _, _, _ := testOptions(t)
	o.Settings.Grid = &layout.Grid{Columns: 2, Rows: 2}
	for i, cfg := range []string{
		`{"cell":[0,0,1,1],"padding_x":300,"padding_y":10}`,
		`{"cell":[1,0,1,1],"padding_x":"abc"}`,
	} {
		o.Settings.Modules = append(o.Settings.Modules, config.ModuleEntry{Position: i + 1, Name: "Dot", Config: json.RawMessage(cfg)})
	}
	got, err := Compose(context.Background(), o.composeOptions(), o.Now())
	if err != nil {
		t.Fatal(err)
	}
	for i, r := range []layout.Rect{{X: 0, Y: 0, W: 240, H: 380}, {X: 240, Y: 0, W: 240, H: 380}} {
		if got.Modules[i].Err == nil {
			t.Errorf("module %d: no error", i+1)
		}
		ink := 0
		for y := r.Y; y < r.Y+r.H; y++ {
			for x := r.X; x < r.X+r.W; x++ {
				if b, _ := got.Canvas.InkAt(x, y); b {
					ink++
				}
			}
		}
		if ink == 0 {
			t.Errorf("module %d: cell is blank, want the error hint", i+1)
		}
	}
}
