package epd

import "time"

func init() {
	Models["epd_5_in_83_V2"] = [2]int{648, 480}
	// Upstream treats this black/white/red panel as black/white: display.py picks
	// colour by "colour" in the model name. See NewEPD5in83bV2.
	Models["epd5in83b_V2"] = [2]int{648, 480}
}

func init5in83V2(e *ucPanel, panel byte, vcom byte) func() error {
	return func() error {
		if err := e.cmd(0x01, 0x07, 0x07, 0x3f, 0x3f); err != nil { // power setting
			return err
		}
		if err := e.cmd(0x04); err != nil { // power on
			return err
		}
		e.sleep(100 * time.Millisecond)
		if err := e.waitBusy(); err != nil {
			return err
		}
		for _, s := range []struct {
			c    byte
			data []byte
		}{
			{0x00, []byte{panel}},                  // panel setting
			{0x61, []byte{0x02, 0x88, 0x01, 0xe0}}, // resolution 648x480
			{0x15, []byte{0x00}},
			{0x50, []byte{vcom, 0x07}}, // VCOM and data interval
			{0x60, []byte{0x22}},       // TCON
		} {
			if err := e.cmd(s.c, s.data...); err != nil {
				return err
			}
		}
		return nil
	}
}

// NewEPD5in83V2 drives the Waveshare 5.83" V2 black/white panel (Python: epd_5_in_83_V2).
func NewEPD5in83V2(bus Bus, sleep func(time.Duration)) Display {
	e := &ucPanel{bus: bus, sleep: sleep, BusyTimeout: 60 * time.Second, w: 648, h: 480,
		pollStep: 20 * time.Millisecond, resetLow: 2 * time.Millisecond, sleepWait: 2 * time.Second}
	e.initSeq = init5in83V2(e, 0x1F, 0x10)
	e.refresh = func(black, _ []byte) error {
		if err := e.cmd(0x10, make([]byte, len(black))...); err != nil {
			return err
		}
		if err := e.cmd(0x13, ucInvert(black)...); err != nil { // this panel wants 1 = black
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

// NewEPD5in83bV2 drives the Waveshare 5.83" V2 black/white/red panel under upstream's
// model name epd5in83b_V2. Upstream's display.py treats it as black/white (no
// "colour" in the name) and then crashes, because the driver's display() needs a
// red buffer. Here it is a black/white panel as upstream intends: red content is
// merged into black and the red plane is sent empty.
func NewEPD5in83bV2(bus Bus, sleep func(time.Duration)) Display {
	e := &ucPanel{bus: bus, sleep: sleep, BusyTimeout: 60 * time.Second, w: 648, h: 480,
		poll71: true, pollStep: 200 * time.Millisecond, resetLow: 1 * time.Millisecond, sleepWait: 2 * time.Second}
	e.initSeq = init5in83V2(e, 0x0F, 0x11)
	e.refresh = func(black, _ []byte) error {
		if err := e.cmd(0x10, black...); err != nil {
			return err
		}
		if err := e.cmd(0x13, make([]byte, len(black))...); err != nil { // no red
			return err
		}
		if err := e.cmd(0x12); err != nil {
			return err
		}
		e.sleep(200 * time.Millisecond)
		return e.waitBusy()
	}
	return e
}
