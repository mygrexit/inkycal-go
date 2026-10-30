package greek

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"
	"unicode"

	"inkycal/internal/modules"
	"inkycal/internal/modules/modulestest"
	"inkycal/internal/render"
)

func TestWordList(t *testing.T) {
	ws, err := loadWords()
	if err != nil {
		t.Fatal(err)
	}
	if len(ws) != 366 {
		t.Fatalf("%d words, want one per day of a leap year", len(ws))
	}
	seen := map[string]bool{}
	for _, w := range ws {
		if w.El == "" || w.Tr == "" || w.De == "" || w.En == "" || seen[w.El] {
			t.Errorf("bad or duplicate entry %+v", w)
		}
		seen[w.El] = true
		if !render.HasGlyphs(render.FontDisplay, w.El) {
			t.Errorf("%q: display font misses a glyph", w.El)
		}
		if w.Pic == "" {
			continue
		}
		if isNumber(w.Pic) {
			continue
		}
		if !render.HasGlyphs(render.FontEmoji, w.Pic) {
			t.Errorf("%s (%s): pictogram %q not in the emoji font", w.El, w.De, w.Pic)
		}
	}
}

func TestPickIsStableForADayAndCycles(t *testing.T) {
	ws := []word{{El: "a"}, {El: "b"}, {El: "c"}}
	loc, _ := time.LoadLocation("Europe/Berlin")
	morning := time.Date(2026, 10, 13, 6, 0, 0, 0, loc)
	noon := time.Date(2026, 10, 13, 12, 0, 0, 0, loc)
	if pick(ws, morning) != pick(ws, noon) {
		t.Error("word changes within a day")
	}
	next := pick(ws, morning.AddDate(0, 0, 1))
	if next == pick(ws, morning) {
		t.Error("word does not change from one day to the next")
	}
	// Over the length of the list every word comes up once.
	got := map[string]bool{}
	for d := range ws {
		got[pick(ws, morning.AddDate(0, 0, d)).El] = true
	}
	if len(got) != len(ws) {
		t.Errorf("cycle covers %d of %d words", len(got), len(ws))
	}
}

func TestSplitArticle(t *testing.T) {
	for in, want := range map[string][2]string{
		"η χελώνα": {"η", "χελώνα"}, "το σπίτι": {"το", "σπίτι"}, "ο σκύλος": {"ο", "σκύλος"},
		"καλημέρα": {"", "καλημέρα"}, "το ινδικό χοιρίδιο": {"το", "ινδικό χοιρίδιο"},
	} {
		art, rest := splitArticle(in)
		if art != want[0] || rest != want[1] {
			t.Errorf("%q → %q %q", in, art, rest)
		}
	}
}

func TestGenerate(t *testing.T) {
	common := modulestest.Common(t, `{"size":[984,174],"padding_x":40,"padding_y":18,"fontsize":22,"language":"de"}`)
	m, err := Def.New(common, json.RawMessage(`{}`))
	if err != nil {
		t.Fatal(err)
	}
	loc, _ := time.LoadLocation("Europe/Berlin")
	for d := range 3 {
		now := time.Date(2026, 10, 13+d, 6, 0, 0, 0, loc)
		c, err := m.Generate(context.Background(), &modules.Env{Now: now, Loc: loc})
		if err != nil {
			t.Fatal(err)
		}
		if c.W != 904 || c.H != 138 {
			t.Fatalf("canvas %dx%d", c.W, c.H)
		}
		black, colour := modulestest.InkIn(c, 0, 0, c.W, c.H)
		w := pick(m.(*Module).words, now)
		art, _ := splitArticle(w.El)
		if !black || (art != "" && !colour) {
			t.Errorf("%s: black=%v colour=%v", w.El, black, colour)
		}
	}
}

func isNumber(s string) bool {
	return strings.IndexFunc(s, func(r rune) bool { return !unicode.IsDigit(r) }) < 0
}
