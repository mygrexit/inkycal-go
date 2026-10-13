package epd

import (
	"bytes"
	"errors"
	"fmt"
	"time"

	"inkycal/internal/render"
)

func init() {
	Models["epd_13_in_3"] = [2]int{w13in3, h13in3}
	Models["epd_13_in_3_colour"] = [2]int{w13in3, h13in3}
}

const (
	w13in3, h13in3 = 960, 680
	plane13in3     = w13in3 / 8 * h13in3
)

// EPD13in3 drives the Waveshare 13.3" panels (Python: epd_13_in_3 for black/white (K),
// epd_13_in_3_colour for black/white/red (B)). Both use the standard HAT; upstream
// toggles CS_PIN around every byte, which its epdconfig ignores (the kernel drives CE0),
// so only DC and the SPI bytes matter.
type EPD13in3 struct {
	bus         Bus
	sleep       func(time.Duration)
	colour      bool
	BusyTimeout time.Duration // give up waiting for the panel after this long
}

// NewEPD13in3 creates the black/white driver; sleep is time.Sleep in production.
func NewEPD13in3(bus Bus, sleep func(time.Duration)) *EPD13in3 {
	return &EPD13in3{bus: bus, sleep: sleep, BusyTimeout: 60 * time.Second}
}

// NewEPD13in3b creates the black/white/red driver.
func NewEPD13in3b(bus Bus, sleep func(time.Duration)) *EPD13in3 {
	e := NewEPD13in3(bus, sleep)
	e.colour = true
	return e
}

func (e *EPD13in3) Size() (int, int) { return w13in3, h13in3 }

func (e *EPD13in3) cmd(c byte, data ...byte) error {
	if err := e.bus.Command(c); err != nil {
		return err
	}
	if len(data) == 0 {
		return nil
	}
	return e.bus.Data(data...)
}

// waitBusy mirrors upstream ReadBusy: BUSY high = busy, polled every 20 ms.
func (e *EPD13in3) waitBusy() error {
	const step = 20 * time.Millisecond
	level, err := e.bus.BusyLevel()
	if err != nil {
		return err
	}
	for waited := time.Duration(0); level == 1; waited += step {
		if waited >= e.BusyTimeout {
			return errors.New("epd: panel stays busy, check wiring and power")
		}
		if level, err = e.bus.BusyLevel(); err != nil {
			return err
		}
		e.sleep(step)
	}
	e.sleep(step)
	return nil
}

func (e *EPD13in3) init() error {
	if err := e.bus.Open(); err != nil {
		return err
	}
	for _, step := range []struct {
		high bool
		wait time.Duration
	}{{true, 20 * time.Millisecond}, {false, 2 * time.Millisecond}, {true, 20 * time.Millisecond}} {
		if err := e.bus.SetReset(step.high); err != nil {
			return err
		}
		e.sleep(step.wait)
	}
	if err := e.waitBusy(); err != nil {
		return err
	}
	if err := e.cmd(0x12); err != nil { // SWRESET
		return err
	}
	if err := e.waitBusy(); err != nil {
		return err
	}
	border := byte(0x05)
	if e.colour {
		border = 0x01
	}
	seq := []struct {
		c    byte
		data []byte
	}{
		{0x0C, []byte{0xAE, 0xC7, 0xC3, 0xC0, 0x80}}, // booster soft start
		{0x01, []byte{0xA7, 0x02, 0x00}},             // driver output control: 680 gates
		{0x11, []byte{0x03}},                         // data entry mode
		{0x44, []byte{0x00, 0x00, 0xBF, 0x03}},       // RAM x range 0..959
		{0x45, []byte{0x00, 0x00, 0xA7, 0x02}},       // RAM y range 0..679
		{0x3C, []byte{border}},                       // border waveform
		{0x18, []byte{0x80}},                         // internal temperature sensor
		{0x4E, []byte{0x00, 0x00}},                   // RAM x counter
		{0x4F, []byte{0x00, 0x00}},                   // RAM y counter
	}
	for _, s := range seq {
		if err := e.cmd(s.c, s.data...); err != nil {
			return err
		}
	}
	if e.colour {
		return e.waitBusy()
	}
	return nil
}

// refresh writes the planes (red is ignored on the black/white panel) and updates.
func (e *EPD13in3) refresh(black, red []byte) error {
	if err := e.cmd(0x24, black...); err != nil {
		return err
	}
	if e.colour {
		if err := e.cmd(0x26, red...); err != nil { // upstream inverts getbuffer: 1 = red
			return err
		}
	}
	if err := e.cmd(0x22, 0xF7); err != nil { // display update control
		return err
	}
	if err := e.cmd(0x20); err != nil { // activate
		return err
	}
	return e.waitBusy()
}

func (e *EPD13in3) deepSleep() error {
	if err := e.cmd(0x10, 0x03); err != nil {
		return err
	}
	e.sleep(2 * time.Second)
	return e.bus.Close()
}

// run wraps a panel session so the bus is always closed (panel unpowered) on errors.
func (e *EPD13in3) run(body func() error) error {
	if err := e.init(); err != nil {
		e.bus.Close()
		return err
	}
	if err := body(); err != nil {
		e.bus.Close()
		return err
	}
	if err := e.deepSleep(); err != nil {
		e.bus.Close()
		return err
	}
	return nil
}

// Show transfers the frame and refreshes the panel.
func (e *EPD13in3) Show(f render.Frame) error {
	if f.W != w13in3 || f.H != h13in3 || len(f.Black) != plane13in3 || len(f.Red) != plane13in3 {
		return fmt.Errorf("epd: frame %dx%d, panel is %dx%d", f.W, f.H, w13in3, h13in3)
	}
	if !e.colour {
		return e.run(func() error { return e.refresh(mergedBlack(f), nil) })
	}
	return e.run(func() error { return e.refresh(f.Black, f.Red) })
}

// Clear cycles black → white (black/white panel) or black → red → white (colour panel).
func (e *EPD13in3) Clear(cycles int) error {
	fill := func(b byte) []byte { return bytes.Repeat([]byte{b}, plane13in3) }
	allBlack, white, noRed, allRed := fill(0x00), fill(0xFF), fill(0x00), fill(0xFF)
	steps := [][2][]byte{{allBlack, noRed}, {white, noRed}}
	if e.colour {
		steps = [][2][]byte{{allBlack, noRed}, {white, allRed}, {white, noRed}}
	}
	return e.run(func() error {
		for i := 0; i < cycles; i++ {
			for _, step := range steps {
				if err := e.refresh(step[0], step[1]); err != nil {
					return err
				}
			}
		}
		return nil
	})
}
