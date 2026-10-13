//go:build linux

package epd

import (
	"fmt"
	"os"
	"time"
	"unsafe"

	"github.com/warthog618/go-gpiocdev"
	"golang.org/x/sys/unix"
)

// Waveshare HAT wiring (BCM numbering) and SPI settings, as in Python's epdconfig.py.
const (
	pinRST   = 17
	pinDC    = 25
	pinPWR   = 18
	pinBusy  = 24
	gpioChip = "gpiochip0"
	spiDev   = "/dev/spidev0.0"
	spiSpeed = 4_000_000

	spiIocWrMode        = 0x40016b01
	spiIocWrBitsPerWord = 0x40016b03
	spiIocWrMaxSpeedHz  = 0x40046b04
	spiChunk            = 4096 // spidev default bufsiz
)

type linuxBus struct {
	rst, dc, pwr, busy *gpiocdev.Line
	spi                *os.File
}

func newLinuxBus() (*linuxBus, error) {
	b := &linuxBus{}
	var err error
	if b.rst, err = gpiocdev.RequestLine(gpioChip, pinRST, gpiocdev.AsOutput(0)); err != nil {
		return nil, fmt.Errorf("gpio %d: %w", pinRST, err)
	}
	if b.dc, err = gpiocdev.RequestLine(gpioChip, pinDC, gpiocdev.AsOutput(0)); err != nil {
		return nil, fmt.Errorf("gpio %d: %w", pinDC, err)
	}
	if b.pwr, err = gpiocdev.RequestLine(gpioChip, pinPWR, gpiocdev.AsOutput(0)); err != nil {
		return nil, fmt.Errorf("gpio %d: %w", pinPWR, err)
	}
	if b.busy, err = gpiocdev.RequestLine(gpioChip, pinBusy, gpiocdev.AsInput, gpiocdev.WithPullDown); err != nil {
		return nil, fmt.Errorf("gpio %d: %w", pinBusy, err)
	}
	return b, nil
}

func ioctl(fd uintptr, req uintptr, val unsafe.Pointer) error {
	if _, _, errno := unix.Syscall(unix.SYS_IOCTL, fd, req, uintptr(val)); errno != 0 {
		return errno
	}
	return nil
}

func (b *linuxBus) Open() error {
	if err := b.pwr.SetValue(1); err != nil {
		return err
	}
	time.Sleep(10 * time.Millisecond) // let the HAT's supply settle
	f, err := openSPI(spiSpeed)
	if err != nil {
		return err
	}
	b.spi = f
	return nil
}

// openSPI opens spidev0.0 in mode 0, 8 bit.
func openSPI(speed uint32) (*os.File, error) {
	f, err := os.OpenFile(spiDev, os.O_RDWR, 0)
	if err != nil {
		return nil, fmt.Errorf("open %s (is SPI enabled?): %w", spiDev, err)
	}
	mode, bits := uint8(0), uint8(8)
	for _, s := range []struct {
		req uintptr
		val unsafe.Pointer
	}{{spiIocWrMode, unsafe.Pointer(&mode)}, {spiIocWrBitsPerWord, unsafe.Pointer(&bits)}, {spiIocWrMaxSpeedHz, unsafe.Pointer(&speed)}} {
		if err := ioctl(f.Fd(), s.req, s.val); err != nil {
			f.Close()
			return nil, fmt.Errorf("spi ioctl %#x: %w", s.req, err)
		}
	}
	return f, nil
}

// writeSPI writes p in spidev-sized chunks.
func writeSPI(f *os.File, p []byte) error {
	for len(p) > 0 {
		n := min(len(p), spiChunk)
		if _, err := f.Write(p[:n]); err != nil {
			return err
		}
		p = p[n:]
	}
	return nil
}

func (b *linuxBus) Close() error {
	if b.spi != nil {
		b.spi.Close()
		b.spi = nil
	}
	b.rst.SetValue(0)
	b.dc.SetValue(0)
	return b.pwr.SetValue(0)
}

func (b *linuxBus) SetReset(high bool) error {
	v := 0
	if high {
		v = 1
	}
	return b.rst.SetValue(v)
}

func (b *linuxBus) write(dc int, p []byte) error {
	if err := b.dc.SetValue(dc); err != nil {
		return err
	}
	return writeSPI(b.spi, p)
}

func (b *linuxBus) Command(c byte) error { return b.write(0, []byte{c}) }

func (b *linuxBus) Data(p ...byte) error { return b.write(1, p) }

func (b *linuxBus) BusyLevel() (int, error) { return b.busy.Value() }

// openers builds the hardware display for models registered by driver files.
var openers = map[string]func() (Display, error){}

// openSingle wires a single-controller driver to the standard HAT bus.
func openSingle(newDriver func(Bus, func(time.Duration)) Display) func() (Display, error) {
	return func() (Display, error) {
		bus, err := newLinuxBus()
		if err != nil {
			return nil, err
		}
		return newDriver(bus, time.Sleep), nil
	}
}

// Open returns the hardware display for a model name from settings.json.
func Open(model string) (Display, error) {
	if open, ok := openers[model]; ok {
		return open()
	}
	switch model {
	case "epd_7_in_5_v2_colour":
		bus, err := newLinuxBus()
		if err != nil {
			return nil, err
		}
		return NewEPD7in5bV2(bus, time.Sleep), nil
	case "epd_12_in_48_colour_V2":
		bus, err := newLinuxBus12()
		if err != nil {
			return nil, err
		}
		return NewEPD12in48bV2(bus, time.Sleep), nil
	}
	return nil, fmt.Errorf("display model %q not supported", model)
}
