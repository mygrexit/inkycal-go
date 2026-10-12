//go:build linux

package epd

import (
	"fmt"
	"os"
	"unsafe"

	"github.com/warthog618/go-gpiocdev"
)

// Wiring of the Waveshare 12.48" HAT (BCM numbering, Python: epdconfig_12_in_48.py).
// SCK/MOSI (11/10) run on hardware SPI; the chip selects are plain GPIOs, so
// config.txt needs "dtoverlay=spi0-0cs" to stop the SPI driver claiming GPIO 7 and 8.
var (
	pins12CS   = map[Part]int{M1: 8, S1: 7, M2: 17, S2: 18}
	pins12Busy = map[Part]int{M1: 5, S1: 19, M2: 27, S2: 24}
)

const (
	pin12DCM1S1  = 13
	pin12DCM2S2  = 22
	pin12RSTM1S1 = 6
	pin12RSTM2S2 = 23
	spi12Speed   = 4_000_000
	spiMode3Wire = 0x10 // SPI_3WIRE: read on the MOSI line
)

var parts12 = []Part{M1, S1, M2, S2}

type linuxBus12 struct {
	cs, busy         map[Part]*gpiocdev.Line
	dcM1S1, dcM2S2   *gpiocdev.Line
	rstM1S1, rstM2S2 *gpiocdev.Line
	spi              *os.File
}

func newLinuxBus12() (*linuxBus12, error) {
	b := &linuxBus12{cs: map[Part]*gpiocdev.Line{}, busy: map[Part]*gpiocdev.Line{}}
	out := func(pin, initial int) (*gpiocdev.Line, error) {
		l, err := gpiocdev.RequestLine(gpioChip, pin, gpiocdev.AsOutput(initial))
		if err != nil {
			return nil, fmt.Errorf("gpio %d: %w (is dtoverlay=spi0-0cs set?)", pin, err)
		}
		return l, nil
	}
	var err error
	for _, p := range parts12 {
		if b.cs[p], err = out(pins12CS[p], 1); err != nil {
			return nil, err
		}
		if b.busy[p], err = gpiocdev.RequestLine(gpioChip, pins12Busy[p], gpiocdev.AsInput, gpiocdev.WithPullDown); err != nil {
			return nil, fmt.Errorf("gpio %d: %w", pins12Busy[p], err)
		}
	}
	for _, l := range []struct {
		dst *(*gpiocdev.Line)
		pin int
	}{{&b.dcM1S1, pin12DCM1S1}, {&b.dcM2S2, pin12DCM2S2}, {&b.rstM1S1, pin12RSTM1S1}, {&b.rstM2S2, pin12RSTM2S2}} {
		if *l.dst, err = out(l.pin, 0); err != nil {
			return nil, err
		}
	}
	return b, nil
}

func (b *linuxBus12) Open() error {
	f, err := openSPI(spi12Speed)
	if err != nil {
		return err
	}
	b.spi = f
	return nil
}

func (b *linuxBus12) Close() error {
	if b.spi != nil {
		b.spi.Close()
		b.spi = nil
	}
	for _, p := range parts12 {
		b.cs[p].SetValue(1)
	}
	b.dcM1S1.SetValue(0)
	b.dcM2S2.SetValue(0)
	b.rstM1S1.SetValue(0)
	return b.rstM2S2.SetValue(0)
}

func (b *linuxBus12) SetReset(high bool) error {
	v := 0
	if high {
		v = 1
	}
	if err := b.rstM1S1.SetValue(v); err != nil {
		return err
	}
	return b.rstM2S2.SetValue(v)
}

// selectParts sets DC on the pairs involved and pulls the chip selects of p low.
func (b *linuxBus12) selectParts(p Part, dc int) error {
	if p&(M1|S1) != 0 {
		if err := b.dcM1S1.SetValue(dc); err != nil {
			return err
		}
	}
	if p&(M2|S2) != 0 {
		if err := b.dcM2S2.SetValue(dc); err != nil {
			return err
		}
	}
	for _, q := range parts12 {
		if p&q != 0 {
			if err := b.cs[q].SetValue(0); err != nil {
				return err
			}
		}
	}
	return nil
}

func (b *linuxBus12) deselect() error {
	for _, q := range parts12 {
		if err := b.cs[q].SetValue(1); err != nil {
			return err
		}
	}
	return nil
}

func (b *linuxBus12) write(p Part, dc int, data []byte) error {
	if err := b.selectParts(p, dc); err != nil {
		return err
	}
	err := writeSPI(b.spi, data)
	if derr := b.deselect(); err == nil {
		err = derr
	}
	return err
}

func (b *linuxBus12) Command(p Part, c byte) error { return b.write(p, 0, []byte{c}) }

func (b *linuxBus12) Data(p Part, data ...byte) error { return b.write(p, 1, data) }

func (b *linuxBus12) BusyLevel(p Part) (int, error) { return b.busy[p].Value() }

// ReadData clocks one byte in over the MOSI line (the panel's SPI is 3-wire).
func (b *linuxBus12) ReadData(p Part) (byte, error) {
	setMode := func(m uint8) error { return ioctl(b.spi.Fd(), spiIocWrMode, unsafe.Pointer(&m)) }
	if err := setMode(spiMode3Wire); err != nil {
		return 0, fmt.Errorf("spi 3-wire mode: %w", err)
	}
	defer setMode(0)
	if err := b.selectParts(p, 1); err != nil {
		return 0, err
	}
	defer b.deselect()
	var buf [1]byte
	if _, err := b.spi.Read(buf[:]); err != nil {
		return 0, err
	}
	return buf[0], nil
}
