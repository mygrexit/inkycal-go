package greek

import (
	"image"
	"testing"

	"inkycal/internal/modules/modulestest"
	"inkycal/internal/render"
)

// In every tile shape the pictogram, the Greek word, its transliteration and the
// translation keep apart and inside the tile, and the word stays readable.
func TestLayoutNeverOverlaps(t *testing.T) {
	ws, err := loadWords()
	if err != nil {
		t.Fatal(err)
	}
	samples := []word{ws[0]} // with pictogram
	for _, w := range ws {
		if len(w.De) > 16 { // a long translation
			samples = append(samples, w)
			break
		}
	}
	samples = append(samples, word{El: "η καλημέρα", Tr: "i kaliméra", De: "guten Morgen", En: "good morning"}) // no pictogram
	for _, s := range modulestest.ShapesFor(Meta) {
		for _, wd := range samples {
			c := render.NewCanvas(s.W, s.H, render.FontDefault, 14)
			l := plan(c, wd, wd.De)
			boxes := append([]image.Rectangle{l.Pic}, l.Word.Box, l.Tr.Box, l.Meaning.Box)
			bounds := image.Rect(0, 0, s.W, s.H)
			for i, a := range boxes {
				if a.Empty() {
					continue
				}
				if !a.In(bounds) {
					t.Errorf("%s %q: box %d %v outside %v", s.Name, wd.De, i, a, bounds)
				}
				for j := i + 1; j < len(boxes); j++ {
					if b := boxes[j]; !b.Empty() && a.Overlaps(b) {
						t.Errorf("%s %q: box %d %v overlaps box %d %v", s.Name, wd.De, i, a, j, b)
					}
				}
			}
			if l.Word.Box.Dy() < 12 {
				t.Errorf("%s %q: word only %d px high", s.Name, wd.De, l.Word.Box.Dy())
			}
		}
	}
}
