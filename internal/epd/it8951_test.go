package epd

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"inkycal/internal/render"
)

// fakeIT8951 logs every transaction. Each Write is one line (data > 32 bytes as length +
// sha256 prefix). LUTAFSR reads alternate busy/idle like traceBus's BUSY.
type fakeIT8951 struct {
	ops     []string
	panelW  int
	panelH  int
	lut     string
	vcomReg uint16
	lastCmd uint16
	lastArg []uint16
	lutBusy uint16
	failCmd uint16
	closed  int
	setVCOM []uint16
}

func (b *fakeIT8951) log(s string) { b.ops = append(b.ops, s) }
func (b *fakeIT8951) Open() error  { b.log("open"); return nil }
func (b *fakeIT8951) Close() error { b.log("close"); b.closed++; return nil }
func (b *fakeIT8951) SetReset(high bool) error {
	if high {
		b.log("rst 01")
	} else {
		b.log("rst 00")
	}
	return nil
}
func (b *fakeIT8951) Command(c uint16) error {
	if b.failCmd != 0 && c == b.failCmd {
		return errors.New("spi failed")
	}
	b.log(fmt.Sprintf("cmd %04x", c))
	b.lastCmd, b.lastArg = c, nil
	return nil
}
func (b *fakeIT8951) Write(p []byte) error {
	if len(p) > 32 {
		b.log(fmt.Sprintf("data %d sha256:%s", len(p), sha16(p)))
		return nil
	}
	b.log("data " + hex.EncodeToString(p))
	if len(p) == 2 {
		b.lastArg = append(b.lastArg, uint16(p[0])<<8|uint16(p[1]))
		if b.lastCmd == itVCOM && len(b.lastArg) == 2 && b.lastArg[0] == 1 {
			b.vcomReg = b.lastArg[1]
			b.setVCOM = append(b.setVCOM, b.lastArg[1])
		}
	}
	return nil
}
func (b *fakeIT8951) Read(n int) ([]uint16, error) {
	b.log(fmt.Sprintf("read %d", n))
	out := make([]uint16, n)
	switch {
	case b.lastCmd == itGetDevInfo:
		out[0], out[1], out[2], out[3] = uint16(b.panelW), uint16(b.panelH), 0x36E0, 0x0012
		lut := []byte(b.lut)
		for i := 0; i < len(lut); i += 2 {
			w := uint16(lut[i])
			if i+1 < len(lut) {
				w |= uint16(lut[i+1]) << 8
			}
			out[12+i/2] = w
		}
	case b.lastCmd == itVCOM:
		out[0] = b.vcomReg
	case b.lastCmd == itRegRead && len(b.lastArg) == 1 && b.lastArg[0] == itRegLUTAFSR:
		b.lutBusy ^= 1
		out[0] = b.lutBusy
	}
	return out, nil
}

func sha16(p []byte) string {
	s := sha256.Sum256(p)
	return hex.EncodeToString(s[:])[:16]
}

// expectedWire packs grey nibbles (one per pixel, row-major, w even) the way the C code
// ends up on the wire: Paint_SetPixel shifts by 7-(x*4+3)%8, i.e. even x lands in the
// low nibble and odd x in the high nibble; LD_IMG sends each
// little-endian UWORD high byte first, so every byte pair is swapped.
func expectedWire(nib func(x, y int) byte, w, h int) []byte {
	mem := make([]byte, 0, w/2*h)
	for y := 0; y < h; y++ {
		for x := 0; x < w; x += 2 {
			mem = append(mem, nib(x+1, y)<<4|nib(x, y))
		}
	}
	for i := 0; i+1 < len(mem); i += 2 {
		mem[i], mem[i+1] = mem[i+1], mem[i]
	}
	return mem
}

func frameNibble(f render.Frame, mirror bool) func(x, y int) byte {
	stride := (f.W + 7) / 8
	return func(x, y int) byte {
		if mirror {
			x = f.W - 1 - x
		}
		idx, bit := y*stride+x/8, byte(0x80>>(x%8))
		switch {
		case f.Red[idx]&bit != 0:
			return 0x4
		case f.Black[idx]&bit == 0:
			return 0x0
		}
		return 0xF
	}
}

// itSession is the transaction sequence of one run of upstream's `epd -<vcom> 0 file.bmp`
// (main.c), derived by hand from EPD_IT8951.c / example.c, for a panel reporting w x h,
// image memory 0x001236E0 and a VCOM register that differs from the requested one.
func itSession(w, h int, vcom uint16, white, image []byte) []string {
	hex4 := func(v int) string { return fmt.Sprintf("data %04x", v) }
	wait := []string{"cmd 0010", "data 1224", "read 1", "cmd 0010", "data 1224", "read 1"} // LUTAFSR busy, idle
	load := func(data []byte) []string {
		s := []string{"cmd 0011", "data 020a", "data 0012", "cmd 0011", "data 0208", "data 36e0",
			"cmd 0021", "data 0020", "data 0000", "data 0000", hex4(w), hex4(h)}
		s = append(s, fmt.Sprintf("data %d sha256:%s", len(data), sha16(data)))
		return append(s, "cmd 0022")
	}
	ops := []string{"open", "rst 01", "rst 00", "rst 01",
		"cmd 0001",            // SYS_RUN
		"cmd 0302", "read 20", // GET_DEV_INFO
		"cmd 0011", "data 0004", "data 0001", // I80CPCR = 1 (pack write)
		"cmd 0039", "data 0000", "read 1", // get VCOM
		"cmd 0039", "data 0001", hex4(int(vcom)), // set VCOM
		"cmd 0039", "data 0000", "read 1", // get VCOM (Debug line)
	}
	// EPD_IT8951_Clear_Refresh(INIT_Mode)
	ops = append(ops, wait...)
	ops = append(ops, load(white)...)
	ops = append(ops, "cmd 0034", "data 0000", "data 0000", hex4(w), hex4(h), "data 0000")
	// Display_BMP_Example, twice
	for range 2 {
		ops = append(ops, wait...)
		ops = append(ops, load(image)...)
		ops = append(ops, "cmd 0037", "data 0000", "data 0000", hex4(w), hex4(h), "data 0002", "data 36e0", "data 0012")
	}
	return append(ops, "cmd 0003", "close") // sleep, DEV_Module_Exit
}

func compareOps(t *testing.T, want, got []string) {
	t.Helper()
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("sequence differs from the C program: %s", diffLines(strings.Join(want, "\n"), strings.Join(got, "\n")))
	}
}

// TestIT8951Sequence checks the command/register sequence derived from the C source.
// NOT trace-verified against upstream: there is no way to run the C program here.
func TestIT8951Sequence(t *testing.T) {
	for _, tc := range []struct {
		model  string
		mirror bool
	}{{"9_in_7", false}, {"10_in_3", true}, {"7_in_8", false}} {
		t.Run(tc.model, func(t *testing.T) {
			size := Models[tc.model]
			w, h := size[0], size[1]
			bus := &fakeIT8951{panelW: w, panelH: h, lut: "M841_TFA5210", vcomReg: 1500}
			var slept time.Duration
			d, err := NewIT8951(bus, func(d time.Duration) { slept += d }, tc.model, "1.48")
			if err != nil {
				t.Fatal(err)
			}
			f := patternFrame(w, h)
			if err := d.Show(f); err != nil {
				t.Fatal(err)
			}
			white := bytes.Repeat([]byte{0xFF}, w/2*h)
			image := expectedWire(frameNibble(f, tc.mirror), w, h)
			compareOps(t, itSession(w, h, 1480, white, image), bus.ops)
			// reset 410 ms, two LUTAFSR polls per wait (3 waits), 5 s after each image, 5 s after sleep
			if want := 410*time.Millisecond + 3*10*time.Millisecond + 15*time.Second; slept != want {
				t.Errorf("slept %v, want %v", slept, want)
			}
		})
	}
}

func TestIT8951SkipsVCOMWhenEqual(t *testing.T) {
	bus := &fakeIT8951{panelW: 1200, panelH: 825, lut: "M841", vcomReg: 2000}
	d, _ := NewIT8951(bus, func(time.Duration) {}, "9_in_7", "")
	if err := d.Show(patternFrame(1200, 825)); err != nil {
		t.Fatal(err)
	}
	if len(bus.setVCOM) != 0 {
		t.Errorf("VCOM written although equal: %v", bus.setVCOM)
	}
}

func TestIT8951RedIsDarkGrey(t *testing.T) {
	d, _ := NewIT8951(&fakeIT8951{}, func(time.Duration) {}, "9_in_7", "")
	stride := (1200 + 7) / 8
	f := render.Frame{W: 1200, H: 825, Black: bytes.Repeat([]byte{0xFF}, stride*825), Red: make([]byte, stride*825)}
	f.Black[0] = 0x7F // (0,0) black
	f.Red[0] = 0x40   // (1,0) red
	g := d.greyLevels(f)
	if g[0] != 0x00 || g[1] != 0x40 || g[2] != 0xF0 {
		t.Errorf("grey = %#x %#x %#x, want 0x00 0x40 0xf0", g[0], g[1], g[2])
	}
}

func TestIT8951FourByteAlign(t *testing.T) {
	bus := &fakeIT8951{panelW: 1448, panelH: 1072, lut: "M641", vcomReg: 2000}
	d, _ := NewIT8951(bus, func(time.Duration) {}, "9_in_7", "") // 1200x825 BMP on a 6" HD panel
	if err := d.Show(patternFrame(1200, 825)); err != nil {
		t.Fatal(err)
	}
	// Clear_Refresh uses the panel width, Display_BMP_Example the 32-aligned one (1440 = 0x5a0).
	got := strings.Join(bus.ops, "\n")
	if !strings.Contains(got, "cmd 0034\ndata 0000\ndata 0000\ndata 05a8") || !strings.Contains(got, "cmd 0037\ndata 0000\ndata 0000\ndata 05a0") {
		t.Error("widths not as in the C code")
	}
}

func TestParseVCOM(t *testing.T) {
	for in, want := range map[string]uint16{"": 2000, "2.0": 2000, "1.48": 1480, "-1.48": 1480, "2": 2000, "2.51": 2510, "1.005": 1004} {
		got, err := parseVCOM(in)
		if err != nil || got != want {
			t.Errorf("parseVCOM(%q) = %d, %v; want %d", in, got, err, want)
		}
	}
	if _, err := parseVCOM("abc"); err == nil {
		t.Error("parseVCOM(abc) accepted")
	}
	if d, _ := NewIT8951(&fakeIT8951{}, nil, "7_in_8", "junk"); d.vcom != 2000 {
		t.Errorf("invalid vcom → %d, want default 2000", d.vcom)
	}
}

func TestIT8951RejectsWrongFrameAndModel(t *testing.T) {
	bus := &fakeIT8951{panelW: 1200, panelH: 825}
	d, _ := NewIT8951(bus, func(time.Duration) {}, "9_in_7", "")
	if err := d.Show(patternFrame(800, 480)); err == nil {
		t.Error("wrong frame size accepted")
	}
	if len(bus.ops) != 0 {
		t.Errorf("bus touched: %v", bus.ops)
	}
	if _, err := NewIT8951(bus, nil, "epd_7_in_5_v2_colour", ""); err == nil {
		t.Error("non-IT8951 model accepted")
	}
}

func TestIT8951ClosesOnError(t *testing.T) {
	for _, c := range []uint16{itSysRun, itLoadArea, itDisplayBuf, itSleep} {
		bus := &fakeIT8951{panelW: 1200, panelH: 825, failCmd: c}
		d, _ := NewIT8951(bus, func(time.Duration) {}, "9_in_7", "")
		if err := d.Show(patternFrame(1200, 825)); err == nil {
			t.Errorf("cmd %04x: error swallowed", c)
		}
		if bus.closed != 1 {
			t.Errorf("cmd %04x: closed %d times", c, bus.closed)
		}
	}
	bus := &fakeIT8951{} // panel reports 0x0
	d, _ := NewIT8951(bus, func(time.Duration) {}, "9_in_7", "")
	if err := d.Show(patternFrame(1200, 825)); err == nil || bus.closed != 1 {
		t.Errorf("0x0 panel: err %v, closed %d", err, bus.closed)
	}
}

func TestIT8951ClearCycles(t *testing.T) {
	bus := &fakeIT8951{panelW: 1200, panelH: 825, vcomReg: 2000}
	d, _ := NewIT8951(bus, func(time.Duration) {}, "9_in_7", "")
	if err := d.Clear(2); err != nil {
		t.Fatal(err)
	}
	black := "data 495000 sha256:" + sha16(make([]byte, 495000))
	white := "data 495000 sha256:" + sha16(bytes.Repeat([]byte{0xFF}, 495000))
	var images []string
	opens := 0
	for i, op := range bus.ops {
		if op == "open" {
			opens++
		}
		if op == "cmd 0037" { // image written just before: data, cmd 0022, cmd 0037
			images = append(images, bus.ops[i-2])
		}
	}
	want := []string{black, black, white, white, black, black, white, white}
	if opens != 4 || strings.Join(images, ",") != strings.Join(want, ",") {
		t.Errorf("opens %d, images %v", opens, images)
	}
}
