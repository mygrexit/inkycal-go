package config

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
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

func moduleErrs(s *Settings) []string {
	var out []string
	for _, m := range s.Modules {
		if m.Err == nil {
			out = append(out, "")
		} else {
			out = append(out, m.Err.Error())
		}
	}
	return out
}

// On the device a broken tile must not keep the whole panel from refreshing: Load
// marks the module, the builder (Parse) still refuses to save such settings.
func TestLoadMarksBrokenModulesParseRejects(t *testing.T) {
	cases := []struct {
		name, body string
		bad        int    // index of the broken module
		mention    string // in its error
	}{
		{"bad style", `{"model":"m","modules":[
			{"position":1,"name":"A","config":{"size":[480,200]}},
			{"position":2,"name":"B","config":{"size":[480,200],"style":{"scale":3}}}]}`, 1, "style"},
		{"missing cell", `{"model":"m","modules":[
			{"position":1,"name":"A","config":{"cell":[0,0,12,4]}},
			{"position":2,"name":"B","config":{}}]}`, 1, "no cell"},
		{"outside the grid", `{"model":"m","modules":[
			{"position":1,"name":"A","config":{"cell":[0,0,12,4]}},
			{"position":2,"name":"B","config":{"cell":[0,4,99,4]}}]}`, 1, "outside"},
		{"overlap", `{"model":"m","modules":[
			{"position":1,"name":"A","config":{"cell":[0,0,12,4]}},
			{"position":2,"name":"B","config":{"cell":[0,2,12,4]}}]}`, 1, "overlaps"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if _, err := Parse([]byte(c.body), "settings"); err == nil {
				t.Error("Parse accepted it; the builder must refuse to save")
			}
			s := load(t, c.body)
			errs := moduleErrs(s)
			for i, e := range errs {
				if i == c.bad && !strings.Contains(e, c.mention) {
					t.Errorf("module %d: error %q, want mention of %q", i+1, e, c.mention)
				}
				if i != c.bad && e != "" {
					t.Errorf("module %d: unexpected error %q", i+1, e)
				}
			}
			if _, _, _, err := GridCells(s); err != nil {
				t.Errorf("GridCells after Load: %v", err)
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

// A style of the wrong type is the module's own problem (reported when it runs); it
// must not cost the module its valid cell.
func TestLoadKeepsCellWhenStyleDoesNotDecode(t *testing.T) {
	s := load(t, `{"model":"m","modules":[
		{"position":1,"name":"A","config":{"cell":[0,0,12,4],"style":{"scale":"big"}}},
		{"position":2,"name":"B","config":{"cell":[0,4,12,4]}}]}`)
	if errs := moduleErrs(s); errs[0] != "" || errs[1] != "" {
		t.Errorf("errors %q: the cell is fine", errs)
	}
}

// Both problems of a module are reported at once, not one per wake.
func TestLoadReportsStyleAndCellTogether(t *testing.T) {
	s := load(t, `{"model":"m","modules":[
		{"position":1,"name":"A","config":{"cell":[0,0,12,4]}},
		{"position":2,"name":"B","config":{"cell":[0,2,12,4],"style":{"font":"comic"}}}]}`)
	e := s.Modules[1].Err
	if !errors.Is(e, ErrCell) || !strings.Contains(e.Error(), "overlaps") || !strings.Contains(e.Error(), "comic") {
		t.Errorf("err %v, want the overlap and the style", e)
	}
}

// When no module can be placed the page stays a grid page (with a hint), not a stack.
func TestGridCellsAllUnplaced(t *testing.T) {
	s := load(t, `{"model":"m","modules":[{"position":1,"name":"A","config":{"cell":[0,0,99,4]}}]}`)
	_, _, grid, err := GridCells(s)
	if err != nil || !grid {
		t.Errorf("grid %v err %v", grid, err)
	}
}
