package epd

import (
	"errors"
	"fmt"
	"time"

	"inkycal/internal/render"
)

// panel is the shared skeleton of the single-controller drivers (4.2", 5.83",
// 7.5" V1/V2/V3, 13.3"). Each model fills in its timings and the hooks below with
// the sequences of its upstream Python driver; panel does reset, BUSY polling,
// sessions (init → body → deep sleep, bus closed on any error), frame checks and
// calibration. testdata/trace/<model>.txt pins every sequence byte for byte.
type panel struct {
	bus         Bus
	sleep       func(time.Duration)
	BusyTimeout time.Duration // give up waiting for the panel after this long

	w, h   int
	colour bool // takes black + red planes; otherwise red is merged into black

	resetHigh, resetLow time.Duration // upstream reset(): RST high, low, high

	// BUSY polling as in the upstream ReadBusy. busyLevel is the level that means
	// busy (UC81xx: 0, SSD16xx: 1). poll71 sends GET_STATUS (0x71) before the
	// first read and before every retry. busyTail is slept once BUSY is clear.
	busyLevel          int
	poll71             bool
	pollStep, busyTail time.Duration

	initSeq   func() error // everything after reset() in upstream init()
	refresh   func(black, red []byte) error
	sleepSeq  func() error  // nil: power off (0x02), wait, deep sleep (0x07 0xA5)
	sleepWait time.Duration // upstream delay after the deep-sleep command, before module_exit
}

// newPanel returns a panel with the defaults most upstream drivers share:
// 60 s BUSY timeout, 200 ms RST high phases, BUSY low = busy.
func newPanel(bus Bus, sleep func(time.Duration), w, h int) *panel {
	return &panel{bus: bus, sleep: sleep, BusyTimeout: 60 * time.Second, w: w, h: h, resetHigh: 200 * time.Millisecond}
}

func (e *panel) Size() (int, int) { return e.w, e.h }

// resolution is the 4-byte w, h argument of the resolution commands (0x61).
func (e *panel) resolution() []byte {
	return []byte{byte(e.w >> 8), byte(e.w), byte(e.h >> 8), byte(e.h)}
}

func (e *panel) cmd(c byte, data ...byte) error {
	if err := e.bus.Command(c); err != nil {
		return err
	}
	if len(data) == 0 {
		return nil
	}
	return e.bus.Data(data...)
}

// op is one command with its data.
type op struct {
	c    byte
	data []byte
}

func (e *panel) cmds(seq ...op) error {
	for _, s := range seq {
		if err := e.cmd(s.c, s.data...); err != nil {
			return err
		}
	}
	return nil
}

// waitBusy mirrors upstream ReadBusy, plus a timeout. One read per loop pass, like
// the upstream while loops, so the trace stub's alternating BUSY sees the same reads.
func (e *panel) waitBusy() error {
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
		if level != e.busyLevel {
			break
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
	if e.busyTail > 0 {
		e.sleep(e.busyTail)
	}
	return nil
}

func (e *panel) init() error {
	if err := e.bus.Open(); err != nil {
		return err
	}
	for _, step := range []struct {
		high bool
		wait time.Duration
	}{{true, e.resetHigh}, {false, e.resetLow}, {true, e.resetHigh}} {
		if err := e.bus.SetReset(step.high); err != nil {
			return err
		}
		e.sleep(step.wait)
	}
	return e.initSeq()
}

func (e *panel) deepSleep() error {
	seq := e.sleepSeq
	if seq == nil {
		seq = e.powerOffDeepSleep
	}
	if err := seq(); err != nil {
		return err
	}
	if e.sleepWait > 0 {
		e.sleep(e.sleepWait)
	}
	return e.bus.Close()
}

// powerOffDeepSleep is the UC81xx sleep(): power off, wait, deep sleep.
func (e *panel) powerOffDeepSleep() error {
	if err := e.cmd(0x02); err != nil { // power off
		return err
	}
	if err := e.waitBusy(); err != nil {
		return err
	}
	return e.cmd(0x07, 0xA5) // deep sleep
}

func (e *panel) run(body func() error) error {
	return session(e.bus.Close, e.init, body, e.deepSleep)
}

// Show transfers the frame and refreshes the panel.
func (e *panel) Show(f render.Frame) error {
	if err := checkFrame(f, e.w, e.h); err != nil {
		return err
	}
	black := f.Black
	if !e.colour {
		black = mergedBlack(f) // b/w panel: red shows as black (upstream main.py)
	}
	return e.run(func() error { return e.refresh(black, f.Red) })
}

// Clear runs upstream calibrate() in one session ending in deep sleep.
func (e *panel) Clear(cycles int) error {
	return e.run(calibrate(e.w/8*e.h, e.colour, cycles, e.refresh))
}

// --- helpers shared with the four-controller 12.48" V1 driver ---

// checkFrame rejects frames that do not match a w x h panel (both planes required).
func checkFrame(f render.Frame, w, h int) error {
	n := w / 8 * h
	if f.W != w || f.H != h || len(f.Black) != n || len(f.Red) != n {
		return fmt.Errorf("epd: frame %dx%d, panel is %dx%d", f.W, f.H, w, h)
	}
	return nil
}

// session runs init, body and deep sleep; after any error the bus is closed
// (panel unpowered) and the error returned.
func session(closeBus func() error, init, body, deepSleep func() error) error {
	for _, step := range []func() error{init, body, deepSleep} {
		if err := step(); err != nil {
			closeBus()
			return err
		}
	}
	return nil
}

// calibrate ports display.py calibrate() for planes of n bytes: per cycle
// black → white on b/w panels, black → colour → white on colour panels.
// Planes are in render.Frame terms (black: 0 = black, red: 1 = red).
func calibrate(n int, colour bool, cycles int, refresh func(black, red []byte) error) func() error {
	allBlack, white, noRed, allRed := filled(n, 0x00), filled(n, 0xFF), filled(n, 0x00), filled(n, 0xFF)
	steps := [][2][]byte{{allBlack, noRed}, {white, noRed}}
	if colour {
		steps = [][2][]byte{{allBlack, noRed}, {white, allRed}, {white, noRed}}
	}
	return func() error {
		for i := 0; i < cycles; i++ {
			for _, s := range steps {
				if err := refresh(s[0], s[1]); err != nil {
					return err
				}
			}
		}
		return nil
	}
}

// filled returns n bytes of b.
func filled(n int, b byte) []byte {
	p := make([]byte, n)
	for i := range p {
		p[i] = b
	}
	return p
}

// inverted returns ^p (panels whose RAM wants 1 = black, or 0 = red).
func inverted(p []byte) []byte {
	out := make([]byte, len(p))
	for i, b := range p {
		out[i] = ^b
	}
	return out
}
