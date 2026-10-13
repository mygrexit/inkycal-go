package config

import (
	"strings"
	"testing"

	"inkycal/internal/layout"
)

const gridSettings = `{
  "model": "epd_7_in_5_v2_colour",
  "grid": {"columns": 6, "rows": 8},
  "modules": [
    {"position": 2, "name": "B", "config": {"cell": [3, 0, 3, 8], "padding_x": 5, "padding_y": 5}},
    {"position": 1, "name": "A", "config": {"cell": [0, 0, 3, 8], "padding_x": 5, "padding_y": 5}}
  ]
}`

func TestParseGridSettings(t *testing.T) {
	s, err := Parse([]byte(gridSettings), "test")
	if err != nil {
		t.Fatal(err)
	}
	g, cells, grid, err := GridCells(s)
	if err != nil || !grid {
		t.Fatalf("grid=%v err=%v", grid, err)
	}
	if g != (layout.Grid{Columns: 6, Rows: 8}) {
		t.Errorf("grid = %+v", g)
	}
	// Modules are sorted by position, cells follow them.
	if s.Modules[0].Name != "A" || cells[0] != (layout.Cell{0, 0, 3, 8}) || cells[1] != (layout.Cell{3, 0, 3, 8}) {
		t.Errorf("modules %v cells %v", s.Modules, cells)
	}
	// A cell module without size parses; WithSize applies the padding check.
	c, err := ParseCommon(s.Modules[0].Config)
	if err != nil {
		t.Fatalf("cell module without size: %v", err)
	}
	if c, err = c.WithSize(100, 50); err != nil || c.Size != [2]int{100, 50} {
		t.Errorf("WithSize: %+v %v", c.Size, err)
	}
	if _, err := c.WithSize(8, 50); err == nil {
		t.Error("WithSize accepted a size smaller than the padding")
	}
}

func TestGridDefaultsAndLegacy(t *testing.T) {
	s, err := Parse([]byte(`{"model":"m","modules":[{"position":1,"name":"A","config":{"cell":[0,0,12,16]}}]}`), "test")
	if err != nil {
		t.Fatal(err)
	}
	if g, _, grid, _ := GridCells(s); !grid || g != layout.Default {
		t.Errorf("default grid: %+v %v", g, grid)
	}
	legacy, err := Parse([]byte(`{"model":"m","modules":[{"position":1,"name":"A","config":{"size":[480,200]}}]}`), "test")
	if err != nil {
		t.Fatal(err)
	}
	if _, cells, grid, err := GridCells(legacy); grid || cells != nil || err != nil {
		t.Errorf("legacy: grid=%v cells=%v err=%v", grid, cells, err)
	}
}

func TestGridErrors(t *testing.T) {
	cases := map[string]string{
		"mixed": `{"model":"m","modules":[
			{"position":1,"name":"A","config":{"cell":[0,0,6,8]}},
			{"position":2,"name":"B","config":{"size":[480,200]}}]}`,
		"overlap": `{"model":"m","modules":[
			{"position":1,"name":"A","config":{"cell":[0,0,6,8]}},
			{"position":2,"name":"B","config":{"cell":[5,0,6,8]}}]}`,
		"outside": `{"model":"m","grid":{"columns":4,"rows":4},"modules":[
			{"position":1,"name":"A","config":{"cell":[0,0,6,8]}}]}`,
	}
	want := map[string]string{"mixed": "module 2 (B) has no cell", "overlap": "overlap", "outside": "outside"}
	for name, js := range cases {
		_, err := Parse([]byte(js), "test")
		if err == nil || !strings.Contains(err.Error(), want[name]) {
			t.Errorf("%s: err = %v, want %q", name, err, want[name])
		}
	}
}
