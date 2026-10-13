package epd

import (
	"errors"
	"fmt"
	"log/slog"
	"math"
	"strconv"
	"strings"
	"time"

	"inkycal/internal/render"
)

// Waveshare IT8951 e-Paper HAT ("parallel" panels). Upstream drives these through the C
// program in inkycal/display/drivers/parallel_drivers (main.c, example.c, EPD_IT8951.c,
// GUI_BMPfile.c, GUI_Paint.c), started once per refresh as `epd -<VCOM> 0 canvas.bmp`.
// This is a port of that program's command/register sequence. It is NOT trace-verified
// against upstream (no Python/stub path exists for the C program); it8951_test.go checks
// the sequence derived by hand from the C source.

func init() {
	Models["7_in_8"] = [2]int{1872, 1404}
	Models["9_in_7"] = [2]int{1200, 825}
	Models["10_in_3"] = [2]int{1872, 1404}
}

// it8951Bus is the HAT's SPI protocol (preamble words 0x6000 command, 0x0000 write,
// 0x1000 read). The bus waits for HRDY around every transaction like the C code does.
type it8951Bus interface {
	Open() error
	Close() error
	SetReset(high bool) error
	Command(c uint16) error       // one command transaction
	Write(p []byte) error         // data words as sent on the wire (big-endian pairs)
	Read(n int) ([]uint16, error) // read transaction: dummy word, then n words
}

// IT8951 command codes and registers (EPD_IT8951.h).
const (
	itSysRun      = 0x0001
	itSleep       = 0x0003
	itRegRead     = 0x0010
	itRegWrite    = 0x0011
	itLoadArea    = 0x0021
	itLoadEnd     = 0x0022
	itDisplayArea = 0x0034
	itDisplayBuf  = 0x0037
	itGetDevInfo  = 0x0302
	itVCOM        = 0x0039

	itRegI80CPCR = 0x0004
	itRegLISAR   = 0x0208
	itRegLUTAFSR = 0x1224

	itModeInit = 0
	itModeGC16 = 2

	itLoadArgs4bpp = 0x0020 // little endian <<8 | 4bpp(2) <<4 | rotate 0

	itDefaultVCOM = "2.0" // inkycal/settings.py Settings.VCOM
)

// IT8951 drives one of the IT8951-HAT panels.
type IT8951 struct {
	bus          it8951Bus
	sleep        func(time.Duration)
	w, h         int
	mirror       bool   // 10_in_3: upstream flips the image left-right before writing the BMP
	vcom         uint16 // in mV, as main.c computes it: (UWORD)(fabs(arg)*1000)
	ReadyTimeout time.Duration
}

// NewIT8951 creates the driver for one of the models registered above. vcom is the
// settings.json value ("vcom"/"VCOM", e.g. "1.48"), "" for upstream's default "2.0".
func NewIT8951(bus it8951Bus, sleep func(time.Duration), model, vcom string) (*IT8951, error) {
	size, ok := Models[model]
	if !ok || (model != "7_in_8" && model != "9_in_7" && model != "10_in_3") {
		return nil, fmt.Errorf("epd: %q is not an IT8951 model", model)
	}
	v, err := parseVCOM(vcom)
	if err != nil {
		slog.Warn("epd: invalid vcom in settings.json, using default", "vcom", vcom, "default", itDefaultVCOM, "err", err)
		v, _ = parseVCOM(itDefaultVCOM)
	}
	return &IT8951{bus: bus, sleep: sleep, w: size[0], h: size[1], mirror: model == "10_in_3", vcom: v, ReadyTimeout: 60 * time.Second}, nil
}

// parseVCOM mirrors main.c: sscanf("%lf") then (UWORD)(fabs(v)*1000), truncating.
// Upstream passes "-<value>", so a value saved with its own minus sign breaks sscanf
// there; here the sign is simply ignored.
func parseVCOM(s string) (uint16, error) {
	if s == "" {
		s = itDefaultVCOM
	}
	f, err := strconv.ParseFloat(strings.TrimSpace(s), 64)
	if err != nil {
		return 0, err
	}
	mv := math.Abs(f) * 1000
	if math.IsNaN(mv) || mv > math.MaxUint16 {
		return 0, fmt.Errorf("vcom %q out of range", s)
	}
	return uint16(mv), nil
}

func (e *IT8951) Size() (int, int) { return e.w, e.h }

// itDevInfo is IT8951_Dev_Info as read by GET_DEV_INFO (20 words).
type itDevInfo struct {
	panelW, panelH int
	addr           uint32
	lut            string
}

func (e *IT8951) cmd(c uint16, args ...uint16) error {
	if err := e.bus.Command(c); err != nil {
		return err
	}
	for _, a := range args { // EPD_IT8951_WriteMultiArg: one data transaction per word
		if err := e.bus.Write([]byte{byte(a >> 8), byte(a)}); err != nil {
			return err
		}
	}
	return nil
}

func (e *IT8951) readReg(reg uint16) (uint16, error) {
	if err := e.cmd(itRegRead, reg); err != nil {
		return 0, err
	}
	v, err := e.bus.Read(1)
	if err != nil {
		return 0, err
	}
	return v[0], nil
}

func (e *IT8951) writeReg(reg, val uint16) error { return e.cmd(itRegWrite, reg, val) }

func (e *IT8951) getVCOM() (uint16, error) {
	if err := e.cmd(itVCOM, 0x0000); err != nil {
		return 0, err
	}
	v, err := e.bus.Read(1)
	if err != nil {
		return 0, err
	}
	return v[0], nil
}

// waitDisplayReady polls LUTAFSR until all LUT engines are idle (C polls without pause).
func (e *IT8951) waitDisplayReady() error {
	const step = 10 * time.Millisecond
	for waited := time.Duration(0); ; waited += step {
		v, err := e.readReg(itRegLUTAFSR)
		if err != nil {
			return err
		}
		if v == 0 {
			return nil
		}
		if waited >= e.ReadyTimeout {
			return errors.New("epd: IT8951 display engine stays busy")
		}
		e.sleep(step)
	}
}

// init is EPD_IT8951_Init: reset, run, read device info, enable packed write, set VCOM.
func (e *IT8951) init() (itDevInfo, error) {
	var d itDevInfo
	if err := e.bus.Open(); err != nil {
		return d, err
	}
	for _, s := range []struct {
		high bool
		wait time.Duration
	}{{true, 200 * time.Millisecond}, {false, 10 * time.Millisecond}, {true, 200 * time.Millisecond}} {
		if err := e.bus.SetReset(s.high); err != nil {
			return d, err
		}
		e.sleep(s.wait)
	}
	if err := e.cmd(itSysRun); err != nil {
		return d, err
	}
	if err := e.cmd(itGetDevInfo); err != nil {
		return d, err
	}
	words, err := e.bus.Read(20)
	if err != nil {
		return d, err
	}
	d.panelW, d.panelH = int(words[0]), int(words[1])
	d.addr = uint32(words[2]) | uint32(words[3])<<16
	var lut []byte // UWORD array read as a C string on a little-endian CPU
	for _, w := range words[12:20] {
		lut = append(lut, byte(w), byte(w>>8))
	}
	if i := strings.IndexByte(string(lut), 0); i >= 0 {
		lut = lut[:i]
	}
	d.lut = string(lut)
	if d.panelW == 0 || d.panelH == 0 {
		return d, fmt.Errorf("epd: IT8951 reports panel %dx%d, check wiring", d.panelW, d.panelH)
	}
	if err := e.writeReg(itRegI80CPCR, 0x0001); err != nil { // enable pack write
		return d, err
	}
	cur, err := e.getVCOM()
	if err != nil {
		return d, err
	}
	if cur != e.vcom {
		if err := e.cmd(itVCOM, 0x0001, e.vcom); err != nil {
			return d, err
		}
		if _, err := e.getVCOM(); err != nil { // main.c reads it back for its Debug line
			return d, err
		}
	}
	return d, nil
}

// load4bpp is EPD_IT8951_HostAreaPackedPixelWrite_4bp: buf is the Paint buffer
// (row stride ceil(w/2)); words are taken contiguously and sent high byte first.
func (e *IT8951) load4bpp(buf []byte, w, h int, addr uint32) error {
	if err := e.writeReg(itRegLISAR+2, uint16(addr>>16)); err != nil {
		return err
	}
	if err := e.writeReg(itRegLISAR, uint16(addr)); err != nil {
		return err
	}
	if err := e.cmd(itLoadArea, itLoadArgs4bpp, 0, 0, uint16(w), uint16(h)); err != nil {
		return err
	}
	n := (w * 4 / 8) / 2 * h
	wire := make([]byte, 2*n)
	for k := 0; k < n; k++ { // UWORD from little-endian memory, written >>8 then low byte
		wire[2*k], wire[2*k+1] = buf[2*k+1], buf[2*k]
	}
	if err := e.bus.Write(wire); err != nil {
		return err
	}
	return e.cmd(itLoadEnd)
}

// clearRefresh is EPD_IT8951_Clear_Refresh: full-panel white, shown in mode.
func (e *IT8951) clearRefresh(d itDevInfo, mode uint16) error {
	buf := make([]byte, (d.panelW*4+7)/8*d.panelH)
	for i := range buf {
		buf[i] = 0xFF
	}
	if err := e.waitDisplayReady(); err != nil {
		return err
	}
	if err := e.load4bpp(buf, d.panelW, d.panelH, d.addr); err != nil {
		return err
	}
	return e.cmd(itDisplayArea, 0, 0, uint16(d.panelW), uint16(d.panelH), mode)
}

// showGrey is Display_BMP_Example(..., BitsPerPixel_4, path) with the BMP already
// converted to grey levels (Color & 0xF0 values, one per pixel, e.w x e.h).
func (e *IT8951) showGrey(d itDevInfo, grey []byte) error {
	width := d.panelW
	if d.lut == "M641" { // Four_Byte_Align (6" panels)
		width -= width % 32
	}
	height := d.panelH
	stride := (width*4 + 7) / 8
	buf := make([]byte, stride*height)
	for i := range buf {
		buf[i] = 0xFF // Paint_Clear(WHITE)
	}
	for y := 0; y < e.h; y++ { // GUI_ReadBmp → DrawMatrix → Paint_SetPixel
		for x := 0; x < e.w; x++ {
			if x > width || y > height { // Paint_SetPixel's bounds check (sic: >)
				continue
			}
			addr := x*4/8 + y*stride
			if addr >= len(buf) {
				continue // the C code would write past its buffer here
			}
			shift := 7 - (x*4+3)%8
			buf[addr] &^= 0xF0 >> shift
			buf[addr] |= grey[y*e.w+x] >> shift
		}
	}
	if err := e.waitDisplayReady(); err != nil {
		return err
	}
	if err := e.load4bpp(buf, width, height, d.addr); err != nil {
		return err
	}
	if err := e.cmd(itDisplayBuf, 0, 0, uint16(width), uint16(height), itModeGC16, uint16(d.addr), uint16(d.addr>>16)); err != nil {
		return err
	}
	e.sleep(5 * time.Second)
	return nil
}

// session runs main.c once for a converted image.
func (e *IT8951) session(grey []byte) error {
	d, err := e.init()
	if err != nil {
		e.bus.Close()
		return err
	}
	slog.Debug("epd: IT8951", "panel", fmt.Sprintf("%dx%d", d.panelW, d.panelH), "lut", d.lut, "vcom_mV", e.vcom)
	if err := func() error {
		if err := e.clearRefresh(d, itModeInit); err != nil {
			return err
		}
		// main.c calls Display_BMP_Example twice with the same file (SHOW_BMP block, then
		// again in the else branch); kept so the panel behaves like upstream.
		for range 2 {
			if err := e.showGrey(d, grey); err != nil {
				return err
			}
		}
		return e.cmd(itSleep)
	}(); err != nil {
		e.bus.Close()
		return err
	}
	e.sleep(5 * time.Second)
	return e.bus.Close()
}

// greyLevels converts the frame like upstream: main.py merges the colour band into the
// black image as pure red (Images.merge, no optimisation for parallel models), the C
// program turns (R,G,B) into (R*299+G*587+B*114+500)/1000 and Paint keeps the top nibble:
// white 0xF0, black 0x00, red 76 → 0x40 (dark grey).
func (e *IT8951) greyLevels(f render.Frame) []byte {
	stride := (f.W + 7) / 8
	out := make([]byte, f.W*f.H)
	for y := 0; y < f.H; y++ {
		for x := 0; x < f.W; x++ {
			sx := x
			if e.mirror {
				sx = f.W - 1 - x
			}
			idx, bit := y*stride+sx/8, byte(0x80>>(sx%8))
			g := byte(0xF0)
			if f.Black[idx]&bit == 0 {
				g = 0x00
			}
			if f.Red[idx]&bit != 0 {
				g = 0x40
			}
			out[y*f.W+x] = g
		}
	}
	return out
}

// Show displays the frame (one run of upstream's epd program).
func (e *IT8951) Show(f render.Frame) error {
	if f.W != e.w || f.H != e.h {
		return fmt.Errorf("epd: frame %dx%d, panel is %dx%d", f.W, f.H, e.w, e.h)
	}
	return e.session(e.greyLevels(f))
}

// Clear mirrors display.py calibrate() for black/white models: per cycle a black and a
// white image, each a full run of the epd program.
func (e *IT8951) Clear(cycles int) error {
	black := make([]byte, e.w*e.h)
	white := make([]byte, e.w*e.h)
	for i := range white {
		white[i] = 0xF0
	}
	for i := 0; i < cycles; i++ {
		for _, g := range [][]byte{black, white} {
			if err := e.session(g); err != nil {
				return err
			}
		}
	}
	return nil
}
