package render

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func inkCount(c *Canvas) int {
	n := 0
	for y := 0; y < c.H; y++ {
		for x := 0; x < c.W; x++ {
			if b, col := c.InkAt(x, y); b || col {
				n++
			}
		}
	}
	return n
}

func samePixels(a, b *Canvas) bool {
	for i := range a.Black.Pix {
		if a.Black.Pix[i] != b.Black.Pix[i] || a.Colour.Pix[i] != b.Colour.Pix[i] {
			return false
		}
	}
	return true
}

func drawWord(c *Canvas, s string) *Canvas {
	c.Text(4, 40, s, Style{Font: FontRegular, Size: 28}, InkBlack)
	return c
}

func TestNilThemeDrawsLikeToday(t *testing.T) {
	a := drawWord(NewCanvas(300, 60, FontDefault, 14), "Hamburg äß")
	b := drawWord(NewThemedCanvas(300, 60, FontDefault, 14, nil), "Hamburg äß")
	if !samePixels(a, b) {
		t.Error("nil theme changed the rendering")
	}
}

func TestSerifPairChangesTheType(t *testing.T) {
	sans := drawWord(NewCanvas(300, 60, FontDefault, 14), "Hamburg")
	serif := drawWord(NewThemedCanvas(300, 60, FontDefault, 14, &Theme{Fonts: Pair("serif")}), "Hamburg")
	if samePixels(sans, serif) || inkCount(serif) == 0 {
		t.Error("serif pair renders like sans")
	}
}

func TestPairsCoverUmlautsGreekAndEuro(t *testing.T) {
	for _, name := range []string{"sans", "serif", "mono"} {
		for id, src := range Pair(name) {
			f, err := loadSource(src, "")
			if err != nil {
				t.Fatalf("%s/%d: %v", name, id, err)
			}
			if id == FontDisplay {
				continue // display faces carry Latin and Greek
			}
			if !fontHas(f, "Ωμέγα äöü ß € –") {
				t.Errorf("%s role %d lacks glyphs", name, id)
			}
		}
	}
}

// A custom font without "ä" still shows it: the default face fills in per character.
func TestCustomFontFallsBackPerCharacter(t *testing.T) {
	th := &Theme{Fonts: map[FontID]FontSource{FontRegular: {File: "ascii.ttf"}}, Base: "testdata"}
	c := NewThemedCanvas(200, 60, FontDefault, 14, th)
	st := Style{Font: FontRegular, Size: 28}
	if c.Advance("ä", st) <= 0 {
		t.Fatal("ä has no advance")
	}
	c.Text(4, 40, "ä", st, InkBlack)
	if inkCount(c) == 0 {
		t.Error("ä drew nothing")
	}
}

func TestMissingCustomFontFallsBack(t *testing.T) {
	th := &Theme{Fonts: map[FontID]FontSource{FontRegular: {File: "nope.ttf"}}, Base: t.TempDir()}
	c := drawWord(NewThemedCanvas(300, 60, FontDefault, 14, th), "Hamburg")
	if !samePixels(c, drawWord(NewCanvas(300, 60, FontDefault, 14), "Hamburg")) {
		t.Error("a missing font must render with the default face")
	}
}

func TestDeriveKeepsTheTheme(t *testing.T) {
	th := &Theme{Fonts: Pair("serif")}
	c := NewThemedCanvas(10, 10, FontDefault, 14, th).Derive(300, 60)
	if c.W != 300 || c.H != 60 || !samePixels(drawWord(c, "Hamburg"), drawWord(NewThemedCanvas(300, 60, FontDefault, 14, th), "Hamburg")) {
		t.Error("Derive lost the theme or the size")
	}
}

// The builder runs long: a font file that appears or changes later must be used.
func TestCustomFontPickedUpWhenWrittenLater(t *testing.T) {
	dir := t.TempDir()
	th := &Theme{Fonts: map[FontID]FontSource{FontRegular: {File: "late.ttf"}}, Base: dir}
	before := drawWord(NewThemedCanvas(300, 60, FontDefault, 14, th), "Hamburg")
	serif, err := os.ReadFile("fonts/NotoSerif-Regular.ttf")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "late.ttf"), serif, 0o644); err != nil {
		t.Fatal(err)
	}
	after := drawWord(NewThemedCanvas(300, 60, FontDefault, 14, th), "Hamburg")
	if samePixels(before, after) {
		t.Error("font written after a failed load is not used")
	}
	// replacing the file with another font is picked up too
	mono, _ := os.ReadFile("fonts/NotoSansMono-Regular.ttf")
	os.WriteFile(filepath.Join(dir, "late.ttf"), mono, 0o644)
	later := time.Now().Add(time.Second)
	os.Chtimes(filepath.Join(dir, "late.ttf"), later, later)
	if samePixels(after, drawWord(NewThemedCanvas(300, 60, FontDefault, 14, th), "Hamburg")) {
		t.Error("a replaced font file is not picked up")
	}
}
