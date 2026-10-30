package app

import (
	"context"
	"encoding/json"
	"sync"
	"testing"

	"inkycal/internal/config"
	"inkycal/internal/layout"
	"inkycal/internal/modules"
	"inkycal/internal/render"
)

// wordModule writes a word in black and a dot in colour on a canvas from env.Canvas.
type wordModule struct{ c config.Common }

func (m wordModule) Generate(_ context.Context, env *modules.Env) (*render.Canvas, error) {
	w, h := m.c.InnerSize()
	cv := env.Canvas(w, h, render.FontDefault, int(m.c.FontSize))
	cv.Text(4, h/2, "Hamburg", render.Style{Font: render.FontRegular, Size: int(m.c.FontSize)}, render.InkBlack)
	cv.FillCircle(w-20, h/2, 5, render.InkColour)
	return cv, nil
}

func styledOptions(t *testing.T, theme string, styles ...string) *Options {
	t.Helper()
	o, _, _, _ := testOptions(t)
	o.Settings.Grid = &layout.Grid{Columns: 1, Rows: 2}
	o.Settings.InfoSection = false
	if theme != "" {
		o.Settings.Theme = &config.Style{}
		if err := json.Unmarshal([]byte(theme), o.Settings.Theme); err != nil {
			t.Fatal(err)
		}
	}
	o.Registry = map[string]modules.Factory{"Word": func(c config.Common, _ json.RawMessage) (modules.Module, error) { return wordModule{c}, nil }}
	for i, st := range styles {
		cfg := `{"cell":[0,` + string(rune('0'+i)) + `,1,1],"padding_x":10,"padding_y":10,"fontsize":24`
		if st != "" {
			cfg += `,"style":` + st
		}
		o.Settings.Modules = append(o.Settings.Modules, config.ModuleEntry{Position: i + 1, Name: "Word", Config: json.RawMessage(cfg + "}")})
	}
	return o
}

func compose(t *testing.T, o *Options) *render.Canvas {
	t.Helper()
	got, err := Compose(context.Background(), o.composeOptions(), o.Now())
	if err != nil {
		t.Fatal(err)
	}
	for i, m := range got.Modules {
		if m.Err != nil {
			t.Fatalf("module %d: %v", i+1, m.Err)
		}
	}
	return got.Canvas
}

// region counts black, white and colour pixels of a rectangle.
func region(c *render.Canvas, x0, y0, x1, y1 int) (black, white, colour int) {
	for y := y0; y < y1; y++ {
		for x := x0; x < x1; x++ {
			b, col := c.InkAt(x, y)
			switch {
			case col:
				colour++
			case b:
				black++
			default:
				white++
			}
		}
	}
	return
}

// page is 480×800 (fakeDisplay 800×480), two rows of 400 px.
func TestTileWithOwnColours(t *testing.T) {
	c := compose(t, styledOptions(t, "", "", `{"colors":{"paper":"black","ink":"white"}}`))
	b0, w0, _ := region(c, 0, 0, 480, 400)
	b1, w1, col1 := region(c, 0, 400, 480, 800)
	if w0 < b0 {
		t.Errorf("top tile should stay white paper: black %d white %d", b0, w0)
	}
	if b1 < 10*w1 || w1 == 0 {
		t.Errorf("bottom tile should be black with white text: black %d white %d", b1, w1)
	}
	if col1 == 0 {
		t.Error("the accent dot should keep its colour")
	}
}

func TestAccentOffRemovesColour(t *testing.T) {
	c := compose(t, styledOptions(t, `{"colors":{"accent":"black"}}`, "", ""))
	if _, _, col := region(c, 0, 0, 480, 800); col != 0 {
		t.Errorf("%d colour pixels with accent off", col)
	}
}

func TestPageFontAndModuleOptOut(t *testing.T) {
	plain := compose(t, styledOptions(t, "", "", ""))
	serif := compose(t, styledOptions(t, `{"font":"serif"}`, "", `{"font":"sans"}`))
	if same(plain, serif, 0, 400) {
		t.Error("page serif theme did not change the first tile")
	}
	if !same(plain, serif, 400, 800) {
		t.Error("module font sans should look like the default")
	}
}

func same(a, b *render.Canvas, y0, y1 int) bool {
	for y := y0; y < y1; y++ {
		for x := 0; x < a.W; x++ {
			ab, ac := a.InkAt(x, y)
			bb, bc := b.InkAt(x, y)
			if ab != bb || ac != bc {
				return false
			}
		}
	}
	return true
}

func TestScaleMultipliesTheFontSize(t *testing.T) {
	var (
		mu    sync.Mutex
		sizes []int
	)
	o := styledOptions(t, `{"scale":1.5}`, "", `{"scale":0.5}`)
	o.Registry["Word"] = func(c config.Common, _ json.RawMessage) (modules.Module, error) {
		mu.Lock()
		sizes = append(sizes, int(c.FontSize))
		mu.Unlock()
		return wordModule{c}, nil
	}
	compose(t, o)
	if len(sizes) != 2 || sizes[0]+sizes[1] != 36+12 {
		t.Errorf("sizes %v, want 36 and 12", sizes)
	}
}

func TestNoStyleKeysRenderAsBefore(t *testing.T) {
	a := compose(t, styledOptions(t, "", "", ""))
	b := compose(t, styledOptions(t, `{}`, `{}`, `{"colors":{"paper":"white","ink":"black","accent":"colour"}}`))
	if !same(a, b, 0, 800) {
		t.Error("empty or default styles changed the page")
	}
}
