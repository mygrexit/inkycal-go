package epd

import "time"

// The 7.5" V3/HD family, 880x528 (SSD1677-style controller): epd_7_in_5_v3 (b/w)
// and epd_7_in_5_v3_colour (b/w/red). Init is shared; the upstream differences
// are listed in epd75V3Variant.

func init() {
	Models["epd_7_in_5_v3"] = [2]int{880, 528}
	Models["epd_7_in_5_v3_colour"] = [2]int{880, 528}
}

// epd75V3Variant spells out every point where the two upstream drivers differ.
type epd75V3Variant struct {
	colour         bool
	resetLow       time.Duration // RST low pulse
	busyBeforeSWR  bool          // wait for BUSY before the software reset (b/w only)
	vbd            byte          // 0x3C border waveform
	initYCounter   []byte        // 0x4F at the end of init
	showYCounter   []byte        // 0x4F before the RAM write (colour sends a single byte upstream)
	updateCtl      byte          // 0x22 before 0x20 in display()
	postUpdateWait time.Duration // delay between 0x20 and the BUSY wait
}

var (
	epd75V3BW = epd75V3Variant{
		resetLow: 2 * time.Millisecond, busyBeforeSWR: true, vbd: 0x05,
		initYCounter: []byte{0x00, 0x00}, showYCounter: []byte{0x00, 0x00},
		updateCtl: 0xF7, postUpdateWait: 10 * time.Millisecond,
	}
	epd75V3Colour = epd75V3Variant{
		colour: true, resetLow: 4 * time.Millisecond, vbd: 0x01,
		initYCounter: []byte{0xAF, 0x02}, showYCounter: []byte{0xAF},
		updateCtl: 0xC7, postUpdateWait: 200 * time.Millisecond,
	}
)

// NewEPD7in5V3 creates the epd_7_in_5_v3 driver; sleep is time.Sleep in production.
func NewEPD7in5V3(bus Bus, sleep func(time.Duration)) Display {
	return newEPD7in5V3(bus, sleep, epd75V3BW)
}

// NewEPD7in5V3Colour creates the epd_7_in_5_v3_colour driver.
func NewEPD7in5V3Colour(bus Bus, sleep func(time.Duration)) Display {
	return newEPD7in5V3(bus, sleep, epd75V3Colour)
}

func newEPD7in5V3(bus Bus, sleep func(time.Duration), v epd75V3Variant) *panel {
	e := newPanel(bus, sleep, 880, 528)
	e.colour, e.resetLow = v.colour, v.resetLow
	// BUSY high = busy, then 200 ms (upstream polls without delay; 10 ms here).
	e.busyLevel, e.pollStep, e.busyTail = 1, 10*time.Millisecond, 200*time.Millisecond
	e.initSeq = func() error {
		if v.busyBeforeSWR {
			if err := e.waitBusy(); err != nil {
				return err
			}
		}
		for _, s := range []op{
			{0x12, nil},          // software reset
			{0x46, []byte{0xF7}}, // auto write red RAM
			{0x47, []byte{0xF7}}, // auto write b/w RAM
		} {
			if err := e.cmd(s.c, s.data...); err != nil {
				return err
			}
			if err := e.waitBusy(); err != nil {
				return err
			}
		}
		if err := e.cmds(
			op{0x0C, []byte{0xAE, 0xC7, 0xC3, 0xC0, 0x40}}, // soft start
			op{0x01, []byte{0xAF, 0x02, 0x01}},             // MUX 527
			op{0x11, []byte{0x01}},                         // data entry mode
			op{0x44, []byte{0x00, 0x00, 0x6F, 0x03}},       // RAM x 0..879
			op{0x45, []byte{0xAF, 0x02, 0x00, 0x00}},       // RAM y 527..0
			op{0x3C, []byte{v.vbd}},                        // border
			op{0x18, []byte{0x80}},                         // internal temperature sensor
			op{0x22, []byte{0xB1}},                         // load temperature and waveform
			op{0x20, nil},
		); err != nil {
			return err
		}
		if err := e.waitBusy(); err != nil {
			return err
		}
		return e.cmds(
			op{0x4E, []byte{0x00, 0x00}}, // RAM x counter
			op{0x4F, v.initYCounter},     // RAM y counter
		)
	}
	// refresh: 0x24 takes the black plane as is (1 = white); the colour driver sends
	// ~colourbuffer to 0x26, which is the red plane as is (1 = red).
	e.refresh = func(black, red []byte) error {
		if err := e.cmds(op{0x4F, v.showYCounter}, op{0x24, black}); err != nil {
			return err
		}
		if v.colour {
			if err := e.cmd(0x26, red...); err != nil {
				return err
			}
		}
		if err := e.cmds(op{0x22, []byte{v.updateCtl}}, op{0x20, nil}); err != nil {
			return err
		}
		e.sleep(v.postUpdateWait)
		return e.waitBusy()
	}
	// upstream sends deep sleep mode 1 and closes right away (no BUSY wait)
	e.sleepSeq = func() error { return e.cmd(0x10, 0x01) }
	return e
}
