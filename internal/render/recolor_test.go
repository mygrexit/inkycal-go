package render

import "testing"

func sample() *Canvas {
	c := NewCanvas(60, 40, FontDefault, 14)
	c.FillRect(5, 5, 10, 10, InkBlack) // "text"
	c.FillCircle(40, 20, 6, InkColour) // "accent"
	return c
}

func TestRecolorInverts(t *testing.T) {
	c := sample()
	c.Recolor(InkMap{Paper: InkBlack, Ink: InkWhite, Accent: InkColour})
	if b, col := c.InkAt(1, 1); !b || col {
		t.Error("paper should be black")
	}
	if b, col := c.InkAt(8, 8); b || col {
		t.Error("text should be white")
	}
	if _, col := c.InkAt(40, 20); !col {
		t.Error("accent should stay colour")
	}
}

func TestRecolorAccentToBlack(t *testing.T) {
	c := sample()
	c.Recolor(InkMap{Paper: InkWhite, Ink: InkBlack, Accent: InkBlack})
	for y := 0; y < c.H; y++ {
		for x := 0; x < c.W; x++ {
			if _, col := c.InkAt(x, y); col {
				t.Fatalf("colour ink left at %d,%d", x, y)
			}
		}
	}
	if b, _ := c.InkAt(40, 20); !b {
		t.Error("accent should now be black")
	}
}

func TestDefaultInksChangeNothing(t *testing.T) {
	a, b := sample(), sample()
	b.Recolor(DefaultInks)
	if !samePixels(a, b) {
		t.Error("default ink map changed pixels")
	}
}

// White text on a black tile survives assembly: the section is filled with the tile's
// paper and the module pasted opaquely.
func TestAssembleFillsPaperAndPastesOpaque(t *testing.T) {
	inv := InkMap{Paper: InkBlack, Ink: InkWhite, Accent: InkColour}
	img := sample()
	img.Recolor(inv)
	out := AssembleGrid(100, 60, []Placed{{Img: img, X: 20, Y: 10, W: 80, H: 50, Inks: &inv}}, Layout{})
	if b, _ := out.InkAt(21, 11); !b {
		t.Error("section padding is not the tile's paper")
	}
	if b, col := out.InkAt(30+8, 15+8); b || col {
		t.Error("white text vanished on paste")
	}
	if b, _ := out.InkAt(5, 5); b {
		t.Error("outside the section must stay white")
	}
	legacy := Assemble(80, 60, []Part{{Img: img, SectionW: 80, SectionH: 50, Inks: &inv}}, Layout{})
	if b, _ := legacy.InkAt(1, 1); !b {
		t.Error("stacked layout: section not filled")
	}
}
