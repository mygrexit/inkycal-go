package epd

import "time"

func init() {
	Models["epd_5_in_83"] = [2]int{600, 448}
	Models["epd_5_in_83_colour"] = [2]int{600, 448}
}

// pack5in83 converts the planes to the 5.83" (V1) 4 bit/px format, two pixels per
// byte, high nibble first: 0x3 white, 0x0 black, 0x4 red (red wins over black).
func pack5in83(black, red []byte) []byte {
	out := make([]byte, 0, len(black)*4)
	for i, b := range black {
		r := red[i]
		var byt byte
		for bit := 7; bit >= 0; bit-- {
			px := byte(0x3)
			switch {
			case r>>bit&1 == 1:
				px = 0x4
			case b>>bit&1 == 0:
				px = 0x0
			}
			if bit%2 == 1 {
				byt = px << 4
			} else {
				out = append(out, byt|px)
			}
		}
	}
	return out
}

// NewEPD5in83 drives the Waveshare 5.83" (V1) black/white panel (Python: epd_5_in_83).
func NewEPD5in83(bus Bus, sleep func(time.Duration)) Display {
	e := &ucPanel{bus: bus, sleep: sleep, BusyTimeout: 60 * time.Second, w: 600, h: 448,
		pollStep: 100 * time.Millisecond, resetLow: 10 * time.Millisecond}
	e.initSeq = func() error {
		if err := e.cmd(0x01, 0x37, 0x00); err != nil { // power setting
			return err
		}
		if err := e.cmd(0x00, 0xCF, 0x08); err != nil { // panel setting
			return err
		}
		if err := e.cmd(0x06, 0xc7, 0xcc, 0x28); err != nil { // booster soft start
			return err
		}
		if err := e.cmd(0x04); err != nil { // power on
			return err
		}
		if err := e.waitBusy(); err != nil {
			return err
		}
		for _, s := range []struct {
			c    byte
			data []byte
		}{
			{0x30, []byte{0x3c}},                   // PLL
			{0x41, []byte{0x00}},                   // temperature calibration
			{0x50, []byte{0x77}},                   // VCOM and data interval
			{0x60, []byte{0x22}},                   // TCON
			{0x61, []byte{0x02, 0x58, 0x01, 0xC0}}, // resolution 600x448
			{0x82, []byte{0x1E}},                   // VCOM DC
			{0xe5, []byte{0x03}},                   // flash mode
		} {
			if err := e.cmd(s.c, s.data...); err != nil {
				return err
			}
		}
		return nil
	}
	e.refresh = func(black, _ []byte) error {
		if err := e.cmd(0x10, pack5in83(black, make([]byte, len(black)))...); err != nil {
			return err
		}
		if err := e.cmd(0x12); err != nil {
			return err
		}
		e.sleep(100 * time.Millisecond)
		return e.waitBusy()
	}
	return e
}

// NewEPD5in83Colour drives the Waveshare 5.83" (V1) black/white/red panel (Python: epd_5_in_83_colour).
func NewEPD5in83Colour(bus Bus, sleep func(time.Duration)) Display {
	e := &ucPanel{bus: bus, sleep: sleep, BusyTimeout: 60 * time.Second, w: 600, h: 448, colour: true,
		pollStep: 100 * time.Millisecond, resetLow: 10 * time.Millisecond}
	e.initSeq = func() error {
		for _, s := range []struct {
			c    byte
			data []byte
		}{
			{0x01, []byte{0x37, 0x00}},             // power setting
			{0x00, []byte{0xCF, 0x08}},             // panel setting
			{0x30, []byte{0x3A}},                   // PLL
			{0x82, []byte{0x28}},                   // VCOM DC
			{0x06, []byte{0xc7, 0xcc, 0x15}},       // booster soft start
			{0x50, []byte{0x77}},                   // VCOM and data interval
			{0x60, []byte{0x22}},                   // TCON
			{0x65, []byte{0x00}},                   // flash control
			{0x61, []byte{0x02, 0x58, 0x01, 0xc0}}, // resolution 600x448
			{0xe5, []byte{0x03, 0x03}},             // flash mode
		} {
			if err := e.cmd(s.c, s.data...); err != nil {
				return err
			}
		}
		return nil
	}
	e.refresh = func(black, red []byte) error {
		if err := e.cmd(0x10, pack5in83(black, red)...); err != nil {
			return err
		}
		if err := e.cmd(0x04); err != nil { // power on (upstream does it per refresh)
			return err
		}
		if err := e.waitBusy(); err != nil {
			return err
		}
		if err := e.cmd(0x12); err != nil {
			return err
		}
		e.sleep(100 * time.Millisecond)
		return e.waitBusy()
	}
	return e
}
