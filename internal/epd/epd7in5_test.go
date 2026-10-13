package epd

import (
	"bytes"
	"errors"
	"strings"
	"testing"
	"time"

	"inkycal/internal/render"
)

func init() {
	traced["epd_7_in_5"] = blackOnly(NewEPD7in5)
	traced["epd_7_in_5_colour"] = NewEPD7in5Colour
	traced["epd_7_in_5_v2"] = blackOnly(NewEPD7in5V2)
	traced["epd_7_in_5_v3"] = blackOnly(NewEPD7in5V3)
	traced["epd_7_in_5_v3_colour"] = NewEPD7in5V3Colour
}

var epd75Models = []struct {
	model  string
	colour bool
	new    func(Bus, func(time.Duration)) Display
}{
	{"epd_7_in_5", false, NewEPD7in5},
	{"epd_7_in_5_colour", true, NewEPD7in5Colour},
	{"epd_7_in_5_v2", false, NewEPD7in5V2},
	{"epd_7_in_5_v3", false, NewEPD7in5V3},
	{"epd_7_in_5_v3_colour", true, NewEPD7in5V3Colour},
}

func TestEPD75RejectsWrongFrame(t *testing.T) {
	for _, m := range epd75Models {
		bus := &traceBus{}
		if err := m.new(bus, noSleep).Show(render.Frame{W: 10, H: 10}); err == nil {
			t.Errorf("%s: expected error", m.model)
		}
		if len(bus.ops) != 0 {
			t.Errorf("%s: touched the bus for a bad frame: %v", m.model, bus.ops)
		}
	}
}

func TestEPD75SizeMatchesModels(t *testing.T) {
	for _, m := range epd75Models {
		w, h := m.new(&traceBus{}, noSleep).Size()
		if Models[m.model] != [2]int{w, h} {
			t.Errorf("%s: Size %dx%d, Models %v", m.model, w, h, Models[m.model])
		}
	}
}

// v1/v2 panels signal busy with BUSY low, v3 with BUSY high.
func TestEPD75BusyTimeoutClosesBus(t *testing.T) {
	for _, m := range epd75Models {
		bus := &fakeBus{alwaysBusy: true}
		d := m.new(bus, noSleep)
		if strings.HasPrefix(m.model, "epd_7_in_5_v3") {
			d = m.new(&stuckHighBus{bus}, noSleep)
		}
		w, h := d.Size()
		f := render.Frame{W: w, H: h, Black: make([]byte, w/8*h), Red: make([]byte, w/8*h)}
		if err := d.Show(f); err == nil {
			t.Errorf("%s: expected busy timeout", m.model)
		}
		if bus.closed == 0 {
			t.Errorf("%s: bus must be closed after an error", m.model)
		}
	}
}

type stuckHighBus struct{ *fakeBus }

func (b *stuckHighBus) BusyLevel() (int, error) { return 1, nil }

type failBus struct{ traceBus }

func (b *failBus) Data(p ...byte) error { return errors.New("spi broken") }
func (b *failBus) Close() error         { b.add("close"); return nil }

func TestEPD75CloseOnError(t *testing.T) {
	for _, m := range epd75Models {
		bus := &failBus{}
		d := m.new(bus, noSleep)
		w, h := d.Size()
		f := render.Frame{W: w, H: h, Black: make([]byte, w/8*h), Red: make([]byte, w/8*h)}
		if err := d.Show(f); err == nil {
			t.Errorf("%s: expected error", m.model)
		}
		if n := len(bus.ops); n == 0 || bus.ops[n-1] != "close" {
			t.Errorf("%s: bus not closed on error: %v", m.model, bus.ops)
		}
	}
}

func TestEPD75ClearCycles(t *testing.T) {
	refreshCmd := map[string]string{
		"epd_7_in_5": "cmd 12", "epd_7_in_5_colour": "cmd 12", "epd_7_in_5_v2": "cmd 12",
		"epd_7_in_5_v3": "cmd 22\ndata f7\ncmd 20", "epd_7_in_5_v3_colour": "cmd 22\ndata c7\ncmd 20",
	}
	for _, m := range epd75Models {
		bus := &traceBus{}
		if err := m.new(bus, noSleep).Clear(2); err != nil {
			t.Fatal(err)
		}
		bus.flush()
		log := strings.Join(bus.ops, "\n")
		want := 4
		if m.colour {
			want = 6
		}
		if got := strings.Count(log, refreshCmd[m.model]); got != want {
			t.Errorf("%s: %d refreshes for 2 cycles, want %d", m.model, got, want)
		}
		if strings.Count(log, "open") != 1 || !strings.HasSuffix(log, "close") {
			t.Errorf("%s: Clear must be one session ending in close", m.model)
		}
	}
}

// B/W panels show red pixels as black (mergedBlack).
func TestEPD75BWMergesRed(t *testing.T) {
	bus := &fakeBus{}
	d := NewEPD7in5V2(bus, noSleep)
	n := 800 / 8 * 480
	f := render.Frame{W: 800, H: 480, Black: bytes.Repeat([]byte{0xFF}, n), Red: make([]byte, n)}
	f.Red[0] = 0x80
	if err := d.Show(f); err != nil {
		t.Fatal(err)
	}
	if got := bus.data[0x13][0]; got != 0x80 { // panel RAM: 1 = black
		t.Errorf("first byte %02x, want 80 (red pixel drawn black)", got)
	}
}

func TestPixels4bpp(t *testing.T) {
	// pixels: black, white, red (over black), white, white, black, red, white
	black := []byte{0b01011011}
	red := []byte{0b00100010}
	want := []byte{0x03, 0x43, 0x30, 0x43}
	if got := pixels4bpp(black, red); !bytes.Equal(got, want) {
		t.Errorf("got % x, want % x", got, want)
	}
	if got := pixels4bpp(black, nil); !bytes.Equal(got, []byte{0x03, 0x03, 0x30, 0x33}) {
		t.Errorf("b/w: got % x", got)
	}
}
