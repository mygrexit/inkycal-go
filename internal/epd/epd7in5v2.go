package epd

import (
	"time"

	"inkycal/internal/render"
)

// epd_7_in_5_v2: Waveshare 7.5" V2 black/white, 800x480 (UC8179). Same controller
// as epd_7_in_5_v2_colour (epd7in5bv2.go) but a different upstream sequence:
// booster 0x06, panel setting 0x1F, VCOM 0x10, no 0x65, short reset, 20 ms tail
// after BUSY, and only the new-data RAM (0x13) is written.

func init() { Models["epd_7_in_5_v2"] = [2]int{800, 480} }

// EPD7in5V2 drives epd_7_in_5_v2.
type EPD7in5V2 struct{ epd75Panel }

// NewEPD7in5V2 creates the driver; sleep is time.Sleep in production.
func NewEPD7in5V2(bus Bus, sleep func(time.Duration)) *EPD7in5V2 {
	return &EPD7in5V2{newEPD75Panel(bus, sleep, 800, 480)}
}

// waitBusy sends the status query 0x71 before every BUSY read (BUSY low = busy).
func (e *EPD7in5V2) waitBusy() error {
	return e.waitWhile(0, 10*time.Millisecond, 20*time.Millisecond, func() error { return e.cmd(0x71) })
}

func (e *EPD7in5V2) init() error {
	if err := e.openReset(20*time.Millisecond, 2*time.Millisecond); err != nil {
		return err
	}
	if err := e.cmds(
		epd75Cmd{0x06, []byte{0x17, 0x17, 0x28, 0x17}}, // booster soft start
		epd75Cmd{0x01, []byte{0x07, 0x07, 0x3F, 0x3F}}, // power setting
		epd75Cmd{0x04, nil},                            // power on
	); err != nil {
		return err
	}
	e.sleep(100 * time.Millisecond)
	if err := e.waitBusy(); err != nil {
		return err
	}
	return e.cmds(
		epd75Cmd{0x00, []byte{0x1F}},                   // panel setting (b/w OTP)
		epd75Cmd{0x61, []byte{0x03, 0x20, 0x01, 0xE0}}, // resolution 800x480
		epd75Cmd{0x15, []byte{0x00}},
		epd75Cmd{0x50, []byte{0x10, 0x07}}, // VCOM and data interval
		epd75Cmd{0x60, []byte{0x22}},       // TCON
	)
}

// refresh writes the black plane inverted (panel: 1 = black), as upstream getbuffer() does.
func (e *EPD7in5V2) refresh(black, _ []byte) error {
	inv := make([]byte, len(black))
	for i, b := range black {
		inv[i] = ^b
	}
	if err := e.cmd(0x13, inv...); err != nil {
		return err
	}
	if err := e.cmd(0x12); err != nil {
		return err
	}
	e.sleep(100 * time.Millisecond)
	return e.waitBusy()
}

func (e *EPD7in5V2) deepSleep() error {
	if err := e.cmd(0x02); err != nil { // power off
		return err
	}
	if err := e.waitBusy(); err != nil {
		return err
	}
	if err := e.cmd(0x07, 0xA5); err != nil { // deep sleep
		return err
	}
	e.sleep(2 * time.Second)
	return e.bus.Close()
}

// Show transfers the frame and refreshes the panel.
func (e *EPD7in5V2) Show(f render.Frame) error {
	if err := e.checkFrame(f); err != nil {
		return err
	}
	return e.session(e.init, func() error { return e.refresh(mergedBlack(f), nil) }, e.deepSleep)
}

// Clear cycles black → white.
func (e *EPD7in5V2) Clear(cycles int) error {
	return e.clear(cycles, false, e.init, e.deepSleep, e.refresh)
}
