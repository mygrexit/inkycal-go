package render

import (
	"image/color"
	"testing"
)

func bitSet(plane []byte, w, x, y int) bool {
	stride := (w + 7) / 8
	return plane[y*stride+x/8]&(0x80>>(x%8)) != 0
}

func TestPackRotatesPortraitCounterClockwise(t *testing.T) {
	c := NewCanvas(4, 8, FontDefault, 14) // portrait for an 8x4 panel
	c.set(3, 0, InkBlack)                 // top-right of portrait → top-left of panel
	c.set(0, 0, InkColour)                // top-left of portrait → bottom-left of panel
	f, err := Pack(c, 8, 4, 0)
	if err != nil {
		t.Fatal(err)
	}
	if bitSet(f.Black, 8, 0, 0) {
		t.Error("expected black (bit 0) at panel (0,0)")
	}
	if !bitSet(f.Red, 8, 0, 3) {
		t.Error("expected red at panel (0,3)")
	}
	if !bitSet(f.Black, 8, 7, 3) || bitSet(f.Red, 8, 7, 3) {
		t.Error("untouched pixel must be white")
	}
}

func TestPackOrientation180(t *testing.T) {
	c := NewCanvas(8, 4, FontDefault, 14) // already landscape
	c.set(0, 0, InkBlack)
	f, err := Pack(c, 8, 4, 180)
	if err != nil {
		t.Fatal(err)
	}
	if bitSet(f.Black, 8, 7, 3) {
		t.Error("expected black at (7,3) after 180° flip")
	}
	if !bitSet(f.Black, 8, 0, 0) {
		t.Error("(0,0) must be white after flip")
	}
}

func TestPackRejectsWrongSize(t *testing.T) {
	if _, err := Pack(NewCanvas(5, 5, FontDefault, 14), 8, 4, 0); err == nil {
		t.Fatal("expected error")
	}
}

func TestHashChangesWithContent(t *testing.T) {
	c := NewCanvas(8, 4, FontDefault, 14)
	a, _ := Pack(c, 8, 4, 0)
	b, _ := Pack(c, 8, 4, 0)
	if a.Hash() != b.Hash() {
		t.Fatal("same content, different hash")
	}
	c.set(1, 1, InkColour)
	d, _ := Pack(c, 8, 4, 0)
	if a.Hash() == d.Hash() {
		t.Fatal("different content, same hash")
	}
}

func TestPreviewColours(t *testing.T) {
	c := NewCanvas(8, 4, FontDefault, 14)
	c.set(1, 1, InkBlack)
	c.set(2, 1, InkColour)
	f, _ := Pack(c, 8, 4, 0)
	p := f.Preview()
	if p.RGBAAt(1, 1) != (color.RGBA{0, 0, 0, 255}) || p.RGBAAt(2, 1) != (color.RGBA{255, 0, 0, 255}) || p.RGBAAt(0, 0) != (color.RGBA{255, 255, 255, 255}) {
		t.Error("preview colours wrong")
	}
}

func TestAssembleStacksAndCentres(t *testing.T) {
	top := NewCanvas(80, 20, FontDefault, 14)
	top.FillCircle(40, 10, 3, InkBlack)
	bottom := NewCanvas(80, 20, FontDefault, 14)
	bottom.FillCircle(40, 10, 3, InkColour)
	out := Assemble(100, 200, []Part{{top, 100, 40}, {bottom, 100, 40}}, Layout{})
	if b, _ := out.InkAt(50, 20); !b { // x: (100-80)/2+40, y: (40-20)/2+10
		t.Error("first module not centred in its section")
	}
	if _, col := out.InkAt(50, 60); !col { // y: 40 + 10 + 10
		t.Error("second module not placed below the first")
	}
}

func TestAssembleBorderAndInfo(t *testing.T) {
	m := NewCanvas(100, 50, FontDefault, 14)
	out := Assemble(100, 120, []Part{{m, 100, 50}}, Layout{Border: true, Info: "Battery: 80%", InfoHeight: 30})
	if b, _ := out.InkAt(50, 1); !b {
		t.Error("border missing")
	}
	if inkIn(out, 0, 90, 100, 120, false) == 0 {
		t.Error("info text missing")
	}
	if inkIn(out, 0, 55, 100, 90, false) != 0 {
		t.Error("unexpected ink between module and info")
	}
}

func TestAssembleDividers(t *testing.T) {
	a, b := NewCanvas(80, 20, FontDefault, 14), NewCanvas(80, 20, FontDefault, 14)
	out := Assemble(100, 80, []Part{{a, 100, 40}, {b, 100, 40}}, Layout{Dividers: true})
	if bl, _ := out.InkAt(50, 40); !bl {
		t.Error("no divider at the section boundary")
	}
	if bl, _ := out.InkAt(5, 40); bl {
		t.Error("divider wider than the module")
	}
	if inkIn(out, 0, 0, 100, 39, false) != 0 {
		t.Error("divider above the first section")
	}
}
