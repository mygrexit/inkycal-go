package epd

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"inkycal/internal/render"
)

func init() {
	traced["epd_13_in_3"] = blackOnly(NewEPD13in3)
	traced["epd_13_in_3_colour"] = NewEPD13in3b
}

func TestEPD13in3RejectsWrongFrame(t *testing.T) {
	for _, d := range []Display{NewEPD13in3(&fakeBus{}, noSleep), NewEPD13in3b(&fakeBus{}, noSleep)} {
		if err := d.Show(render.Frame{W: 960, H: 680}); err == nil {
			t.Fatal("expected error for a frame without planes")
		}
	}
}

// idleBus13in3 is a fakeBus whose BUSY reads 0, which this panel reports as idle.
func idleBus13in3() *fakeBus { return &fakeBus{alwaysBusy: true} }

func TestEPD13in3MergesRedIntoBlack(t *testing.T) {
	bus := idleBus13in3()
	f := render.Frame{W: 960, H: 680, Black: bytes.Repeat([]byte{0xFF}, plane13in3), Red: make([]byte, plane13in3)}
	f.Red[0] = 0x80
	if err := NewEPD13in3(bus, noSleep).Show(f); err != nil {
		t.Fatal(err)
	}
	if got := bus.data[0x24]; got[0] != 0x7F || got[1] != 0xFF {
		t.Errorf("black plane starts %#x %#x, want red pixel merged into black", got[0], got[1])
	}
	if _, ok := bus.data[0x26]; ok {
		t.Error("black/white panel must not get a red plane")
	}
}

// busyHigh13in3 keeps BUSY high, which this panel reports as busy.
type busyHigh13in3 struct{ fakeBus }

func (b *busyHigh13in3) BusyLevel() (int, error) { return 1, nil }

func TestEPD13in3BusyTimeoutClosesBus(t *testing.T) {
	bus := &busyHigh13in3{}
	d := NewEPD13in3b(bus, noSleep)
	d.(*panel).BusyTimeout = 50 * time.Millisecond
	f := render.Frame{W: 960, H: 680, Black: make([]byte, plane13in3), Red: make([]byte, plane13in3)}
	if err := d.Show(f); err == nil {
		t.Fatal("expected busy timeout")
	}
	if bus.closed == 0 {
		t.Fatal("bus must be closed after an error")
	}
}

func TestEPD13in3ClearCycles(t *testing.T) {
	for _, tc := range []struct {
		d    func(Bus, func(time.Duration)) Display
		want int
	}{{NewEPD13in3, 4}, {NewEPD13in3b, 6}} {
		bus := idleBus13in3()
		if err := tc.d(bus, noSleep).Clear(2); err != nil {
			t.Fatal(err)
		}
		ops := strings.Join(bus.ops, " ")
		if n := strings.Count(ops, "C20"); n != tc.want {
			t.Errorf("expected %d refreshes for 2 cycles, got %d", tc.want, n)
		}
		if !strings.HasSuffix(ops, "C10 D03 close") {
			t.Errorf("clear must end in deep sleep: %s", ops[len(ops)-40:])
		}
	}
}
