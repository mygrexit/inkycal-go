package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"inkycal/internal/layout"
)

func load(t *testing.T, body string) *Settings {
	t.Helper()
	p := filepath.Join(t.TempDir(), "settings.json")
	if err := os.WriteFile(p, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	s, err := Load(p)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	return s
}

func tiles(t *testing.T, s *Settings) (*layout.Grid, []Tile) {
	t.Helper()
	g, ts, err := s.Tiles()
	if err != nil {
		t.Fatalf("Tiles: %v", err)
	}
	if len(ts) != len(s.Modules) {
		t.Fatalf("%d tiles for %d modules", len(ts), len(s.Modules))
	}
	return g, ts
}

func tileErrs(ts []Tile) []string {
	var out []string
	for _, t := range ts {
		if t.Err == nil {
			out = append(out, "")
		} else {
			out = append(out, t.Err.Error())
		}
	}
	return out
}

// On the device a broken tile must not keep the whole panel from refreshing: Tiles
// marks the tile, the builder (Parse) still refuses to save such settings.
func TestTilesMarkBrokenTilesParseRejects(t *testing.T) {
	cases := []struct {
		name, body string
		bad        int    // index of the broken tile
		mention    string // in its error
		placed     bool   // the broken tile keeps its section
	}{
		{"bad style", `{"model":"m","modules":[
			{"position":1,"name":"A","config":{"size":[480,200]}},
			{"position":2,"name":"B","config":{"size":[480,200],"style":{"scale":3}}}]}`, 1, "style", true},
		{"missing cell", `{"model":"m","modules":[
			{"position":1,"name":"A","config":{"cell":[0,0,12,4]}},
			{"position":2,"name":"B","config":{}}]}`, 1, "no cell", false},
		{"outside the grid", `{"model":"m","modules":[
			{"position":1,"name":"A","config":{"cell":[0,0,12,4]}},
			{"position":2,"name":"B","config":{"cell":[0,4,99,4]}}]}`, 1, "outside", false},
		{"overlap", `{"model":"m","modules":[
			{"position":1,"name":"A","config":{"cell":[0,0,12,4]}},
			{"position":2,"name":"B","config":{"cell":[0,2,12,4]}}]}`, 1, "overlaps module 1", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, err := Parse([]byte(c.body), "settings")
			if err == nil || !strings.Contains(err.Error(), "module 2 (B): ") || !strings.Contains(err.Error(), c.mention) {
				t.Errorf("Parse: %v; the builder must refuse to save and name the tile", err)
			}
			g, ts := tiles(t, load(t, c.body))
			for i, e := range tileErrs(ts) {
				if i == c.bad && !strings.Contains(e, c.mention) {
					t.Errorf("tile %d: error %q, want mention of %q", i+1, e, c.mention)
				}
				if i != c.bad && e != "" {
					t.Errorf("tile %d: unexpected error %q", i+1, e)
				}
			}
			if g != nil && (ts[c.bad].Cell != nil) != c.placed {
				t.Errorf("tile %d cell %v, placed should be %v", c.bad+1, ts[c.bad].Cell, c.placed)
			}
		})
	}
}

func TestLoadFallsBackToDefaultTheme(t *testing.T) {
	s := load(t, `{"model":"m","theme":{"font":"comic"},"modules":[{"position":1,"name":"A","config":{}}]}`)
	if s.Theme != nil || len(s.Warnings) != 1 || !strings.Contains(s.Warnings[0], "comic") {
		t.Errorf("theme %+v warnings %q", s.Theme, s.Warnings)
	}
}

// A style of the wrong type is the tile's own problem; it must not cost the tile its
// valid cell, and Parse leaves it to the preview like any config it cannot decode.
func TestTileKeepsCellWhenStyleDoesNotDecode(t *testing.T) {
	body := `{"model":"m","modules":[
		{"position":1,"name":"A","config":{"cell":[0,0,12,4],"style":{"scale":"big"}}},
		{"position":2,"name":"B","config":{"cell":[0,4,12,4],"padding_x":"wide"}}]}`
	if _, err := Parse([]byte(body), "settings"); err != nil {
		t.Errorf("Parse: %v", err)
	}
	_, ts := tiles(t, load(t, body))
	for i, tl := range ts {
		if tl.Cell == nil || tl.Err == nil || strings.Contains(tl.Err.Error(), "grid layout") {
			t.Errorf("tile %d: cell %v err %v, want its cell and a decode error", i+1, tl.Cell, tl.Err)
		}
	}
}

// Both problems of a tile are reported at once, not one per wake.
func TestTileReportsStyleAndCellTogether(t *testing.T) {
	_, ts := tiles(t, load(t, `{"model":"m","modules":[
		{"position":1,"name":"A","config":{"cell":[0,0,12,4]}},
		{"position":2,"name":"B","config":{"cell":[0,2,12,4],"style":{"font":"comic"}}}]}`))
	e := ts[1].Err
	if ts[1].Cell != nil || e == nil || !strings.Contains(e.Error(), "overlaps") || !strings.Contains(e.Error(), "comic") {
		t.Errorf("cell %v err %v, want unplaced with the overlap and the style", ts[1].Cell, e)
	}
}

// When no tile can be placed the page stays a grid page (with a hint), not a stack.
func TestTilesAllUnplacedStayAGrid(t *testing.T) {
	g, ts := tiles(t, load(t, `{"model":"m","modules":[{"position":1,"name":"A","config":{"cell":[0,0,99,4]}}]}`))
	if g == nil || ts[0].Cell != nil || ts[0].Err == nil {
		t.Errorf("grid %v tile %+v", g, ts[0])
	}
}

// A grid that cannot hold any cell is a page-wide problem, in both modes.
func TestBrokenGridFailsThePage(t *testing.T) {
	body := `{"model":"m","grid":{"columns":0,"rows":4},"modules":[{"position":1,"name":"A","config":{"cell":[0,0,1,1]}}]}`
	if _, _, err := load(t, body).Tiles(); err == nil {
		t.Error("Tiles accepted a grid without columns")
	}
	if _, err := Parse([]byte(body), "settings"); err == nil {
		t.Error("Parse accepted a grid without columns")
	}
}

// A tile's look is its style over the theme; its font size follows the look's scale.
func TestTileLook(t *testing.T) {
	_, ts := tiles(t, load(t, `{"model":"m","theme":{"font":"serif","scale":1.5,"colors":{"accent":"black"}},"modules":[
		{"position":1,"name":"A","config":{"size":[480,200],"fontsize":20,"style":{"font":"mono"}}},
		{"position":2,"name":"B","config":{"size":[480,200],"style":{"font":"comic"}}}]}`))
	if l := ts[0].Look; l.Font != "mono" || l.Scale != 1.5 || l.Colors.Accent != "black" || ts[0].Common.FontSize != 30 {
		t.Errorf("tile 1: look %+v fontsize %d", l, ts[0].Common.FontSize)
	}
	// a broken style falls back to the theme
	if l := ts[1].Look; l.Font != "serif" || ts[1].Common.FontSize != 21 || ts[1].Common.Language != "en" {
		t.Errorf("tile 2: look %+v common %+v", l, ts[1].Common)
	}
}
