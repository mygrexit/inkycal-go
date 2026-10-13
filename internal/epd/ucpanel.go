package epd

import (
	"errors"
	"fmt"
	"time"

	"inkycal/internal/render"
)

// ucPanel is the shared skeleton of the single-controller Waveshare drivers ported
// from upstream (UC8176/UC8179 family: 4.2", 5.83" and their colour variants).
// Each model supplies its init sequence and refresh; ucPanel handles BUSY polling,
// sessions (init → body → deep sleep, bus closed on any error) and calibration.
type ucPanel struct {
	bus         Bus
	sleep       func(time.Duration)
	BusyTimeout time.Duration // give up waiting for the panel after this long

	w, h   int
	colour bool // takes black + red planes; otherwise red is merged into black

	// BUSY polling as in the upstream ReadBusy: low = busy. poll71 sends
	// GET_STATUS (0x71) before the first read and before every retry.
	poll71   bool
	pollStep time.Duration

	resetLow  time.Duration // RST low pulse of upstream reset()
	initSeq   func() error  // everything after reset() in upstream init()
	refresh   func(black, red []byte) error
	sleepWait time.Duration // upstream delay after DEEP_SLEEP, before module_exit
}

func (e *ucPanel) Size() (int, int) { return e.w, e.h }

func (e *ucPanel) cmd(c byte, data ...byte) error {
	if err := e.bus.Command(c); err != nil {
		return err
	}
	if len(data) == 0 {
		return nil
	}
	return e.bus.Data(data...)
}

// waitBusy mirrors upstream ReadBusy loop for loop, plus a timeout.
func (e *ucPanel) waitBusy() error {
	if e.poll71 {
		if err := e.bus.Command(0x71); err != nil {
			return err
		}
	}
	for waited := time.Duration(0); ; waited += e.pollStep {
		level, err := e.bus.BusyLevel()
		if err != nil {
			return err
		}
		if level != 0 {
			return nil
		}
		if waited >= e.BusyTimeout {
			return errors.New("epd: panel stays busy, check wiring and power")
		}
		if e.poll71 {
			if err := e.bus.Command(0x71); err != nil {
				return err
			}
		}
		e.sleep(e.pollStep)
	}
}

func (e *ucPanel) init() error {
	if err := e.bus.Open(); err != nil {
		return err
	}
	for _, step := range []struct {
		high bool
		wait time.Duration
	}{{true, 200 * time.Millisecond}, {false, e.resetLow}, {true, 200 * time.Millisecond}} {
		if err := e.bus.SetReset(step.high); err != nil {
			return err
		}
		e.sleep(step.wait)
	}
	return e.initSeq()
}

func (e *ucPanel) deepSleep() error {
	if err := e.cmd(0x02); err != nil { // power off
		return err
	}
	if err := e.waitBusy(); err != nil {
		return err
	}
	if err := e.cmd(0x07, 0xA5); err != nil { // deep sleep
		return err
	}
	e.sleep(e.sleepWait)
	return e.bus.Close()
}

// run wraps a panel session so the bus is always closed (panel unpowered) on errors.
func (e *ucPanel) run(body func() error) error {
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
func (e *ucPanel) Show(f render.Frame) error {
	n := (e.w + 7) / 8 * e.h
	if f.W != e.w || f.H != e.h || len(f.Black) != n || len(f.Red) != n {
		return fmt.Errorf("epd: frame %dx%d, panel is %dx%d", f.W, f.H, e.w, e.h)
	}
	black := f.Black
	if !e.colour {
		black = mergedBlack(f)
	}
	return e.run(func() error { return e.refresh(black, f.Red) })
}

// Clear runs upstream calibrate(): black → white per cycle on black/white panels,
// black → colour → white on colour panels, all in one session.
func (e *ucPanel) Clear(cycles int) error {
	n := (e.w + 7) / 8 * e.h
	allBlack, white, noRed, allRed := ucFilled(n, 0x00), ucFilled(n, 0xFF), ucFilled(n, 0x00), ucFilled(n, 0xFF)
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

// ucInvert returns ^p (upstream sends getbuffer of the colour image, where 0 = red).
func ucInvert(p []byte) []byte {
	out := make([]byte, len(p))
	for i, b := range p {
		out[i] = ^b
	}
	return out
}

// ucFilled returns n bytes of b.
func ucFilled(n int, b byte) []byte {
	p := make([]byte, n)
	for i := range p {
		p[i] = b
	}
	return p
}
