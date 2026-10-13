package render

import (
	"strings"
	"testing"
)

func inkBox(c *Canvas) (x0, y0, x1, y1 int, ok bool) {
	x0, y0 = c.W, c.H
	for y := 0; y < c.H; y++ {
		for x := 0; x < c.W; x++ {
			if b, col := c.InkAt(x, y); b || col {
				x0, y0, x1, y1, ok = min(x0, x), min(y0, y), max(x1, x), max(y1, y), true
			}
		}
	}
	return
}

func TestTextSitsOnBaseline(t *testing.T) {
	c := NewCanvas(300, 120, FontDefault, 20)
	st := Style{FontDisplay, 60}
	adv := c.Text(10, 90, "13", st, InkColour)
	x0, y0, x1, y1, ok := inkBox(c)
	if !ok {
		t.Fatal("nothing drawn")
	}
	if y1 > 91 || y1 < 85 {
		t.Errorf("digits end at y=%d, baseline 90", y1)
	}
	if cap := c.CapHeight(st); y1-y0+1 < cap-2 || y1-y0+1 > cap+2 {
		t.Errorf("ink height %d, cap height %d", y1-y0+1, cap)
	}
	if x0 < 10 || x1 > 10+adv {
		t.Errorf("ink %d..%d outside advance 10..%d", x0, x1, 10+adv)
	}
	if b, _ := c.InkAt(x0, y1); b {
		t.Error("colour text drawn on the black layer")
	}
}

func TestTextRightAndCenterAlign(t *testing.T) {
	st := Style{FontBold, 30}
	c := NewCanvas(400, 60, FontDefault, 20)
	c.TextRight(300, 45, "14°", st, InkBlack)
	_, _, x1, _, _ := inkBox(c)
	if x1 > 300 || x1 < 290 {
		t.Errorf("right-aligned text ends at %d", x1)
	}
	c2 := NewCanvas(400, 60, FontDefault, 20)
	c2.TextCenter(200, 45, "MMMM", st, InkBlack)
	x0, _, x1, _, _ := inkBox(c2)
	if mid := (x0 + x1) / 2; mid < 197 || mid > 203 {
		t.Errorf("centred text midpoint %d", mid)
	}
}

func TestFit(t *testing.T) {
	c := NewCanvas(10, 10, FontDefault, 20)
	st := Style{FontRegular, 24}
	if c.Fit("kurz", st, 500) != "kurz" {
		t.Error("short text changed")
	}
	if got := c.Fit("ein ziemlich langer Termintitel", st, 120); c.Advance(got, st) > 120 || got[len(got)-3:] != "…" {
		t.Errorf("Fit = %q", got)
	}
	if c.Fit("abc", st, 2) != "" {
		t.Error("expected empty result")
	}
}

func TestWrapStyleAndClip(t *testing.T) {
	c := NewCanvas(400, 100, FontDefault, 20)
	st := Style{Font: FontRegular, Size: 20}
	lines := c.WrapStyle("alpha beta  gamma\n delta", st, c.Advance("alpha beta", st), 1000)
	if len(lines) != 2 || lines[0] != "alpha beta" || lines[1] != "gamma delta" {
		t.Errorf("first/rest widths: %q", lines)
	}
	long := c.WrapStyle("Donaudampfschifffahrtsgesellschaft", st, 60, 60)
	if len(long) != 1 || c.Advance(long[0], st) > 60 || !strings.HasSuffix(long[0], "…") {
		t.Errorf("over-long word: %q", long)
	}
	if got := c.Clip([]string{"a", "b", "c"}, 2, st, 400); len(got) != 2 || got[1] != "b …" {
		t.Errorf("clip: %q", got)
	}
	if got := c.Clip([]string{"a"}, 2, st, 400); len(got) != 1 {
		t.Errorf("clip short: %q", got)
	}
}
