package epd

import "time"

func init() {
	Models["epd_13_in_3"] = [2]int{w13in3, h13in3}
	Models["epd_13_in_3_colour"] = [2]int{w13in3, h13in3}
}

const (
	w13in3, h13in3 = 960, 680
	plane13in3     = w13in3 / 8 * h13in3
)

// NewEPD13in3 drives the Waveshare 13.3" black/white panel (K, Python: epd_13_in_3);
// sleep is time.Sleep in production. Both 13.3" panels use the standard HAT;
// upstream toggles CS_PIN around every byte, which its epdconfig ignores (the
// kernel drives CE0), so only DC and the SPI bytes matter.
func NewEPD13in3(bus Bus, sleep func(time.Duration)) Display {
	return newEPD13in3(bus, sleep, false)
}

// NewEPD13in3b drives the black/white/red panel (B, Python: epd_13_in_3_colour).
func NewEPD13in3b(bus Bus, sleep func(time.Duration)) Display {
	return newEPD13in3(bus, sleep, true)
}

func newEPD13in3(bus Bus, sleep func(time.Duration), colour bool) *panel {
	e := newPanel(bus, sleep, w13in3, h13in3)
	e.colour, e.resetHigh, e.resetLow, e.sleepWait = colour, 20*time.Millisecond, 2*time.Millisecond, 2*time.Second
	// upstream ReadBusy: BUSY high = busy, polled every 20 ms, then 20 ms
	e.busyLevel, e.pollStep, e.busyTail = 1, 20*time.Millisecond, 20*time.Millisecond
	border := byte(0x05)
	if colour {
		border = 0x01
	}
	e.initSeq = func() error {
		if err := e.waitBusy(); err != nil {
			return err
		}
		if err := e.cmd(0x12); err != nil { // SWRESET
			return err
		}
		if err := e.waitBusy(); err != nil {
			return err
		}
		if err := e.cmds(
			op{0x0C, []byte{0xAE, 0xC7, 0xC3, 0xC0, 0x80}}, // booster soft start
			op{0x01, []byte{0xA7, 0x02, 0x00}},             // driver output control: 680 gates
			op{0x11, []byte{0x03}},                         // data entry mode
			op{0x44, []byte{0x00, 0x00, 0xBF, 0x03}},       // RAM x range 0..959
			op{0x45, []byte{0x00, 0x00, 0xA7, 0x02}},       // RAM y range 0..679
			op{0x3C, []byte{border}},                       // border waveform
			op{0x18, []byte{0x80}},                         // internal temperature sensor
			op{0x4E, []byte{0x00, 0x00}},                   // RAM x counter
			op{0x4F, []byte{0x00, 0x00}},                   // RAM y counter
		); err != nil {
			return err
		}
		if colour {
			return e.waitBusy()
		}
		return nil
	}
	// refresh writes the planes (red is ignored on the black/white panel) and updates.
	e.refresh = func(black, red []byte) error {
		if err := e.cmd(0x24, black...); err != nil {
			return err
		}
		if colour {
			if err := e.cmd(0x26, red...); err != nil { // upstream inverts getbuffer: 1 = red
				return err
			}
		}
		if err := e.cmds(
			op{0x22, []byte{0xF7}}, // display update control
			op{0x20, nil},          // activate
		); err != nil {
			return err
		}
		return e.waitBusy()
	}
	e.sleepSeq = func() error { return e.cmd(0x10, 0x03) }
	return e
}
