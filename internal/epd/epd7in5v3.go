package epd

import (
	"time"

	"inkycal/internal/render"
)

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

// EPD7in5V3 drives epd_7_in_5_v3 and epd_7_in_5_v3_colour.
type EPD7in5V3 struct {
	epd75Panel
	v epd75V3Variant
}

// NewEPD7in5V3 creates the epd_7_in_5_v3 driver; sleep is time.Sleep in production.
func NewEPD7in5V3(bus Bus, sleep func(time.Duration)) *EPD7in5V3 {
	return &EPD7in5V3{newEPD75Panel(bus, sleep, 880, 528), epd75V3BW}
}

// NewEPD7in5V3Colour creates the epd_7_in_5_v3_colour driver.
func NewEPD7in5V3Colour(bus Bus, sleep func(time.Duration)) *EPD7in5V3 {
	return &EPD7in5V3{newEPD75Panel(bus, sleep, 880, 528), epd75V3Colour}
}

// waitBusy: BUSY high = busy, then 200 ms (upstream polls without delay; 10 ms here).
func (e *EPD7in5V3) waitBusy() error {
	return e.waitWhile(1, 10*time.Millisecond, 200*time.Millisecond, nil)
}

func (e *EPD7in5V3) init() error {
	if err := e.openReset(200*time.Millisecond, e.v.resetLow); err != nil {
		return err
	}
	if e.v.busyBeforeSWR {
		if err := e.waitBusy(); err != nil {
			return err
		}
	}
	for _, s := range []epd75Cmd{
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
		epd75Cmd{0x0C, []byte{0xAE, 0xC7, 0xC3, 0xC0, 0x40}}, // soft start
		epd75Cmd{0x01, []byte{0xAF, 0x02, 0x01}},             // MUX 527
		epd75Cmd{0x11, []byte{0x01}},                         // data entry mode
		epd75Cmd{0x44, []byte{0x00, 0x00, 0x6F, 0x03}},       // RAM x 0..879
		epd75Cmd{0x45, []byte{0xAF, 0x02, 0x00, 0x00}},       // RAM y 527..0
		epd75Cmd{0x3C, []byte{e.v.vbd}},                      // border
		epd75Cmd{0x18, []byte{0x80}},                         // internal temperature sensor
		epd75Cmd{0x22, []byte{0xB1}},                         // load temperature and waveform
		epd75Cmd{0x20, nil},
	); err != nil {
		return err
	}
	if err := e.waitBusy(); err != nil {
		return err
	}
	return e.cmds(
		epd75Cmd{0x4E, []byte{0x00, 0x00}}, // RAM x counter
		epd75Cmd{0x4F, e.v.initYCounter},   // RAM y counter
	)
}

// refresh: 0x24 takes the black plane as is (1 = white); the colour driver sends
// ~colourbuffer to 0x26, which is the red plane as is (1 = red).
func (e *EPD7in5V3) refresh(black, red []byte) error {
	if err := e.cmd(0x4F, e.v.showYCounter...); err != nil {
		return err
	}
	if err := e.cmd(0x24, black...); err != nil {
		return err
	}
	if e.v.colour {
		if err := e.cmd(0x26, red...); err != nil {
			return err
		}
	}
	if err := e.cmds(epd75Cmd{0x22, []byte{e.v.updateCtl}}, epd75Cmd{0x20, nil}); err != nil {
		return err
	}
	e.sleep(e.v.postUpdateWait)
	return e.waitBusy()
}

// deepSleep: upstream sends deep sleep mode 1 and closes right away (no BUSY wait).
func (e *EPD7in5V3) deepSleep() error {
	if err := e.cmd(0x10, 0x01); err != nil {
		return err
	}
	return e.bus.Close()
}

// Show transfers the frame and refreshes the panel.
func (e *EPD7in5V3) Show(f render.Frame) error {
	if err := e.checkFrame(f); err != nil {
		return err
	}
	black := f.Black
	if !e.v.colour {
		black = mergedBlack(f) // b/w panel: red shows as black (upstream main.py)
	}
	return e.session(e.init, func() error { return e.refresh(black, f.Red) }, e.deepSleep)
}

// Clear cycles black → white (b/w) or black → red → white (colour).
func (e *EPD7in5V3) Clear(cycles int) error {
	return e.clear(cycles, e.v.colour, e.init, e.deepSleep, e.refresh)
}
