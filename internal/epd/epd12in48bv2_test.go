package epd

import (
	"bytes"
	"fmt"
	"strings"
	"testing"

	"inkycal/internal/render"
)

type fakeBus12 struct {
	ops        []string
	lastCmd    map[Part]byte
	data       map[string][]byte // "S2:10" → bytes
	busyPolls  int
	alwaysBusy bool
	temp       byte
	closed     int
}

func (b *fakeBus12) Open() error  { b.ops = append(b.ops, "open"); return nil }
func (b *fakeBus12) Close() error { b.ops = append(b.ops, "close"); b.closed++; return nil }
func (b *fakeBus12) SetReset(h bool) error {
	b.ops = append(b.ops, fmt.Sprintf("rst:%v", h))
	return nil
}
func (b *fakeBus12) Command(p Part, c byte) error {
	if b.lastCmd == nil {
		b.lastCmd = map[Part]byte{}
	}
	b.lastCmd[p] = c
	if c != 0x71 {
		b.ops = append(b.ops, fmt.Sprintf("%v:C%02X", p, c))
	}
	return nil
}
func (b *fakeBus12) Data(p Part, d ...byte) error {
	if b.data == nil {
		b.data = map[string][]byte{}
	}
	k := fmt.Sprintf("%v:%02X", p, b.lastCmd[p])
	b.data[k] = append(b.data[k], d...)
	if len(d) <= 4 {
		b.ops = append(b.ops, fmt.Sprintf("%v:D % X", p, d))
	} else {
		b.ops = append(b.ops, fmt.Sprintf("%v:D[%d]", p, len(d)))
	}
	return nil
}
func (b *fakeBus12) BusyLevel(p Part) (int, error) {
	if b.alwaysBusy {
		return 0, nil
	}
	if b.busyPolls > 0 {
		b.busyPolls--
		return 0, nil
	}
	return 1, nil
}
func (b *fakeBus12) ReadData(p Part) (byte, error) {
	b.ops = append(b.ops, fmt.Sprintf("%v:read", p))
	return b.temp, nil
}

func (b *fakeBus12) has(op string) bool {
	for _, o := range b.ops {
		if o == op {
			return true
		}
	}
	return false
}

func TestEPD12in48InitSequence(t *testing.T) {
	bus := &fakeBus12{temp: 0x17}
	e := NewEPD12in48bV2(bus, noSleep)
	if err := e.Show(testFrame12()); err != nil {
		t.Fatal(err)
	}
	got := strings.Join(bus.ops[:8], " ")
	want := "open rst:true rst:false rst:true M1:C00 M1:D 0F S1:C00 S1:D 0F"
	if got != want {
		t.Fatalf("start of sequence\n got %s\nwant %s", got, want)
	}
	for _, op := range []string{
		"M2:D 03", "S2:D 03",
		"M1:D 02 88 01 EC", "S1:D 02 90 01 EC", "M2:D 02 90 01 EC", "S2:D 02 88 01 EC",
		"ALL:C15", "ALL:D 20", "ALL:C50", "ALL:D 11 07", "ALL:C60", "ALL:D 22",
		"M1:C40", "M1:read", "ALL:CE0", "ALL:D 03", "ALL:CE5", "ALL:D 17",
		"M1+M2:C04", "ALL:C12", "ALL:C02", "ALL:C07", "ALL:D A5", "close",
	} {
		if !bus.has(op) {
			t.Errorf("missing %q in %v", op, bus.ops)
		}
	}
	if got := bus.data["ALL:20"]; len(got) != 60 || got[1] != 0x10 {
		t.Errorf("vcom LUT: %d bytes", len(got))
	}
}

// testFrame12 marks every byte with its row and column so slicing errors show up.
func testFrame12() render.Frame {
	f := render.Frame{W: 1304, H: 984, Black: make([]byte, 163*984), Red: make([]byte, 163*984)}
	for y := 0; y < 984; y++ {
		for x := 0; x < 163; x++ {
			f.Black[y*163+x] = byte(y*7 + x)
			f.Red[y*163+x] = byte(y*3 + x*5)
		}
	}
	return f
}

func region(plane []byte, y0, y1, x0, x1 int) []byte {
	var out []byte
	for y := y0; y < y1; y++ {
		out = append(out, plane[y*163+x0:y*163+x1]...)
	}
	return out
}

func TestEPD12in48SplitsFrameIntoFourParts(t *testing.T) {
	bus := &fakeBus12{}
	f := testFrame12()
	if err := NewEPD12in48bV2(bus, noSleep).Show(f); err != nil {
		t.Fatal(err)
	}
	for _, c := range []struct {
		part           string
		y0, y1, x0, x1 int
	}{
		{"S2", 0, 492, 0, 81}, {"M2", 0, 492, 81, 163},
		{"M1", 492, 984, 0, 81}, {"S1", 492, 984, 81, 163},
	} {
		if !bytes.Equal(bus.data[c.part+":10"], region(f.Black, c.y0, c.y1, c.x0, c.x1)) {
			t.Errorf("%s black plane wrong (%d bytes)", c.part, len(bus.data[c.part+":10"]))
		}
		if !bytes.Equal(bus.data[c.part+":13"], region(f.Red, c.y0, c.y1, c.x0, c.x1)) {
			t.Errorf("%s red plane wrong", c.part)
		}
	}
}

func TestEPD12in48RejectsWrongFrameSize(t *testing.T) {
	bus := &fakeBus12{}
	err := NewEPD12in48bV2(bus, noSleep).Show(render.Frame{W: 800, H: 480})
	if err == nil || len(bus.ops) != 0 {
		t.Fatalf("err=%v ops=%v", err, bus.ops)
	}
}

func TestEPD12in48BusyTimeoutClosesBus(t *testing.T) {
	bus := &fakeBus12{alwaysBusy: true}
	e := NewEPD12in48bV2(bus, noSleep)
	if err := e.Show(testFrame12()); err == nil {
		t.Fatal("expected busy timeout")
	}
	if bus.closed != 1 {
		t.Fatalf("closed %d times", bus.closed)
	}
}

func TestEPD12in48ClearEndsWhite(t *testing.T) {
	bus := &fakeBus12{}
	if err := NewEPD12in48bV2(bus, noSleep).Clear(1); err != nil {
		t.Fatal(err)
	}
	black, red := bus.data["M1:10"], bus.data["M1:13"]
	n := 81 * 492
	if len(black) != 3*n || len(red) != 3*n {
		t.Fatalf("got %d/%d bytes, want 3 refreshes", len(black), len(red))
	}
	last := func(p []byte) []byte { return p[2*n:] }
	if bytes.Count(last(black), []byte{0xFF}) != n || bytes.Count(last(red), []byte{0x00}) != n {
		t.Fatal("last refresh is not white")
	}
}
