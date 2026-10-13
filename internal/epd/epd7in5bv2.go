package epd

import (
	"errors"
	"fmt"
	"time"

	"inkycal/internal/render"
)

// EPD7in5bV2 drives the Waveshare 7.5" V2 black/white/red panel (Python: epd_7_in_5_v2_colour).
type EPD7in5bV2 struct {
	bus         Bus
	sleep       func(time.Duration)
	BusyTimeout time.Duration // give up waiting for the panel after this long
}

// NewEPD7in5bV2 creates the driver; sleep is time.Sleep in production and a fake in tests.
func NewEPD7in5bV2(bus Bus, sleep func(time.Duration)) *EPD7in5bV2 {
	return &EPD7in5bV2{bus: bus, sleep: sleep, BusyTimeout: 60 * time.Second}
}

func (e *EPD7in5bV2) Size() (int, int) { return 800, 480 }

// waitBusy polls the panel status (BUSY low = busy) every 10 ms.
func (e *EPD7in5bV2) waitBusy() error {
	const step = 10 * time.Millisecond
	for waited := time.Duration(0); ; waited += step {
		if err := e.bus.Command(0x71); err != nil {
			return err
		}
		level, err := e.bus.BusyLevel()
		if err != nil {
			return err
		}
		if level == 1 {
			break
		}
		if waited >= e.BusyTimeout {
			return errors.New("epd: panel stays busy, check wiring and power")
		}
		e.sleep(step)
	}
	e.sleep(200 * time.Millisecond)
	return nil
}

func (e *EPD7in5bV2) cmd(c byte, data ...byte) error {
	if err := e.bus.Command(c); err != nil {
		return err
	}
	if len(data) == 0 {
		return nil
	}
	return e.bus.Data(data...)
}

func (e *EPD7in5bV2) init() error {
	if err := e.bus.Open(); err != nil {
		return err
	}
	for _, step := range []struct {
		high bool
		wait time.Duration
	}{{true, 200 * time.Millisecond}, {false, 4 * time.Millisecond}, {true, 200 * time.Millisecond}} {
		if err := e.bus.SetReset(step.high); err != nil {
			return err
		}
		e.sleep(step.wait)
	}
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
	seq := []struct {
		c    byte
		data []byte
	}{
		{0x00, []byte{0x0F}},                   // panel setting
		{0x61, []byte{0x03, 0x20, 0x01, 0xE0}}, // resolution 800x480
		{0x15, []byte{0x00}},
		{0x50, []byte{0x11, 0x07}}, // VCOM and data interval
		{0x60, []byte{0x22}},       // TCON
		{0x65, []byte{0x00, 0x00, 0x00, 0x00}},
	}
	for _, s := range seq {
		if err := e.cmd(s.c, s.data...); err != nil {
			return err
		}
	}
	return nil
}

func (e *EPD7in5bV2) refresh(black, red []byte) error {
	if err := e.cmd(0x10, black...); err != nil {
		return err
	}
	if err := e.cmd(0x13, red...); err != nil {
		return err
	}
	if err := e.cmd(0x12); err != nil {
		return err
	}
	e.sleep(100 * time.Millisecond)
	return e.waitBusy()
}

func (e *EPD7in5bV2) deepSleep() error {
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

// run wraps a panel session so the bus is always closed (panel unpowered) on errors.
func (e *EPD7in5bV2) run(body func() error) error {
	if err := e.init(); err != nil {
		e.bus.Close()
		return err
	}
	if err := body(); err != nil {
		e.bus.Close()
		return err
	}
	if err := e.deepSleep(); err != nil {
		e.bus.Close() // never leave the panel powered or the SPI device open
		return err
	}
	return nil
}

// Show transfers the frame and refreshes the panel (~16-20 s).
func (e *EPD7in5bV2) Show(f render.Frame) error {
	if f.W != 800 || f.H != 480 {
		return fmt.Errorf("epd: frame %dx%d, panel is 800x480", f.W, f.H)
	}
	return e.run(func() error { return e.refresh(f.Black, f.Red) })
}

// Clear cycles black → red → white to remove ghosting.
func (e *EPD7in5bV2) Clear(cycles int) error {
	n := 800 / 8 * 480
	fill := func(b byte) []byte {
		p := make([]byte, n)
		for i := range p {
			p[i] = b
		}
		return p
	}
	allBlack, white, noRed, allRed := fill(0x00), fill(0xFF), fill(0x00), fill(0xFF)
	return e.run(func() error {
		for i := 0; i < cycles; i++ {
			for _, step := range [][2][]byte{{allBlack, noRed}, {white, allRed}, {white, noRed}} {
				if err := e.refresh(step[0], step[1]); err != nil {
					return err
				}
			}
		}
		return nil
	})
}
