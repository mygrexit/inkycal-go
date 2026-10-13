package epd

import (
	"fmt"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"inkycal/internal/render"
)

type fakeBus struct {
	ops        []string
	busyPolls  int // BusyLevel returns 0 this many times, then 1
	data       map[byte][]byte
	lastCmd    byte
	closed     int
	alwaysBusy bool
}

func (b *fakeBus) Open() error  { b.ops = append(b.ops, "open"); return nil }
func (b *fakeBus) Close() error { b.ops = append(b.ops, "close"); b.closed++; return nil }
func (b *fakeBus) SetReset(h bool) error {
	b.ops = append(b.ops, fmt.Sprintf("rst:%v", h))
	return nil
}
func (b *fakeBus) Command(c byte) error {
	b.lastCmd = c
	if c != 0x71 {
		b.ops = append(b.ops, fmt.Sprintf("C%02X", c))
	}
	return nil
}
func (b *fakeBus) Data(p ...byte) error {
	if b.data == nil {
		b.data = map[byte][]byte{}
	}
	b.data[b.lastCmd] = append(b.data[b.lastCmd], p...)
	if len(p) <= 4 {
		b.ops = append(b.ops, fmt.Sprintf("D% X", p))
	} else {
		b.ops = append(b.ops, fmt.Sprintf("D[%d]", len(p)))
	}
	return nil
}
func (b *fakeBus) BusyLevel() (int, error) {
	if b.alwaysBusy {
		return 0, nil
	}
	if b.busyPolls > 0 {
		b.busyPolls--
		return 0, nil
	}
	return 1, nil
}

func noSleep(time.Duration) {}

func TestShowSequence(t *testing.T) {
	bus := &fakeBus{busyPolls: 3}
	d := NewEPD7in5bV2(bus, noSleep)
	c := render.NewCanvas(480, 800, render.FontDefault, 14)
	c.FillCircle(240, 400, 50, render.InkColour)
	f, err := render.Pack(c, 800, 480, 0)
	if err != nil {
		t.Fatal(err)
	}
	if err := d.Show(f); err != nil {
		t.Fatal(err)
	}
	got := strings.Join(bus.ops, " ")
	want := "open rst:true rst:false rst:true C01 D07 07 3F 3F C04 C00 D0F C61 D03 20 01 E0 C15 D00 C50 D11 07 C60 D22 C65 D00 00 00 00 C10 D[48000] C13 D[48000] C12 C02 C07 DA5 close"
	if got != want {
		t.Errorf("ops:\n got %s\nwant %s", got, want)
	}
	if string(bus.data[0x10]) != string(f.Black) || string(bus.data[0x13]) != string(f.Red) {
		t.Error("planes not transferred unchanged")
	}
}

func TestShowRejectsWrongFrame(t *testing.T) {
	d := NewEPD7in5bV2(&fakeBus{}, noSleep)
	if err := d.Show(render.Frame{W: 10, H: 10}); err == nil {
		t.Fatal("expected error")
	}
}

func TestBusyTimeoutClosesBus(t *testing.T) {
	bus := &fakeBus{alwaysBusy: true}
	d := NewEPD7in5bV2(bus, noSleep)
	d.BusyTimeout = 50 * time.Millisecond
	f := render.Frame{W: 800, H: 480, Black: make([]byte, 48000), Red: make([]byte, 48000)}
	if err := d.Show(f); err == nil {
		t.Fatal("expected busy timeout")
	}
	if bus.closed == 0 {
		t.Fatal("bus must be closed after an error so the panel is unpowered")
	}
}

func TestClearCycles(t *testing.T) {
	bus := &fakeBus{}
	d := NewEPD7in5bV2(bus, noSleep)
	if err := d.Clear(2); err != nil {
		t.Fatal(err)
	}
	refreshes := strings.Count(strings.Join(bus.ops, " "), "C12")
	if refreshes != 6 {
		t.Errorf("expected 6 refreshes for 2 cycles, got %d", refreshes)
	}
}

func TestPNGDisplay(t *testing.T) {
	path := filepath.Join(t.TempDir(), "preview.png")
	d := NewPNG(path, 800, 480)
	c := render.NewCanvas(480, 800, render.FontDefault, 14)
	f, _ := render.Pack(c, 800, 480, 0)
	if err := d.Show(f); err != nil {
		t.Fatal(err)
	}
	fh, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer fh.Close()
	img, err := png.Decode(fh)
	if err != nil {
		t.Fatal(err)
	}
	if b := img.Bounds(); b.Dx() != 800 || b.Dy() != 480 {
		t.Errorf("png size %v", b)
	}
}
