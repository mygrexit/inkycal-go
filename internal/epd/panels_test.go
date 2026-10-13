package epd

import (
	"sort"
	"testing"
)

func TestEveryModelHasAFriendlyPanel(t *testing.T) {
	ps := Panels()
	if len(ps) != len(Models) {
		t.Fatalf("%d panels for %d models", len(ps), len(Models))
	}
	names := map[string]bool{}
	for _, p := range ps {
		if p.Name == "" || p.Inches == 0 || p.Group == "" || p.W == 0 || p.Inks == "" {
			t.Errorf("%s: incomplete %+v", p.Model, p)
		}
		if names[p.Name] {
			t.Errorf("duplicate name %q", p.Name)
		}
		names[p.Name] = true
		if size := Models[p.Model]; size != [2]int{p.W, p.H} {
			t.Errorf("%s: size %v vs %dx%d", p.Model, size, p.W, p.H)
		}
	}
	if !sort.SliceIsSorted(ps, func(i, j int) bool { return ps[i].Inches < ps[j].Inches }) {
		t.Error("panels not sorted by size")
	}
	p := PanelOf("epd_12_in_48_colour_V2")
	if p.Name != `Waveshare 12.48″ V2 · black, white, red` || p.Group != "12–13″" {
		t.Errorf("12.48 V2: %+v", p)
	}
}
