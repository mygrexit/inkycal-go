//go:build linux

package epd

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"runtime"
	"strconv"
	"strings"
	"time"
	"unsafe"

	"github.com/warthog618/go-gpiocdev"

	"inkycal/internal/config"
)

// IT8951 HAT wiring (DEV_Config.h): RST 17, HRDY ("BUSY") 24, CS 8 (= CE0) driven by
// hand. bcm2835 uses SPI mode 0, MSB first, clock divider 32 (12.5 MHz at a 400 MHz core).
const (
	itPinRST  = 17
	itPinHRDY = 24
	itPinCS   = 8
	itSpeed   = 12_000_000

	spiNoCS          = 0x40
	spiIocMessage2   = 0x40406b00 // SPI_IOC_MESSAGE(2): _IOW('k', 0, char[2*32])
	itHRDYTimeout    = 10 * time.Second
	itPreambleGap    = 20 // µs between preamble and data when the kernel owns CS
	spidevBufsizPath = "/sys/module/spidev/parameters/bufsiz"
)

func init() {
	for _, m := range []string{"7_in_8", "9_in_7", "10_in_3"} {
		openers[m] = func() (Display, error) {
			bus, err := newIT8951LinuxBus()
			if err != nil {
				return nil, err
			}
			return NewIT8951(bus, time.Sleep, m, settingsVCOM())
		}
	}
}

// settingsVCOM reads "vcom" (or "VCOM") from settings.json like inkycal/settings.py.
func settingsVCOM() string {
	path, err := config.FindSettings(SettingsPath)
	if err != nil {
		slog.Info("epd: no settings.json for VCOM, using default", "vcom", itDefaultVCOM)
		return itDefaultVCOM
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		slog.Info("epd: cannot read settings.json for VCOM, using default", "vcom", itDefaultVCOM, "err", err)
		return itDefaultVCOM
	}
	var m map[string]json.RawMessage
	if err := json.Unmarshal(raw, &m); err != nil {
		return itDefaultVCOM
	}
	v, ok := m["vcom"]
	if !ok {
		v, ok = m["VCOM"]
	}
	if !ok {
		slog.Info("epd: settings.json has no vcom, using upstream default (set \"vcom\" to the value printed on the panel's FPC)", "vcom", itDefaultVCOM)
		return itDefaultVCOM
	}
	var s string
	if json.Unmarshal(v, &s) == nil {
		return s
	}
	return strings.TrimSpace(string(v)) // a JSON number, as the web UI saves it
}

// spiIocTransfer is struct spi_ioc_transfer (linux/spi/spidev.h).
type spiIocTransfer struct {
	txBuf, rxBuf   uint64
	len, speedHz   uint32
	delayUsecs     uint16
	bitsPerWord    uint8
	csChange       uint8
	txNbits        uint8
	rxNbits        uint8
	wordDelayUsecs uint8
	pad            uint8
}

type it8951LinuxBus struct {
	rst, hrdy, cs *gpiocdev.Line // cs nil: the kernel drives CE0
	spi           *os.File
	bufsiz        int
}

func newIT8951LinuxBus() (*it8951LinuxBus, error) {
	b := &it8951LinuxBus{bufsiz: 4096}
	var err error
	if b.rst, err = gpiocdev.RequestLine(gpioChip, itPinRST, gpiocdev.AsOutput(0)); err != nil {
		return nil, fmt.Errorf("gpio %d: %w", itPinRST, err)
	}
	if b.hrdy, err = gpiocdev.RequestLine(gpioChip, itPinHRDY, gpiocdev.AsInput); err != nil {
		b.rst.Close()
		return nil, fmt.Errorf("gpio %d: %w", itPinHRDY, err)
	}
	// Upstream holds CS low across preamble, HRDY wait and data. That needs GPIO 8 free
	// (dtoverlay=spi0-0cs); with the standard overlay the kernel owns CE0 and each
	// transaction becomes one SPI message with a short gap after the preamble.
	if b.cs, err = gpiocdev.RequestLine(gpioChip, itPinCS, gpiocdev.AsOutput(1)); err != nil {
		b.cs = nil
		slog.Info("epd: IT8951 chip select driven by the kernel (GPIO 8 busy)", "err", err)
	}
	if raw, err := os.ReadFile(spidevBufsizPath); err == nil {
		if n, err := strconv.Atoi(strings.TrimSpace(string(raw))); err == nil && n >= 64 {
			b.bufsiz = n
		}
	}
	return b, nil
}

func (b *it8951LinuxBus) Open() error {
	f, err := os.OpenFile(spiDev, os.O_RDWR, 0)
	if err != nil {
		return fmt.Errorf("open %s (is SPI enabled?): %w", spiDev, err)
	}
	mode, bits, speed := uint8(0), uint8(8), uint32(itSpeed)
	if b.cs != nil {
		mode |= spiNoCS
	}
	for _, s := range []struct {
		req uintptr
		val unsafe.Pointer
	}{{spiIocWrMode, unsafe.Pointer(&mode)}, {spiIocWrBitsPerWord, unsafe.Pointer(&bits)}, {spiIocWrMaxSpeedHz, unsafe.Pointer(&speed)}} {
		if err := ioctl(f.Fd(), s.req, s.val); err != nil {
			f.Close()
			return fmt.Errorf("spi ioctl %#x: %w", s.req, err)
		}
	}
	b.spi = f
	return nil
}

func (b *it8951LinuxBus) Close() error {
	if b.spi != nil {
		b.spi.Close()
		b.spi = nil
	}
	if b.cs != nil {
		b.cs.SetValue(0) // DEV_Module_Exit
	}
	return b.rst.SetValue(0)
}

func (b *it8951LinuxBus) SetReset(high bool) error {
	v := 0
	if high {
		v = 1
	}
	return b.rst.SetValue(v)
}

// waitHRDY is EPD_IT8951_ReadBusy (HRDY high = ready), with a timeout.
func (b *it8951LinuxBus) waitHRDY() error {
	deadline := time.Now().Add(itHRDYTimeout)
	for {
		v, err := b.hrdy.Value()
		if err != nil {
			return err
		}
		if v == 1 {
			return nil
		}
		if time.Now().After(deadline) {
			return errors.New("epd: IT8951 HRDY stays low, check wiring and power")
		}
	}
}

// message sends one SPI message (CS asserted throughout): preamble, optional gap, then
// tx; rx (if not nil) receives what was clocked in during tx.
func (b *it8951LinuxBus) message(preamble uint16, tx, rx []byte) error {
	pre := []byte{byte(preamble >> 8), byte(preamble)}
	xfers := []spiIocTransfer{
		{txBuf: uint64(uintptr(unsafe.Pointer(&pre[0]))), len: 2, speedHz: itSpeed, delayUsecs: itPreambleGap, bitsPerWord: 8},
		{txBuf: uint64(uintptr(unsafe.Pointer(&tx[0]))), len: uint32(len(tx)), speedHz: itSpeed, bitsPerWord: 8},
	}
	if rx != nil {
		xfers[1].rxBuf = uint64(uintptr(unsafe.Pointer(&rx[0])))
	}
	err := ioctl(b.spi.Fd(), spiIocMessage2, unsafe.Pointer(&xfers[0]))
	runtime.KeepAlive(pre)
	runtime.KeepAlive(tx)
	runtime.KeepAlive(rx)
	return err
}

// manual is one transaction with CS on GPIO 8, exactly like the C code.
func (b *it8951LinuxBus) manual(preamble uint16, body func() error) error {
	if err := b.waitHRDY(); err != nil {
		return err
	}
	if err := b.cs.SetValue(0); err != nil {
		return err
	}
	err := func() error {
		if err := writeSPI(b.spi, []byte{byte(preamble >> 8), byte(preamble)}); err != nil {
			return err
		}
		if err := b.waitHRDY(); err != nil {
			return err
		}
		return body()
	}()
	if e := b.cs.SetValue(1); err == nil {
		err = e
	}
	return err
}

func (b *it8951LinuxBus) Command(c uint16) error {
	word := []byte{byte(c >> 8), byte(c)}
	if b.cs != nil {
		return b.manual(0x6000, func() error { return writeSPI(b.spi, word) })
	}
	if err := b.waitHRDY(); err != nil {
		return err
	}
	return b.message(0x6000, word, nil)
}

func (b *it8951LinuxBus) Write(p []byte) error {
	if b.cs != nil {
		return b.manual(0x0000, func() error { return writeSPI(b.spi, p) })
	}
	// spidev limits one message to bufsiz bytes, so long data runs become several write
	// transactions; the IT8951 consumes them in order (upstream itself sends one word per
	// transaction).
	max := (b.bufsiz - 2) &^ 1
	for len(p) > 0 {
		n := min(len(p), max)
		if err := b.waitHRDY(); err != nil {
			return err
		}
		if err := b.message(0x0000, p[:n], nil); err != nil {
			return err
		}
		p = p[n:]
	}
	return nil
}

func (b *it8951LinuxBus) Read(n int) ([]uint16, error) {
	buf := make([]byte, 2+2*n) // dummy word, then data
	if b.cs != nil {
		err := b.manual(0x1000, func() error {
			if _, err := b.spi.Read(buf[:2]); err != nil {
				return err
			}
			if err := b.waitHRDY(); err != nil {
				return err
			}
			_, err := b.spi.Read(buf[2:])
			return err
		})
		if err != nil {
			return nil, err
		}
	} else {
		if err := b.waitHRDY(); err != nil {
			return nil, err
		}
		tx := make([]byte, len(buf))
		if err := b.message(0x1000, tx, buf); err != nil {
			return nil, err
		}
	}
	out := make([]uint16, n)
	for i := range out {
		out[i] = uint16(buf[2+2*i])<<8 | uint16(buf[3+2*i])
	}
	return out, nil
}

// spi_ioc_transfer must be exactly 32 bytes.
var (
	_ [32 - unsafe.Sizeof(spiIocTransfer{})]byte
	_ [unsafe.Sizeof(spiIocTransfer{}) - 32]byte
)
