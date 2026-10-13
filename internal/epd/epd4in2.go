package epd

import "time"

func init() {
	Models["epd_4_in_2"] = [2]int{400, 300}
	Models["epd_4_in_2_colour"] = [2]int{400, 300}
}

// 4.2" LUTs from upstream epd_4_in_2.py (lut_vcom0 is 44 bytes, the others 42).
var (
	lut4in2Vcom0 = []byte{
		0x00, 0x17, 0x00, 0x00, 0x00, 0x02,
		0x00, 0x17, 0x17, 0x00, 0x00, 0x02,
		0x00, 0x0A, 0x01, 0x00, 0x00, 0x01,
		0x00, 0x0E, 0x0E, 0x00, 0x00, 0x02,
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
	}
	lut4in2WW = []byte{ // also lut_bw: upstream's tables are identical
		0x40, 0x17, 0x00, 0x00, 0x00, 0x02,
		0x90, 0x17, 0x17, 0x00, 0x00, 0x02,
		0x40, 0x0A, 0x01, 0x00, 0x00, 0x01,
		0xA0, 0x0E, 0x0E, 0x00, 0x00, 0x02,
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
	}
	lut4in2BB = []byte{ // also lut_wb: upstream's tables are identical
		0x80, 0x17, 0x00, 0x00, 0x00, 0x02,
		0x90, 0x17, 0x17, 0x00, 0x00, 0x02,
		0x80, 0x0A, 0x01, 0x00, 0x00, 0x01,
		0x50, 0x0E, 0x0E, 0x00, 0x00, 0x02,
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
	}
)

// NewEPD4in2 drives the Waveshare 4.2" black/white panel (Python: epd_4_in_2).
func NewEPD4in2(bus Bus, sleep func(time.Duration)) Display {
	e := &ucPanel{bus: bus, sleep: sleep, BusyTimeout: 60 * time.Second, w: 400, h: 300,
		poll71: true, pollStep: 100 * time.Millisecond, resetLow: 10 * time.Millisecond}
	e.initSeq = func() error {
		if err := e.cmd(0x01, 0x03, 0x00, 0x2b, 0x2b); err != nil { // power setting
			return err
		}
		if err := e.cmd(0x06, 0x17, 0x17, 0x17); err != nil { // booster soft start
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
			{0x00, []byte{0xbf, 0x0d}},             // panel setting
			{0x30, []byte{0x3c}},                   // PLL
			{0x61, []byte{0x01, 0x90, 0x01, 0x2c}}, // resolution 400x300
			{0x82, []byte{0x28}},                   // VCOM DC
			{0x50, []byte{0x97}},                   // VCOM and data interval
			// set_lut(): upstream writes lut_bb to 0x23 and lut_wb to 0x24 (swapped
			// against its comments); both tables are identical, so it does not matter.
			{0x20, lut4in2Vcom0},
			{0x21, lut4in2WW},
			{0x22, lut4in2WW},
			{0x23, lut4in2BB},
			{0x24, lut4in2BB},
		} {
			if err := e.cmd(s.c, s.data...); err != nil {
				return err
			}
		}
		return nil
	}
	e.refresh = func(black, _ []byte) error {
		if err := e.cmd(0x10, ucFilled(len(black), 0xFF)...); err != nil { // old data: white
			return err
		}
		if err := e.cmd(0x13, black...); err != nil {
			return err
		}
		if err := e.cmd(0x12); err != nil {
			return err
		}
		return e.waitBusy()
	}
	return e
}

// NewEPD4in2Colour drives the Waveshare 4.2" black/white/red panel (Python: epd_4_in_2_colour).
func NewEPD4in2Colour(bus Bus, sleep func(time.Duration)) Display {
	e := &ucPanel{bus: bus, sleep: sleep, BusyTimeout: 60 * time.Second, w: 400, h: 300, colour: true,
		pollStep: 100 * time.Millisecond, resetLow: 5 * time.Millisecond}
	e.initSeq = func() error {
		if err := e.cmd(0x06, 0x17, 0x17, 0x17); err != nil { // booster soft start
			return err
		}
		if err := e.cmd(0x04); err != nil { // power on
			return err
		}
		if err := e.waitBusy(); err != nil {
			return err
		}
		return e.cmd(0x00, 0x0F) // panel setting: LUT from OTP
	}
	e.refresh = func(black, red []byte) error {
		if err := e.cmd(0x10, black...); err != nil {
			return err
		}
		if err := e.cmd(0x13, ucInvert(red)...); err != nil { // 0 = red
			return err
		}
		if err := e.cmd(0x12); err != nil {
			return err
		}
		return e.waitBusy()
	}
	return e
}
