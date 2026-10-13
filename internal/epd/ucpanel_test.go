package epd

import (
	"strings"
	"testing"
	"time"

	"inkycal/internal/render"
)

// ucBlackOnly feeds a black/white driver only the black band, as trace.py does
// upstream (main.py merges colour into black before the driver sees it).
type ucBlackOnly struct{ Display }

func (d ucBlackOnly) Show(f render.Frame) error {
	f.Red = make([]byte, len(f.Red))
	return d.Display.Show(f)
}

func blackOnly(newDriver func(Bus, func(time.Duration)) Display) func(Bus, func(time.Duration)) Display {
	return func(b Bus, s func(time.Duration)) Display { return ucBlackOnly{newDriver(b, s)} }
}

// ucModels are the drivers built on ucPanel, with their colour support.
var ucModels = []struct {
	model     string
	newDriver func(Bus, func(time.Duration)) Display
	colour    bool
}{
	{"epd_4_in_2", NewEPD4in2, false},
	{"epd_4_in_2_colour", NewEPD4in2Colour, true},
	{"epd_5_in_83", NewEPD5in83, false},
	{"epd_5_in_83_colour", NewEPD5in83Colour, true},
	{"epd_5_in_83_V2", NewEPD5in83V2, false},
	{"epd5in83b_V2", NewEPD5in83bV2, false},
}

func ucFrame(model string) render.Frame {
	s := Models[model]
	n := s[0] / 8 * s[1]
	return render.Frame{W: s[0], H: s[1], Black: make([]byte, n), Red: make([]byte, n)}
}

func TestUCPanelsRejectWrongFrame(t *testing.T) {
	for _, m := range ucModels {
		bus := &fakeBus{}
		if err := m.newDriver(bus, noSleep).Show(render.Frame{W: 10, H: 10}); err == nil {
			t.Errorf("%s: expected error", m.model)
		}
		f := ucFrame(m.model)
		f.Red = nil
		if err := m.newDriver(bus, noSleep).Show(f); err == nil {
			t.Errorf("%s: expected error for missing red plane", m.model)
		}
		if len(bus.ops) != 0 {
			t.Errorf("%s: bus touched for a rejected frame", m.model)
		}
		if w, h := m.newDriver(bus, noSleep).Size(); [2]int{w, h} != Models[m.model] {
			t.Errorf("%s: Size %dx%d", m.model, w, h)
		}
	}
}

func TestUCPanelsBusyTimeoutClosesBus(t *testing.T) {
	for _, m := range ucModels {
		bus := &fakeBus{alwaysBusy: true}
		d := m.newDriver(bus, noSleep)
		d.(*ucPanel).BusyTimeout = 50 * time.Millisecond
		if err := d.Show(ucFrame(m.model)); err == nil {
			t.Fatalf("%s: expected busy timeout", m.model)
		}
		if bus.closed == 0 {
			t.Fatalf("%s: bus must be closed after an error", m.model)
		}
	}
}

func TestUCPanelsClearCycles(t *testing.T) {
	for _, m := range ucModels {
		bus := &fakeBus{}
		if err := m.newDriver(bus, noSleep).Clear(2); err != nil {
			t.Fatal(err)
		}
		ops := strings.Join(bus.ops, " ")
		want := 4 // black, white per cycle
		if m.colour {
			want = 6 // black, colour, white per cycle
		}
		if got := strings.Count(ops, "C12"); got != want {
			t.Errorf("%s: %d refreshes for 2 cycles, want %d", m.model, got, want)
		}
		if strings.Count(ops, "open") != 1 || !strings.HasSuffix(ops, "C07 DA5 close") {
			t.Errorf("%s: calibration must be one session ending in deep sleep: %s", m.model, ops)
		}
	}
}

// Black/white panels show red content as black (upstream main.py's _merge_bands).
func TestUCPanelsMergeRedOnBlackWhite(t *testing.T) {
	cases := []struct {
		model string
		cmd   byte
		want  func(n int) []byte // what the panel gets for an all-red frame
	}{
		{"epd_4_in_2", 0x13, func(n int) []byte { return make([]byte, n) }},
		{"epd_5_in_83", 0x10, func(n int) []byte { return make([]byte, n*4) }},
		{"epd_5_in_83_V2", 0x13, func(n int) []byte { return ucFilled(n, 0xFF) }},
		{"epd5in83b_V2", 0x10, func(n int) []byte { return make([]byte, n) }},
	}
	newDriver := map[string]func(Bus, func(time.Duration)) Display{}
	for _, m := range ucModels {
		newDriver[m.model] = m.newDriver
	}
	for _, c := range cases {
		f := ucFrame(c.model)
		for i := range f.Black {
			f.Black[i], f.Red[i] = 0xFF, 0xFF
		}
		bus := &fakeBus{}
		if err := newDriver[c.model](bus, noSleep).Show(f); err != nil {
			t.Fatal(err)
		}
		if string(bus.data[c.cmd]) != string(c.want(len(f.Black))) {
			t.Errorf("%s: red not shown as black", c.model)
		}
	}
}
