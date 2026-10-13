package epd

import "time"

// epd_7_in_5_v2: Waveshare 7.5" V2 black/white, 800x480 (UC8179). Same controller
// as epd_7_in_5_v2_colour (epd7in5bv2.go) but a different upstream sequence:
// booster 0x06, panel setting 0x1F, VCOM 0x10, no 0x65, short reset, 20 ms tail
// after BUSY, and only the new-data RAM (0x13) is written.

func init() { Models["epd_7_in_5_v2"] = [2]int{800, 480} }

// NewEPD7in5V2 creates the epd_7_in_5_v2 driver; sleep is time.Sleep in production.
func NewEPD7in5V2(bus Bus, sleep func(time.Duration)) Display {
	e := newPanel(bus, sleep, 800, 480)
	e.resetHigh, e.resetLow, e.sleepWait = 20*time.Millisecond, 2*time.Millisecond, 2*time.Second
	// status query 0x71 before every BUSY read (BUSY low = busy), 20 ms tail
	e.poll71, e.pollStep, e.busyTail = true, 10*time.Millisecond, 20*time.Millisecond
	e.initSeq = initUC8179(e, []byte{0x17, 0x17, 0x28, 0x17}, 0x1F, 0x10)
	// refresh writes the black plane inverted (panel: 1 = black), as upstream getbuffer() does.
	e.refresh = func(black, _ []byte) error {
		if err := e.cmds(op{0x13, inverted(black)}, op{0x12, nil}); err != nil {
			return err
		}
		e.sleep(100 * time.Millisecond)
		return e.waitBusy()
	}
	return e
}
