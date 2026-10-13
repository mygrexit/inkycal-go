package config

import (
	"strings"
	"testing"
)

func TestEffectiveStyleModuleOverridesPage(t *testing.T) {
	page := &Style{Font: "serif", Scale: 1.2, Colors: Colors{Paper: "white", Ink: "black", Accent: "colour"},
		CustomFonts: map[string]string{"text": "fonts/a.ttf", "bold": "fonts/b.ttf"}}
	mod := &Style{Font: "mono", Colors: Colors{Paper: "black", Ink: "white"}, CustomFonts: map[string]string{"bold": "fonts/c.ttf"}}
	got := Effective(page, mod)
	if got.Font != "mono" || got.Scale != 1.2 || got.Colors != (Colors{"black", "white", "colour"}) {
		t.Errorf("effective %+v", got)
	}
	// the module picked a pair: the page's custom fonts go, its own stay
	if got.CustomFonts["text"] != "" || got.CustomFonts["bold"] != "fonts/c.ttf" {
		t.Errorf("custom fonts %v", got.CustomFonts)
	}
	if e := Effective(nil, nil); e.Font != "" || e.Scale != 0 {
		t.Errorf("empty %+v", e)
	}
	// the page's map is not modified by the merge
	if page.CustomFonts["bold"] != "fonts/b.ttf" {
		t.Error("page custom fonts changed")
	}
}

func TestParseValidatesStyles(t *testing.T) {
	cases := map[string]string{
		`"theme": {"colors": {"paper": "blue"}}`:          `theme: colours: "blue"`,
		`"theme": {"font": "comic"}`:                      `theme: font "comic"`,
		`"theme": {"scale": 3}`:                           `theme: scale 3`,
		`"theme": {"custom_fonts": {"heading": "x.ttf"}}`: `theme: font role "heading"`,
	}
	for theme, want := range cases {
		_, err := Parse([]byte(`{"model":"m",`+theme+`,"modules":[]}`), "test")
		if err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("%s: err = %v, want %q", theme, err, want)
		}
	}
	_, err := Parse([]byte(`{"model":"m","modules":[{"position":1,"name":"A","config":{"size":[10,10],"style":{"colors":{"ink":"pink"}}}}]}`), "test")
	if err == nil || !strings.Contains(err.Error(), "module 1 (A): style") {
		t.Errorf("module style err = %v", err)
	}
	s, err := Parse([]byte(`{"model":"m","theme":{"font":"serif","scale":1.1,"colors":{"accent":"black"}},"modules":[]}`), "test")
	if err != nil || s.Theme == nil || s.Theme.Font != "serif" {
		t.Errorf("valid theme: %v %+v", err, s)
	}
}

// A tile that picks a font pair leaves the page's custom font.
func TestTileFontLeavesPageCustomFont(t *testing.T) {
	page := &Style{Font: "custom", CustomFonts: map[string]string{"text": "fonts/x.ttf", "bold": "fonts/x.ttf"}}
	for _, f := range []string{"serif", "sans", "mono"} {
		got := Effective(page, &Style{Font: f})
		if got.Font != f || len(got.CustomFonts) != 0 {
			t.Errorf("tile %s: %+v", f, got)
		}
	}
	if got := Effective(page, &Style{Scale: 1.2}); got.CustomFonts["text"] != "fonts/x.ttf" {
		t.Errorf("a tile without a font keeps the page font: %+v", got)
	}
}
