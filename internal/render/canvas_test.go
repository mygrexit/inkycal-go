package render

import "testing"

// inkIn counts ink pixels of one layer inside a rectangle.
func inkIn(c *Canvas, x0, y0, x1, y1 int, colour bool) int {
	n := 0
	for y := y0; y < y1; y++ {
		for x := x0; x < x1; x++ {
			b, col := c.InkAt(x, y)
			if (!colour && b) || (colour && col) {
				n++
			}
		}
	}
	return n
}

func TestNewCanvasIsWhite(t *testing.T) {
	c := NewCanvas(10, 5, FontDefault, 14)
	if inkIn(c, 0, 0, 10, 5, false)+inkIn(c, 0, 0, 10, 5, true) != 0 {
		t.Fatal("new canvas has ink")
	}
}

func TestWriteAlignment(t *testing.T) {
	c := NewCanvas(300, 40, FontDefault, 14)
	c.Write(0, 0, 300, 40, "Hi", TextOpts{Align: AlignLeft})
	if inkIn(c, 0, 0, 40, 40, false) == 0 || inkIn(c, 150, 0, 300, 40, false) != 0 {
		t.Error("left aligned text not on the left")
	}
	c = NewCanvas(300, 40, FontDefault, 14)
	c.Write(0, 0, 300, 40, "Hi", TextOpts{Align: AlignRight})
	if inkIn(c, 260, 0, 300, 40, false) == 0 || inkIn(c, 0, 0, 150, 40, false) != 0 {
		t.Error("right aligned text not on the right")
	}
	c = NewCanvas(300, 40, FontDefault, 14)
	c.Write(0, 0, 300, 40, "Hi", TextOpts{})
	if inkIn(c, 130, 0, 170, 40, false) == 0 || inkIn(c, 0, 0, 100, 40, false) != 0 {
		t.Error("centred text not centred")
	}
}

func TestWriteColourOnlyTouchesColourLayer(t *testing.T) {
	c := NewCanvas(100, 30, FontDefault, 14)
	c.Write(0, 0, 100, 30, "Red", TextOpts{Ink: InkColour})
	if inkIn(c, 0, 0, 100, 30, true) == 0 || inkIn(c, 0, 0, 100, 30, false) != 0 {
		t.Error("InkColour must draw on colour layer only")
	}
}

func TestWriteClipsToBox(t *testing.T) {
	c := NewCanvas(200, 100, FontDefault, 40)
	c.Write(50, 40, 60, 10, "WWWWWWWW", TextOpts{Align: AlignLeft})
	if inkIn(c, 0, 0, 200, 40, false) != 0 || inkIn(c, 0, 50, 200, 100, false) != 0 || inkIn(c, 110, 0, 200, 100, false) != 0 {
		t.Error("text drawn outside its box")
	}
	if inkIn(c, 50, 40, 110, 50, false) == 0 {
		t.Error("first line must always be drawn, even if taller than the box")
	}
}

func TestWrap(t *testing.T) {
	c := NewCanvas(10, 10, FontDefault, 14)
	lines := c.Wrap("one two three four five six", c.TextWidth("one two three"))
	if len(lines) != 2 || lines[0] != "one two three" || lines[1] != "four five six" {
		t.Errorf("wrap: %q", lines)
	}
	if got := c.Wrap("Supercalifragilistic", 5); len(got) != 1 {
		t.Errorf("long word must stay one line: %q", got)
	}
}

func TestWriteDropsLinesThatDoNotFit(t *testing.T) {
	c := NewCanvas(60, 200, FontDefault, 14)
	lh := c.LineHeight()
	c.Write(0, 0, 60, lh, "aaa bbb ccc ddd eee fff", TextOpts{Align: AlignLeft})
	if inkIn(c, 0, lh, 60, 200, false) != 0 {
		t.Error("lines below the box were drawn")
	}
}

func TestAutofitDoesNotChangeCanvasFont(t *testing.T) {
	c := NewCanvas(400, 100, FontDefault, 14)
	c.Write(0, 0, 400, 100, "March", TextOpts{Autofit: true})
	if id, size := c.Font(); id != FontDefault || size != 14 {
		t.Errorf("font changed to %d/%d", id, size)
	}
	if inkIn(c, 0, 0, 400, 30, false) == 0 || inkIn(c, 0, 70, 400, 100, false) == 0 {
		t.Error("autofit text should span most of the box height")
	}
}

func TestIconCentred(t *testing.T) {
	c := NewCanvas(100, 100, FontDefault, 14)
	c.Icon(0, 0, 100, 100, "", FontWeather, InkColour, 0.9) // day-sunny
	if inkIn(c, 40, 40, 60, 60, true) == 0 {
		t.Error("icon centre has no ink")
	}
	if inkIn(c, 0, 0, 100, 3, true) != 0 || inkIn(c, 0, 97, 100, 100, true) != 0 {
		t.Error("icon touches the box edge despite fill ratio 0.9")
	}
}

func TestInkWhiteOnFilledCircle(t *testing.T) {
	c := NewCanvas(60, 60, FontDefault, 14)
	c.FillCircle(30, 30, 20, InkBlack)
	c.FillCircle(30, 30, 20, InkColour)
	if b, col := c.InkAt(30, 30); !b || !col {
		t.Fatal("circle centre must be ink on both layers")
	}
	c.Write(10, 10, 40, 40, "8", TextOpts{Ink: InkWhite, Autofit: true})
	white := 0
	for y := 10; y < 50; y++ {
		for x := 10; x < 50; x++ {
			if b, _ := c.InkAt(x, y); !b && (x-30)*(x-30)+(y-30)*(y-30) < 400 {
				white++
			}
		}
	}
	if white == 0 {
		t.Error("white text not visible inside circle")
	}
}

func TestRoundedRectAndBorder(t *testing.T) {
	c := NewCanvas(50, 30, FontDefault, 14)
	c.RoundedRect(0, 0, 50, 30, 5, InkBlack)
	if b, _ := c.InkAt(25, 0); !b {
		t.Error("top edge missing")
	}
	if b, _ := c.InkAt(25, 29); !b {
		t.Error("bottom edge missing")
	}
	if b, _ := c.InkAt(0, 0); b {
		t.Error("rounded corner pixel should stay white")
	}
	if b, _ := c.InkAt(25, 15); b {
		t.Error("outline must not be filled")
	}
	d := NewCanvas(100, 100, FontDefault, 14)
	Border(d, 0, 0, 100, 100, 0, 0.1, 0.1, InkColour)
	if _, col := d.InkAt(5, 50); !col {
		t.Error("shrunk border left edge expected at x=5")
	}
	if _, col := d.InkAt(0, 50); col {
		t.Error("border not shrunk")
	}
}

func TestPasteOnlyCopiesInk(t *testing.T) {
	dst := NewCanvas(20, 20, FontDefault, 14)
	dst.FillCircle(10, 10, 9, InkBlack)
	src := NewCanvas(5, 5, FontDefault, 14)
	src.HLine(0, 4, 2, InkColour)
	dst.Paste(src, 8, 8)
	if b, _ := dst.InkAt(9, 9); !b {
		t.Error("paste overwrote destination ink with white")
	}
	if _, col := dst.InkAt(10, 10); !col {
		t.Error("colour ink not pasted")
	}
}

func TestZeroSizedBoxesDrawNothing(t *testing.T) {
	c := NewCanvas(50, 50, FontDefault, 14)
	c.Write(10, 10, 0, 20, "x", TextOpts{})
	c.Write(10, 10, 20, -5, "x", TextOpts{})
	c.Icon(10, 10, -3, 20, "\uf00d", FontWeather, InkBlack, 0.9)
	if inkIn(c, 0, 0, 50, 50, false) != 0 {
		t.Error("drawing into an empty box produced ink")
	}
}
