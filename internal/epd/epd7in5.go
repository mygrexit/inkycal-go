package epd

import (
	"errors"
	"fmt"
	"time"

	"inkycal/internal/render"
)

// This file holds the 7.5" V1 family (upstream epd_7_in_5 and epd_7_in_5_colour,
// 640x384, 4 bit/px panel RAM) and the helpers shared by the 7.5" drivers in
// epd7in5v2.go and epd7in5v3.go. Every sequence mirrors the upstream Python
// driver; testdata/trace/<model>.txt pins it byte for byte.

func init() {
	Models["epd_7_in_5"] = [2]int{640, 384}
	Models["epd_7_in_5_colour"] = [2]int{640, 384}
}

// epd75Panel is the state and plumbing every 7.5" driver here shares.
type epd75Panel struct {
	bus         Bus
	sleep       func(time.Duration)
	BusyTimeout time.Duration // give up waiting for the panel after this long
	w, h        int
}

func newEPD75Panel(bus Bus, sleep func(time.Duration), w, h int) epd75Panel {
	return epd75Panel{bus: bus, sleep: sleep, BusyTimeout: 60 * time.Second, w: w, h: h}
}

func (p *epd75Panel) Size() (int, int) { return p.w, p.h }

func (p *epd75Panel) planeSize() int { return p.w / 8 * p.h }

func (p *epd75Panel) checkFrame(f render.Frame) error {
	if f.W != p.w || f.H != p.h || len(f.Black) != p.planeSize() || len(f.Red) != p.planeSize() {
		return fmt.Errorf("epd: frame %dx%d, panel is %dx%d", f.W, f.H, p.w, p.h)
	}
	return nil
}

func (p *epd75Panel) cmd(c byte, data ...byte) error {
	if err := p.bus.Command(c); err != nil {
		return err
	}
	if len(data) == 0 {
		return nil
	}
	return p.bus.Data(data...)
}

type epd75Cmd struct {
	c    byte
	data []byte
}

func (p *epd75Panel) cmds(seq ...epd75Cmd) error {
	for _, s := range seq {
		if err := p.cmd(s.c, s.data...); err != nil {
			return err
		}
	}
	return nil
}

// openReset powers the panel and pulses RST: high, low, high (upstream reset()).
func (p *epd75Panel) openReset(high, low time.Duration) error {
	if err := p.bus.Open(); err != nil {
		return err
	}
	for _, step := range []struct {
		high bool
		wait time.Duration
	}{{true, high}, {false, low}, {true, high}} {
		if err := p.bus.SetReset(step.high); err != nil {
			return err
		}
		p.sleep(step.wait)
	}
	return nil
}

// waitWhile polls BUSY until it leaves busyLevel, sleeping step between reads, then
// sleeps tail. query, if set, is sent before every read (UC8179's 0x71 status read).
// One read per loop pass, like the upstream while loops, so the trace stub's
// alternating BUSY sees the same reads.
func (p *epd75Panel) waitWhile(busyLevel int, step, tail time.Duration, query func() error) error {
	for waited := time.Duration(0); ; waited += step {
		if query != nil {
			if err := query(); err != nil {
				return err
			}
		}
		level, err := p.bus.BusyLevel()
		if err != nil {
			return err
		}
		if level != busyLevel {
			break
		}
		if waited >= p.BusyTimeout {
			return errors.New("epd: panel stays busy, check wiring and power")
		}
		p.sleep(step)
	}
	p.sleep(tail)
	return nil
}

// session runs init, body, deep sleep; the bus is closed (panel unpowered) on any error.
func (p *epd75Panel) session(init, body, deepSleep func() error) error {
	for _, step := range []func() error{init, body, deepSleep} {
		if err := step(); err != nil {
			p.bus.Close()
			return err
		}
	}
	return nil
}

func epd75Fill(n int, b byte) []byte {
	p := make([]byte, n)
	for i := range p {
		p[i] = b
	}
	return p
}

// clear ports display.py calibrate(): per cycle black → white on b/w panels,
// black → colour → white on colour panels. Planes are in render.Frame terms.
func (p *epd75Panel) clear(cycles int, colour bool, init, deepSleep func() error, refresh func(black, red []byte) error) error {
	n := p.planeSize()
	allBlack, white, noRed, allRed := epd75Fill(n, 0x00), epd75Fill(n, 0xFF), epd75Fill(n, 0x00), epd75Fill(n, 0xFF)
	steps := [][2][]byte{{allBlack, noRed}, {white, noRed}}
	if colour {
		steps = [][2][]byte{{allBlack, noRed}, {white, allRed}, {white, noRed}}
	}
	return p.session(init, func() error {
		for i := 0; i < cycles; i++ {
			for _, s := range steps {
				if err := refresh(s[0], s[1]); err != nil {
					return err
				}
			}
		}
		return nil
	}, deepSleep)
}

// --- V1 family (UC8159-style controller, 4 bit/px RAM) ---

// EPD7in5V1 drives the original Waveshare 7.5" 640x384 panel: epd_7_in_5 (b/w)
// and epd_7_in_5_colour (b/w/red).
type EPD7in5V1 struct {
	epd75Panel
	colour bool
}

// NewEPD7in5 creates the epd_7_in_5 driver; sleep is time.Sleep in production.
func NewEPD7in5(bus Bus, sleep func(time.Duration)) *EPD7in5V1 {
	return &EPD7in5V1{epd75Panel: newEPD75Panel(bus, sleep, 640, 384)}
}

// NewEPD7in5Colour creates the epd_7_in_5_colour driver.
func NewEPD7in5Colour(bus Bus, sleep func(time.Duration)) *EPD7in5V1 {
	return &EPD7in5V1{epd75Panel: newEPD75Panel(bus, sleep, 640, 384), colour: true}
}

// waitBusy: BUSY low = busy, polled every 100 ms, no status command.
func (e *EPD7in5V1) waitBusy() error { return e.waitWhile(0, 100*time.Millisecond, 0, nil) }

func (e *EPD7in5V1) init() error {
	if err := e.openReset(200*time.Millisecond, 10*time.Millisecond); err != nil {
		return err
	}
	res := []byte{byte(e.w >> 8), byte(e.w), byte(e.h >> 8), byte(e.h)}
	if !e.colour { // epd_7_in_5
		if err := e.cmds(
			epd75Cmd{0x01, []byte{0x37, 0x00}},       // power setting
			epd75Cmd{0x00, []byte{0xCF, 0x08}},       // panel setting
			epd75Cmd{0x06, []byte{0xC7, 0xCC, 0x28}}, // booster soft start
			epd75Cmd{0x04, nil},                      // power on
		); err != nil {
			return err
		}
		if err := e.waitBusy(); err != nil {
			return err
		}
		return e.cmds(
			epd75Cmd{0x30, []byte{0x3C}}, // PLL
			epd75Cmd{0x41, []byte{0x00}}, // temperature calibration
			epd75Cmd{0x50, []byte{0x77}}, // VCOM and data interval
			epd75Cmd{0x60, []byte{0x22}}, // TCON
			epd75Cmd{0x61, res},          // resolution
			epd75Cmd{0x82, []byte{0x1E}}, // VCM DC
			epd75Cmd{0xE5, []byte{0x03}}, // flash mode
		)
	}
	// epd_7_in_5_colour: no power-on here, display() does it.
	return e.cmds(
		epd75Cmd{0x01, []byte{0x37, 0x00}},       // power setting
		epd75Cmd{0x00, []byte{0xCF, 0x08}},       // panel setting
		epd75Cmd{0x30, []byte{0x3A}},             // PLL
		epd75Cmd{0x82, []byte{0x28}},             // VCM DC
		epd75Cmd{0x06, []byte{0xC7, 0xCC, 0x15}}, // booster soft start
		epd75Cmd{0x50, []byte{0x77}},             // VCOM and data interval
		epd75Cmd{0x60, []byte{0x22}},             // TCON
		epd75Cmd{0x65, []byte{0x00}},             // flash control
		epd75Cmd{0x61, res},                      // resolution
		epd75Cmd{0xE5, []byte{0x03}},             // flash mode
	)
}

// pixels4bpp converts the planes to the panel's 4 bit/px RAM, two pixels per byte,
// first pixel in the high nibble: 0x0 black, 0x3 white, 0x4 red. red is nil on b/w.
func pixels4bpp(black, red []byte) []byte {
	out := make([]byte, len(black)*4)
	for i, b := range black {
		var r byte
		if red != nil {
			r = red[i]
		}
		for j := 0; j < 8; j++ {
			bit := byte(0x80 >> j)
			px := byte(0x03)
			switch {
			case r&bit != 0: // upstream checks red first
				px = 0x04
			case b&bit == 0:
				px = 0x00
			}
			if j%2 == 0 {
				out[i*4+j/2] = px << 4
			} else {
				out[i*4+j/2] |= px
			}
		}
	}
	return out
}

func (e *EPD7in5V1) refresh(black, red []byte) error {
	if !e.colour {
		red = nil // the b/w driver's getbuffer() knows no red
	}
	if err := e.cmd(0x10, pixels4bpp(black, red)...); err != nil {
		return err
	}
	if e.colour {
		if err := e.cmd(0x04); err != nil { // power on
			return err
		}
		if err := e.waitBusy(); err != nil {
			return err
		}
	}
	if err := e.cmd(0x12); err != nil { // refresh
		return err
	}
	e.sleep(100 * time.Millisecond)
	return e.waitBusy()
}

func (e *EPD7in5V1) deepSleep() error {
	if err := e.cmd(0x02); err != nil { // power off
		return err
	}
	if err := e.waitBusy(); err != nil {
		return err
	}
	if err := e.cmd(0x07, 0xA5); err != nil { // deep sleep
		return err
	}
	return e.bus.Close()
}

// Show transfers the frame and refreshes the panel.
func (e *EPD7in5V1) Show(f render.Frame) error {
	if err := e.checkFrame(f); err != nil {
		return err
	}
	black := f.Black
	if !e.colour {
		black = mergedBlack(f) // b/w panel: red shows as black (upstream main.py)
	}
	return e.session(e.init, func() error { return e.refresh(black, f.Red) }, e.deepSleep)
}

// Clear cycles black → white (b/w) or black → red → white (colour).
func (e *EPD7in5V1) Clear(cycles int) error {
	return e.clear(cycles, e.colour, e.init, e.deepSleep, e.refresh)
}
