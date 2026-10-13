package epd

import (
	"bytes"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"inkycal/internal/render"
)

// Part selects sub-panels of the 12.48" panel; each one has its own controller.
type Part uint8

const (
	M1  Part = 1 << iota // bottom left
	S1                   // bottom right
	M2                   // top right
	S2                   // top left
	All = M1 | S1 | M2 | S2
)

func (p Part) String() string {
	if p == All {
		return "ALL"
	}
	var names []string
	for _, n := range []struct {
		p    Part
		name string
	}{{M1, "M1"}, {S1, "S1"}, {M2, "M2"}, {S2, "S2"}} {
		if p&n.p != 0 {
			names = append(names, n.name)
		}
	}
	return strings.Join(names, "+")
}

// Bus12 is the hardware interface of the 12.48" panel: one SPI bus, a chip select per part,
// shared DC/RST lines per pair (M1S1, M2S2) and a BUSY pin per part.
type Bus12 interface {
	Open() error
	Close() error
	SetReset(high bool) error        // both RST lines
	Command(p Part, c byte) error    // DC low, write one byte to the selected parts
	Data(p Part, data ...byte) error // DC high, write bytes to the selected parts
	BusyLevel(p Part) (int, error)   // raw BUSY level of one part
	ReadData(p Part) (byte, error)   // DC high, read one byte from one part (3-wire SPI)
}

const (
	w12, h12    = 1304, 984
	stride12    = w12 / 8 // 163 bytes per row
	split12X    = 81      // bytes: left parts are 648 px wide, right parts 656 px
	split12Y    = 492     // rows: top parts, then bottom parts
	defaultTemp = 0x14    // 20 °C, used if the sensor can't be read
)

// EPD12in48bV2 drives the Waveshare 12.48" black/white/red panel, V2 (Python: epd_12_in_48_colour_V2).
type EPD12in48bV2 struct {
	bus         Bus12
	sleep       func(time.Duration)
	BusyTimeout time.Duration
}

// NewEPD12in48bV2 creates the driver; sleep is time.Sleep in production and a fake in tests.
func NewEPD12in48bV2(bus Bus12, sleep func(time.Duration)) *EPD12in48bV2 {
	return &EPD12in48bV2{bus: bus, sleep: sleep, BusyTimeout: 120 * time.Second}
}

func (e *EPD12in48bV2) Size() (int, int) { return w12, h12 }

func (e *EPD12in48bV2) cmd(p Part, c byte, data ...byte) error {
	if err := e.bus.Command(p, c); err != nil {
		return err
	}
	if len(data) == 0 {
		return nil
	}
	return e.bus.Data(p, data...)
}

// waitBusy polls one part's status (BUSY low = busy) every 10 ms.
func (e *EPD12in48bV2) waitBusy(p Part) error {
	const step = 10 * time.Millisecond
	for waited := time.Duration(0); ; waited += step {
		if err := e.bus.Command(p, 0x71); err != nil {
			return err
		}
		level, err := e.bus.BusyLevel(p)
		if err != nil {
			return err
		}
		if level == 1 {
			break
		}
		if waited >= e.BusyTimeout {
			return fmt.Errorf("epd: part %v stays busy, check wiring and power", p)
		}
		e.sleep(step)
	}
	e.sleep(200 * time.Millisecond)
	return nil
}

var (
	lutVcom12 = []byte{
		0x00, 0x10, 0x10, 0x01, 0x08, 0x01, 0x00, 0x06, 0x01, 0x06, 0x01, 0x05,
		0x00, 0x08, 0x01, 0x08, 0x01, 0x06, 0x00, 0x06, 0x01, 0x06, 0x01, 0x05,
		0x00, 0x05, 0x01, 0x1E, 0x0F, 0x06, 0x00, 0x05, 0x01, 0x1E, 0x0F, 0x01,
		0x00, 0x04, 0x05, 0x08, 0x08, 0x01, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0,
	}
	lutWW12 = []byte{
		0x91, 0x10, 0x10, 0x01, 0x08, 0x01, 0x04, 0x06, 0x01, 0x06, 0x01, 0x05,
		0x84, 0x08, 0x01, 0x08, 0x01, 0x06, 0x80, 0x06, 0x01, 0x06, 0x01, 0x05,
		0x00, 0x05, 0x01, 0x1E, 0x0F, 0x06, 0x00, 0x05, 0x01, 0x1E, 0x0F, 0x01,
		0x08, 0x04, 0x05, 0x08, 0x08, 0x01, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0,
	}
	lutBW12 = []byte{
		0xA8, 0x10, 0x10, 0x01, 0x08, 0x01, 0x84, 0x06, 0x01, 0x06, 0x01, 0x05,
		0x84, 0x08, 0x01, 0x08, 0x01, 0x06, 0x86, 0x06, 0x01, 0x06, 0x01, 0x05,
		0x8C, 0x05, 0x01, 0x1E, 0x0F, 0x06, 0x8C, 0x05, 0x01, 0x1E, 0x0F, 0x01,
		0xF0, 0x04, 0x05, 0x08, 0x08, 0x01, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0,
	}
	lutWB12 = lutWW12 // identical tables in the Waveshare driver
	lutBB12 = []byte{
		0x92, 0x10, 0x10, 0x01, 0x08, 0x01, 0x80, 0x06, 0x01, 0x06, 0x01, 0x05,
		0x84, 0x08, 0x01, 0x08, 0x01, 0x06, 0x04, 0x06, 0x01, 0x06, 0x01, 0x05,
		0x00, 0x05, 0x01, 0x1E, 0x0F, 0x06, 0x00, 0x05, 0x01, 0x1E, 0x0F, 0x01,
		0x01, 0x04, 0x05, 0x08, 0x08, 0x01, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0,
	}
)

func (e *EPD12in48bV2) init() error {
	if err := e.bus.Open(); err != nil {
		return err
	}
	for _, step := range []struct {
		high bool
		wait time.Duration
	}{{true, 200 * time.Millisecond}, {false, 10 * time.Millisecond}, {true, 200 * time.Millisecond}} {
		if err := e.bus.SetReset(step.high); err != nil {
			return err
		}
		e.sleep(step.wait)
	}
	seq := []struct {
		p    Part
		c    byte
		data []byte
	}{
		{M1, 0x00, []byte{0x0F}}, // panel setting
		{S1, 0x00, []byte{0x0F}},
		{M2, 0x00, []byte{0x03}},
		{S2, 0x00, []byte{0x03}},
		{M1, 0x06, []byte{0x17, 0x17, 0x39, 0x17}}, // booster soft start
		{M2, 0x06, []byte{0x17, 0x17, 0x39, 0x17}},
		{M1, 0x61, []byte{0x02, 0x88, 0x01, 0xEC}}, // resolution 648x492
		{S1, 0x61, []byte{0x02, 0x90, 0x01, 0xEC}}, // 656x492
		{M2, 0x61, []byte{0x02, 0x90, 0x01, 0xEC}},
		{S2, 0x61, []byte{0x02, 0x88, 0x01, 0xEC}},
		{All, 0x15, []byte{0x20}},       // DUSPI
		{All, 0x50, []byte{0x11, 0x07}}, // VCOM and data interval
		{All, 0x60, []byte{0x22}},       // TCON
		{All, 0xE3, []byte{0x00}},
	}
	for _, s := range seq {
		if err := e.cmd(s.p, s.c, s.data...); err != nil {
			return err
		}
	}
	if err := e.setTemperature(); err != nil {
		return err
	}
	for _, l := range []struct {
		c   byte
		lut []byte
	}{{0x20, lutVcom12}, {0x21, lutWW12}, {0x22, lutBW12}, {0x23, lutWB12}, {0x24, lutBB12}, {0x25, lutWW12}} {
		if err := e.cmd(All, l.c, l.lut...); err != nil {
			return err
		}
	}
	return nil
}

// setTemperature reads M1's sensor and hands the value to all four controllers.
func (e *EPD12in48bV2) setTemperature() error {
	if err := e.cmd(M1, 0x40); err != nil {
		return err
	}
	if err := e.waitBusy(M1); err != nil {
		return err
	}
	e.sleep(300 * time.Millisecond)
	temp, err := e.bus.ReadData(M1)
	if err != nil || temp == 0 || temp > 50 { // °C; anything else is a misread
		slog.Warn("epd: temperature implausible, assuming 20 °C", "raw", temp, "err", err)
		temp = defaultTemp
	} else {
		slog.Debug("epd: temperature", "celsius", int8(temp))
	}
	if err := e.cmd(All, 0xE0, 0x03); err != nil {
		return err
	}
	return e.cmd(All, 0xE5, temp)
}

// part copies one quadrant out of a full-panel bit plane.
func part12(plane []byte, top, left bool) []byte {
	y0, x0, x1 := 0, 0, split12X
	if !top {
		y0 = split12Y
	}
	if !left {
		x0, x1 = split12X, stride12
	}
	out := make([]byte, 0, (x1-x0)*split12Y)
	for y := y0; y < y0+split12Y; y++ {
		out = append(out, plane[y*stride12+x0:y*stride12+x1]...)
	}
	return out
}

func (e *EPD12in48bV2) refresh(black, red []byte) error {
	for _, q := range []struct {
		p         Part
		top, left bool
	}{{S2, true, true}, {M2, true, false}, {M1, false, true}, {S1, false, false}} {
		if err := e.cmd(q.p, 0x10, part12(black, q.top, q.left)...); err != nil {
			return err
		}
		if err := e.cmd(q.p, 0x13, part12(red, q.top, q.left)...); err != nil {
			return err
		}
	}
	if err := e.cmd(M1|M2, 0x04); err != nil { // power on
		return err
	}
	e.sleep(300 * time.Millisecond)
	if err := e.cmd(All, 0x12); err != nil { // refresh
		return err
	}
	for _, p := range []Part{M1, S1, M2, S2} {
		if err := e.waitBusy(p); err != nil {
			return err
		}
	}
	return nil
}

func (e *EPD12in48bV2) deepSleep() error {
	if err := e.cmd(All, 0x02); err != nil { // power off
		return err
	}
	e.sleep(300 * time.Millisecond)
	if err := e.cmd(All, 0x07, 0xA5); err != nil { // deep sleep
		return err
	}
	e.sleep(300 * time.Millisecond)
	return e.bus.Close()
}

func (e *EPD12in48bV2) run(body func() error) error {
	if err := e.init(); err != nil {
		e.bus.Close()
		return err
	}
	if err := body(); err != nil {
		e.bus.Close()
		return err
	}
	if err := e.deepSleep(); err != nil {
		e.bus.Close() // never leave the panel powered or the SPI device open
		return err
	}
	return nil
}

// Show transfers the frame and refreshes the panel.
func (e *EPD12in48bV2) Show(f render.Frame) error {
	if f.W != w12 || f.H != h12 || len(f.Black) != stride12*h12 || len(f.Red) != stride12*h12 {
		return fmt.Errorf("epd: frame %dx%d, panel is %dx%d", f.W, f.H, w12, h12)
	}
	return e.run(func() error { return e.refresh(f.Black, f.Red) })
}

// Clear cycles black → red → white to remove ghosting.
func (e *EPD12in48bV2) Clear(cycles int) error {
	fill := func(b byte) []byte { return bytes.Repeat([]byte{b}, stride12*h12) }
	allBlack, white, noRed, allRed := fill(0x00), fill(0xFF), fill(0x00), fill(0xFF)
	return e.run(func() error {
		for i := 0; i < cycles; i++ {
			for _, step := range [][2][]byte{{allBlack, noRed}, {white, allRed}, {white, noRed}} {
				if err := e.refresh(step[0], step[1]); err != nil {
					return err
				}
			}
		}
		return nil
	})
}
