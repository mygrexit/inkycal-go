package epd

import (
	"bytes"
	"fmt"
	"time"

	"inkycal/internal/render"
)

func init() {
	Models["epd_12_in_48"] = [2]int{w12, h12}
	Models["epd_12_in_48_colour"] = [2]int{w12, h12}
}

// EPD12in48 drives the first Waveshare 12.48" panels on the same four-controller HAT as
// the V2: black/white (Python: epd_12_in_48) and black/white/red V1 (epd_12_in_48_colour).
// It reuses the V2 driver's Bus12, quadrant split (part12) and LUT tables, but mirrors the
// upstream V1 drivers' own init, transfer and busy sequences.
type EPD12in48 struct {
	bus         Bus12
	sleep       func(time.Duration)
	colour      bool
	BusyTimeout time.Duration
}

// NewEPD12in48 creates the black/white driver; sleep is time.Sleep in production.
func NewEPD12in48(bus Bus12, sleep func(time.Duration)) *EPD12in48 {
	return &EPD12in48{bus: bus, sleep: sleep, BusyTimeout: 120 * time.Second}
}

// NewEPD12in48b creates the black/white/red V1 driver.
func NewEPD12in48b(bus Bus12, sleep func(time.Duration)) *EPD12in48 {
	e := NewEPD12in48(bus, sleep)
	e.colour = true
	return e
}

func (e *EPD12in48) Size() (int, int) { return w12, h12 }

func (e *EPD12in48) cmd(p Part, c byte, data ...byte) error {
	if err := e.bus.Command(p, c); err != nil {
		return err
	}
	if len(data) == 0 {
		return nil
	}
	return e.bus.Data(p, data...)
}

// waitBusy polls one part's status (BUSY low = busy). Upstream's colour driver sends
// M2 one extra status command before its loop; that is kept.
func (e *EPD12in48) waitBusy(p Part) error {
	const step = 10 * time.Millisecond
	if err := e.bus.Command(p, 0x71); err != nil {
		return err
	}
	level, err := e.bus.BusyLevel(p)
	if err != nil {
		return err
	}
	if e.colour && p == M2 {
		if err := e.bus.Command(p, 0x71); err != nil {
			return err
		}
	}
	for waited := time.Duration(0); level != 1; waited += step {
		if waited >= e.BusyTimeout {
			return fmt.Errorf("epd: part %v stays busy, check wiring and power", p)
		}
		e.sleep(step)
		if err := e.bus.Command(p, 0x71); err != nil {
			return err
		}
		if level, err = e.bus.BusyLevel(p); err != nil {
			return err
		}
	}
	e.sleep(200 * time.Millisecond)
	return nil
}

type step12 struct {
	p    Part
	c    byte
	data []byte
}

func (e *EPD12in48) send(seq []step12) error {
	for _, s := range seq {
		if err := e.cmd(s.p, s.c, s.data...); err != nil {
			return err
		}
	}
	return nil
}

var resolution12 = []step12{
	{M1, 0x61, []byte{0x02, 0x88, 0x01, 0xEC}}, // 648x492
	{S1, 0x61, []byte{0x02, 0x90, 0x01, 0xEC}}, // 656x492
	{M2, 0x61, []byte{0x02, 0x90, 0x01, 0xEC}},
	{S2, 0x61, []byte{0x02, 0x88, 0x01, 0xEC}},
}

var booster12 = []step12{
	{M1, 0x06, []byte{0x17, 0x17, 0x39, 0x17}},
	{M2, 0x06, []byte{0x17, 0x17, 0x39, 0x17}},
}

func (e *EPD12in48) init() error {
	if err := e.bus.Open(); err != nil {
		return err
	}
	for _, s := range []struct {
		high bool
		wait time.Duration
	}{{true, 200 * time.Millisecond}, {false, 10 * time.Millisecond}, {true, 200 * time.Millisecond}} {
		if err := e.bus.SetReset(s.high); err != nil {
			return err
		}
		e.sleep(s.wait)
	}
	if e.colour {
		return e.initColour()
	}
	return e.initBW()
}

func (e *EPD12in48) initColour() error {
	seq := []step12{
		{M1, 0x00, []byte{0x2F}}, // panel setting
		{S1, 0x00, []byte{0x2F}},
		{M2, 0x00, []byte{0x23}},
		{S2, 0x00, []byte{0x23}},
		{M1, 0x01, []byte{0x07, 0x17, 0x3F, 0x3F, 0x0D}}, // power setting
		{M2, 0x01, []byte{0x07, 0x17, 0x3F, 0x3F, 0x0D}},
	}
	seq = append(seq, booster12...)
	seq = append(seq, resolution12...)
	seq = append(seq,
		step12{All, 0x15, []byte{0x20}}, // DUSPI
		step12{All, 0x30, []byte{0x08}}, // PLL
		step12{All, 0x50, []byte{0x31, 0x07}},
		step12{All, 0x60, []byte{0x22}}, // TCON
		step12{M1, 0xE0, []byte{0x01}},  // power setting
		step12{M2, 0xE0, []byte{0x01}},
		step12{All, 0xE3, []byte{0x00}},
		step12{M1, 0x82, []byte{0x1C}}, // VCOM DC
		step12{M2, 0x82, []byte{0x1C}},
	)
	for _, l := range []struct {
		c   byte
		lut []byte
	}{{0x20, lutVcom12}, {0x21, lutWW12}, {0x22, lutBW12}, {0x23, lutWB12}, {0x24, lutBB12}, {0x25, lutWW12}} {
		seq = append(seq, step12{All, l.c, l.lut})
	}
	return e.send(seq)
}

func (e *EPD12in48) initBW() error {
	seq := []step12{
		{M1, 0x00, []byte{0x1F}}, // panel setting
		{S1, 0x00, []byte{0x1F}},
		{M2, 0x00, []byte{0x13}},
		{S2, 0x00, []byte{0x13}},
	}
	seq = append(seq, booster12...)
	seq = append(seq, resolution12...)
	seq = append(seq,
		step12{All, 0x15, []byte{0x20}},       // DUSPI
		step12{All, 0x50, []byte{0x21, 0x07}}, // border KW
		step12{All, 0x60, []byte{0x22}},       // TCON
		step12{All, 0xE3, []byte{0x00}},
	)
	if err := e.send(seq); err != nil {
		return err
	}
	// Upstream starts a temperature measurement but never reads it (the read is
	// commented out) and forces 25 °C instead.
	if err := e.cmd(M1, 0x40); err != nil {
		return err
	}
	if err := e.waitBusy(M1); err != nil {
		return err
	}
	e.sleep(310 * time.Millisecond)
	return e.send([]step12{{All, 0xE0, []byte{0x03}}, {All, 0xE5, []byte{25}}}) // cascade, force temperature
}

func (e *EPD12in48) refresh(black, red []byte) error {
	var seq []step12
	if e.colour {
		for _, q := range []struct {
			p         Part
			top, left bool
		}{{S2, true, true}, {M2, true, false}, {M1, false, true}, {S1, false, false}} {
			seq = append(seq, step12{q.p, 0x10, part12(black, q.top, q.left)}, step12{q.p, 0x13, part12(red, q.top, q.left)})
		}
	} else {
		for _, q := range []struct {
			p         Part
			top, left bool
		}{{M1, false, true}, {S1, false, false}, {M2, true, false}, {S2, true, true}} {
			seq = append(seq, step12{q.p, 0x13, part12(black, q.top, q.left)})
		}
	}
	if err := e.send(seq); err != nil {
		return err
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

func (e *EPD12in48) deepSleep() error {
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

func (e *EPD12in48) run(body func() error) error {
	if err := e.init(); err != nil {
		e.bus.Close()
		return err
	}
	if err := body(); err != nil {
		e.bus.Close()
		return err
	}
	if err := e.deepSleep(); err != nil {
		e.bus.Close()
		return err
	}
	return nil
}

// Show transfers the frame and refreshes the panel; the black/white panel shows red as black.
func (e *EPD12in48) Show(f render.Frame) error {
	if f.W != w12 || f.H != h12 || len(f.Black) != stride12*h12 || len(f.Red) != stride12*h12 {
		return fmt.Errorf("epd: frame %dx%d, panel is %dx%d", f.W, f.H, w12, h12)
	}
	if !e.colour {
		return e.run(func() error { return e.refresh(mergedBlack(f), nil) })
	}
	return e.run(func() error { return e.refresh(f.Black, f.Red) })
}

// Clear cycles black → white (black/white) or black → red → white (colour).
func (e *EPD12in48) Clear(cycles int) error {
	fill := func(b byte) []byte { return bytes.Repeat([]byte{b}, stride12*h12) }
	allBlack, white, noRed, allRed := fill(0x00), fill(0xFF), fill(0x00), fill(0xFF)
	steps := [][2][]byte{{allBlack, noRed}, {white, noRed}}
	if e.colour {
		steps = [][2][]byte{{allBlack, noRed}, {white, allRed}, {white, noRed}}
	}
	return e.run(func() error {
		for i := 0; i < cycles; i++ {
			for _, s := range steps {
				if err := e.refresh(s[0], s[1]); err != nil {
					return err
				}
			}
		}
		return nil
	})
}
