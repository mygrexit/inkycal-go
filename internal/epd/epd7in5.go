package epd

import "time"

// This file holds the UC8159 family with 4 bit/px panel RAM: the 7.5" V1
// (upstream epd_7_in_5 and epd_7_in_5_colour, 640x384) and the 5.83" V1
// (epd5in83.go), whose upstream drivers differ only in size and one flash-mode byte.

func init() {
	Models["epd_7_in_5"] = [2]int{640, 384}
	Models["epd_7_in_5_colour"] = [2]int{640, 384}
}

// NewEPD7in5 creates the epd_7_in_5 driver; sleep is time.Sleep in production.
func NewEPD7in5(bus Bus, sleep func(time.Duration)) Display {
	return newUC8159(bus, sleep, 640, 384, false, nil)
}

// NewEPD7in5Colour creates the epd_7_in_5_colour driver.
func NewEPD7in5Colour(bus Bus, sleep func(time.Duration)) Display {
	return newUC8159(bus, sleep, 640, 384, true, []byte{0x03})
}

// newUC8159 builds a 4 bit/px UC8159 driver. colourFlash is the colour driver's
// flash mode (0xE5) argument; the b/w drivers all send 0x03.
func newUC8159(bus Bus, sleep func(time.Duration), w, h int, colour bool, colourFlash []byte) *panel {
	e := newPanel(bus, sleep, w, h)
	e.colour, e.pollStep, e.resetLow = colour, 100*time.Millisecond, 10*time.Millisecond
	e.initSeq = func() error {
		if !colour {
			if err := e.cmds(
				op{0x01, []byte{0x37, 0x00}},       // power setting
				op{0x00, []byte{0xCF, 0x08}},       // panel setting
				op{0x06, []byte{0xC7, 0xCC, 0x28}}, // booster soft start
				op{0x04, nil},                      // power on
			); err != nil {
				return err
			}
			if err := e.waitBusy(); err != nil {
				return err
			}
			return e.cmds(
				op{0x30, []byte{0x3C}},   // PLL
				op{0x41, []byte{0x00}},   // temperature calibration
				op{0x50, []byte{0x77}},   // VCOM and data interval
				op{0x60, []byte{0x22}},   // TCON
				op{0x61, e.resolution()}, // resolution
				op{0x82, []byte{0x1E}},   // VCM DC
				op{0xE5, []byte{0x03}},   // flash mode
			)
		}
		// colour: no power-on here, display() does it.
		return e.cmds(
			op{0x01, []byte{0x37, 0x00}},       // power setting
			op{0x00, []byte{0xCF, 0x08}},       // panel setting
			op{0x30, []byte{0x3A}},             // PLL
			op{0x82, []byte{0x28}},             // VCM DC
			op{0x06, []byte{0xC7, 0xCC, 0x15}}, // booster soft start
			op{0x50, []byte{0x77}},             // VCOM and data interval
			op{0x60, []byte{0x22}},             // TCON
			op{0x65, []byte{0x00}},             // flash control
			op{0x61, e.resolution()},           // resolution
			op{0xE5, colourFlash},              // flash mode
		)
	}
	e.refresh = func(black, red []byte) error {
		if !colour {
			red = nil // the b/w driver's getbuffer() knows no red
		}
		if err := e.cmd(0x10, pixels4bpp(black, red)...); err != nil {
			return err
		}
		if colour {
			if err := e.cmd(0x04); err != nil { // power on (upstream does it per refresh)
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
	return e
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
