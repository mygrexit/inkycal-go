package epd

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"inkycal/internal/render"
)

// traceBus logs what a driver sends in the format of tools/epdtrace/trace.py:
// consecutive data writes are merged, BUSY reads alternate 1, 0, 1, 0 …
type traceBus struct {
	ops  []string
	data []byte
	busy int
}

func (b *traceBus) flush() {
	if b.data == nil {
		return
	}
	if len(b.data) > 32 {
		sum := sha256.Sum256(b.data)
		b.ops = append(b.ops, fmt.Sprintf("data %d sha256:%s", len(b.data), hex.EncodeToString(sum[:])[:16]))
	} else {
		b.ops = append(b.ops, "data "+hex.EncodeToString(b.data))
	}
	b.data = nil
}

func (b *traceBus) add(op string) { b.flush(); b.ops = append(b.ops, op) }

func (b *traceBus) Open() error  { b.add("open"); return nil }
func (b *traceBus) Close() error { b.add("close"); return nil }
func (b *traceBus) SetReset(high bool) error {
	if high {
		b.add("rst 01")
	} else {
		b.add("rst 00")
	}
	return nil
}
func (b *traceBus) Command(c byte) error { b.add(fmt.Sprintf("cmd %02x", c)); return nil }
func (b *traceBus) Data(p ...byte) error {
	if b.data == nil {
		b.data = []byte{}
	}
	b.data = append(b.data, p...)
	return nil
}
func (b *traceBus) BusyLevel() (int, error) { b.busy ^= 1; return b.busy, nil }

// patternFrame is trace.py's test frame: the same formula, packed like render.Pack.
func patternFrame(w, h int) render.Frame {
	c := render.NewCanvas(w, h, render.FontDefault, 12)
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			switch {
			case (x*7+y*3)%11 < 3:
				c.Set(x, y, render.InkBlack)
			case (x+2*y)%13 == 0:
				c.Set(x, y, render.InkColour)
			}
		}
	}
	f, err := render.Pack(c, w, h, 0)
	if err != nil {
		panic(err)
	}
	return f
}

// traced lists the single-controller drivers checked against upstream traces;
// driver test files add theirs in init(). Golden files: tools/epdtrace/regen.sh.
var traced = map[string]func(Bus, func(time.Duration)) Display{
	"epd_7_in_5_v2_colour": func(b Bus, s func(time.Duration)) Display { return NewEPD7in5bV2(b, s) },
}

func TestDriversMatchUpstreamTraces(t *testing.T) {
	for model, newDriver := range traced {
		t.Run(model, func(t *testing.T) {
			want, err := os.ReadFile(filepath.Join("testdata", "trace", model+".txt"))
			if err != nil {
				t.Fatalf("golden trace missing (run tools/epdtrace/regen.sh): %v", err)
			}
			size, ok := Models[model]
			if !ok {
				t.Fatalf("%s not in Models", model)
			}
			bus := &traceBus{}
			if err := newDriver(bus, func(time.Duration) {}).Show(patternFrame(size[0], size[1])); err != nil {
				t.Fatal(err)
			}
			bus.flush()
			got := strings.Join(bus.ops, "\n") + "\n"
			if got != string(want) {
				t.Errorf("%s differs from upstream:\n%s", model, diffLines(string(want), got))
			}
		})
	}
}

// diffLines shows the first differing line with some context.
func diffLines(want, got string) string {
	w, g := strings.Split(want, "\n"), strings.Split(got, "\n")
	for i := 0; i < max(len(w), len(g)); i++ {
		var a, b string
		if i < len(w) {
			a = w[i]
		}
		if i < len(g) {
			b = g[i]
		}
		if a != b {
			return fmt.Sprintf("line %d: upstream %q, go %q", i+1, a, b)
		}
	}
	return "lengths differ"
}
