package epd

import "time"

func init() {
	Models["epd_5_in_83_V2"] = [2]int{648, 480}
	// Upstream treats this black/white/red panel as black/white: display.py picks
	// colour by "colour" in the model name. See NewEPD5in83bV2.
	Models["epd5in83b_V2"] = [2]int{648, 480}
}

// initUC8179 is the init() shared by the UC8179 V2 drivers (5.83" V2, 5.83" b V2,
// 7.5" V2); booster, if set, is the 7.5" V2's leading booster soft start (0x06).
func initUC8179(e *panel, booster []byte, panelSetting, vcom byte) func() error {
	return func() error {
		if booster != nil {
			if err := e.cmd(0x06, booster...); err != nil {
				return err
			}
		}
		if err := e.cmds(
			op{0x01, []byte{0x07, 0x07, 0x3f, 0x3f}}, // power setting
			op{0x04, nil},                            // power on
		); err != nil {
			return err
		}
		e.sleep(100 * time.Millisecond)
		if err := e.waitBusy(); err != nil {
			return err
		}
		return e.cmds(
			op{0x00, []byte{panelSetting}},
			op{0x61, e.resolution()},
			op{0x15, []byte{0x00}},
			op{0x50, []byte{vcom, 0x07}}, // VCOM and data interval
			op{0x60, []byte{0x22}},       // TCON
		)
	}
}

// NewEPD5in83V2 drives the Waveshare 5.83" V2 black/white panel (Python: epd_5_in_83_V2).
func NewEPD5in83V2(bus Bus, sleep func(time.Duration)) Display {
	e := newPanel(bus, sleep, 648, 480)
	e.pollStep, e.resetLow, e.sleepWait = 20*time.Millisecond, 2*time.Millisecond, 2*time.Second
	e.initSeq = initUC8179(e, nil, 0x1F, 0x10)
	e.refresh = func(black, _ []byte) error {
		if err := e.cmds(
			op{0x10, make([]byte, len(black))},
			op{0x13, inverted(black)}, // this panel wants 1 = black
			op{0x12, nil},
		); err != nil {
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
	e := newPanel(bus, sleep, 648, 480)
	e.poll71, e.pollStep, e.resetLow, e.sleepWait = true, 200*time.Millisecond, 1*time.Millisecond, 2*time.Second
	e.initSeq = initUC8179(e, nil, 0x0F, 0x11)
	e.refresh = func(black, _ []byte) error {
		if err := e.cmds(
			op{0x10, black},
			op{0x13, make([]byte, len(black))}, // no red
			op{0x12, nil},
		); err != nil {
			return err
		}
		e.sleep(200 * time.Millisecond)
		return e.waitBusy()
	}
	return e
}
