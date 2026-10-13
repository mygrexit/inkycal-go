package epd

import (
	"testing"

	"inkycal/internal/render"
)

func TestMergedBlackTurnsRedIntoBlack(t *testing.T) {
	f := render.Frame{Black: []byte{0xFF, 0x0F}, Red: []byte{0x80, 0xF0}}
	got := mergedBlack(f)
	if got[0] != 0x7F || got[1] != 0x0F {
		t.Errorf("merged = %#x %#x", got[0], got[1])
	}
	if f.Black[0] != 0xFF {
		t.Error("input frame modified")
	}
}
