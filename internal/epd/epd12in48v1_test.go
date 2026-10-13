package epd

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"inkycal/internal/render"
)

// traceBus12 logs what a 12.48" driver sends in the format of tools/epdtrace/trace12.py:
// every byte with the parts it is selected for, consecutive data to the same parts
// merged, BUSY reads alternating 1, 0, 1, 0 … across all parts, temperature reads 0x17.
type traceBus12 struct {
	ops       []string
	data      []byte
	dataParts Part
	busy      int
}

func (b *traceBus12) flush() {
	if b.data == nil {
		return
	}
	if len(b.data) > 32 {
		sum := sha256.Sum256(b.data)
		b.ops = append(b.ops, fmt.Sprintf("data %v %d sha256:%s", b.dataParts, len(b.data), hex.EncodeToString(sum[:])[:16]))
	} else {
		b.ops = append(b.ops, fmt.Sprintf("data %v %s", b.dataParts, hex.EncodeToString(b.data)))
	}
	b.data = nil
}

func (b *traceBus12) add(op string) { b.flush(); b.ops = append(b.ops, op) }

func (b *traceBus12) Open() error  { b.add("open"); return nil }
func (b *traceBus12) Close() error { b.add("close"); return nil }
func (b *traceBus12) SetReset(high bool) error {
	v := 0
	if high {
		v = 1
	}
	b.add(fmt.Sprintf("rst M1S1 %02x", v))
	b.add(fmt.Sprintf("rst M2S2 %02x", v))
	return nil
}
func (b *traceBus12) Command(p Part, c byte) error {
	b.add(fmt.Sprintf("cmd %v %02x", p, c))
	return nil
}
func (b *traceBus12) Data(p Part, d ...byte) error {
	if b.data != nil && b.dataParts != p {
		b.flush()
	}
	if b.data == nil {
		b.data, b.dataParts = []byte{}, p
	}
	b.data = append(b.data, d...)
	return nil
}
func (b *traceBus12) BusyLevel(Part) (int, error) { b.busy ^= 1; return b.busy, nil }
func (b *traceBus12) ReadData(p Part) (byte, error) {
	b.add(fmt.Sprintf("read %v dc=1", p))
	return 0x17, nil
}

// runTrace12 shows trace.py's pattern frame on d (driving bus) and returns the log
// and the golden trace of model.
func runTrace12(t *testing.T, model string, d Display, bus *traceBus12) (got, want string) {
	t.Helper()
	golden, err := os.ReadFile(filepath.Join("testdata", "trace", model+".txt"))
	if err != nil {
		t.Fatalf("golden trace missing (run tools/epdtrace/regen12.sh): %v", err)
	}
	if err := d.Show(patternFrame(w12, h12)); err != nil {
		t.Fatal(err)
	}
	bus.flush()
	return strings.Join(bus.ops, "\n") + "\n", string(golden)
}

// blackOnly12 hands a black/white driver the frame without its red plane, as the
// tracers give upstream B/W drivers only the black band.
type blackOnly12 struct{ Display }

func (d blackOnly12) Show(f render.Frame) error {
	f.Red = make([]byte, len(f.Black))
	return d.Display.Show(f)
}

func TestEPD12in48V1MatchesUpstreamTraces(t *testing.T) {
	for model, newDriver := range map[string]func(Bus12) Display{
		"epd_12_in_48":        func(b Bus12) Display { return blackOnly12{NewEPD12in48(b, noSleep)} },
		"epd_12_in_48_colour": func(b Bus12) Display { return NewEPD12in48b(b, noSleep) },
	} {
		t.Run(model, func(t *testing.T) {
			bus := &traceBus12{}
			got, want := runTrace12(t, model, newDriver(bus), bus)
			if got != want {
				t.Errorf("%s differs from upstream:\n%s", model, diffLines(want, got))
			}
		})
	}
}

// The hardware-tested V2 driver proves the tracer: its transfer is identical to
// upstream except that upstream's M2_ReadBusy sends one extra 0x71 (get status)
// before its loop, which the V2 port leaves out (harmless: a status read).
func TestEPD12in48bV2MatchesUpstreamTrace(t *testing.T) {
	bus := &traceBus12{}
	got, want := runTrace12(t, "epd_12_in_48_colour_V2", NewEPD12in48bV2(bus, noSleep), bus)
	const m2 = "cmd M2 71\n"
	adjusted := strings.Replace(want, m2+m2+m2, m2+m2, 1)
	if adjusted == want {
		t.Fatal("upstream trace no longer has the extra M2 status command; update this test")
	}
	if got != adjusted {
		t.Errorf("V2 differs from upstream beyond the known M2 status command:\n%s", diffLines(adjusted, got))
	}
}

func TestEPD12in48V1RejectsWrongFrame(t *testing.T) {
	for _, d := range []Display{NewEPD12in48(&fakeBus12{}, noSleep), NewEPD12in48b(&fakeBus12{}, noSleep)} {
		if err := d.Show(render.Frame{W: w12, H: h12}); err == nil {
			t.Fatal("expected error for a frame without planes")
		}
	}
}

func TestEPD12in48V1MergesRedIntoBlack(t *testing.T) {
	bus := &fakeBus12{}
	n := stride12 * h12
	f := render.Frame{W: w12, H: h12, Black: bytes.Repeat([]byte{0xFF}, n), Red: make([]byte, n)}
	f.Red[0] = 0x80 // top left pixel: S2's first byte
	if err := NewEPD12in48(bus, noSleep).Show(f); err != nil {
		t.Fatal(err)
	}
	if got := bus.data["S2:13"]; len(got) != 81*492 || got[0] != 0x7F || got[1] != 0xFF {
		t.Errorf("S2 plane starts % X (len %d), want red merged into black", got[:2], len(got))
	}
	for k := range bus.data {
		if strings.HasSuffix(k, ":10") {
			t.Errorf("black/white panel got a %s transfer", k)
		}
	}
}

func TestEPD12in48V1BusyTimeoutClosesBus(t *testing.T) {
	for _, colour := range []bool{false, true} {
		bus := &fakeBus12{alwaysBusy: true}
		d := NewEPD12in48(bus, noSleep)
		d.colour = colour
		d.BusyTimeout = 50 * time.Millisecond
		n := stride12 * h12
		if err := d.Show(render.Frame{W: w12, H: h12, Black: make([]byte, n), Red: make([]byte, n)}); err == nil {
			t.Fatal("expected busy timeout")
		}
		if bus.closed == 0 {
			t.Fatal("bus must be closed after an error")
		}
	}
}

func TestEPD12in48V1ClearCycles(t *testing.T) {
	for colour, want := range map[bool]int{false: 4, true: 6} {
		bus := &fakeBus12{}
		d := NewEPD12in48(bus, noSleep)
		d.colour = colour
		if err := d.Clear(2); err != nil {
			t.Fatal(err)
		}
		ops := strings.Join(bus.ops, " ")
		if n := strings.Count(ops, "ALL:C12"); n != want {
			t.Errorf("colour=%v: expected %d refreshes for 2 cycles, got %d", colour, want, n)
		}
		if !strings.HasSuffix(ops, "ALL:C07 ALL:D A5 close") {
			t.Errorf("colour=%v: clear must end in deep sleep", colour)
		}
	}
}
