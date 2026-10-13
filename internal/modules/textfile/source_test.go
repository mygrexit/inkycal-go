package textfile

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"inkycal/internal/config"
	"inkycal/internal/modules"
	"inkycal/internal/modules/modulestest"
	"inkycal/internal/render"
)

func common(t *testing.T) config.Common {
	t.Helper()
	c, err := config.ParseCommon(json.RawMessage(`{"size":[600,300],"padding_x":10,"padding_y":10,"fontsize":20,"language":"de"}`))
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func inked(c *render.Canvas) bool {
	for y := 0; y < c.H; y++ {
		for x := 0; x < c.W; x++ {
			if b, _ := c.InkAt(x, y); b {
				return true
			}
		}
	}
	return false
}

func TestTypedText(t *testing.T) {
	for _, raw := range []string{
		`{"source":"text","text":"# Einkauf\n- Milch"}`,
		`{"text":"Hallo Welt"}`, // source defaults to text when text is set
	} {
		m, err := New(common(t), json.RawMessage(raw))
		if err != nil {
			t.Fatalf("%s: %v", raw, err)
		}
		c, err := m.Generate(context.Background(), &modules.Env{Now: time.Now()})
		if err != nil || !inked(c) {
			t.Errorf("%s: err %v inked %v", raw, err, c != nil && inked(c))
		}
	}
	if _, err := New(common(t), json.RawMessage(`{"source":"text"}`)); err == nil || !strings.Contains(err.Error(), "text is missing") {
		t.Errorf("empty text: %v", err)
	}
	if _, err := New(common(t), json.RawMessage(`{"source":"url"}`)); err == nil || !strings.Contains(err.Error(), "filepath is missing") {
		t.Errorf("url without address: %v", err)
	}
}

func TestOnePerDay(t *testing.T) {
	text := "# Sprüche\n\nCarpe diem\nHakuna matata\n- Keep calm"
	day := func(d int) time.Time { return time.Date(2026, 10, d, 7, 0, 0, 0, time.UTC) }
	a, b := lineOfDay(text, day(13)), lineOfDay(text, day(13).Add(10*time.Hour))
	if a != b {
		t.Errorf("not stable within a day: %q vs %q", a, b)
	}
	seen := map[string]bool{}
	for d := 13; d < 16; d++ {
		seen[lineOfDay(text, day(d))] = true
	}
	if len(seen) != 3 || seen["Sprüche"] || seen[""] {
		t.Errorf("three days, three lines without the heading: %v", seen)
	}
	if got := lineOfDay("# Nur Überschrift", day(13)); got != "# Nur Überschrift" {
		t.Errorf("no candidates: %q", got)
	}
}

func TestShapesOnePerDay(t *testing.T) {
	modulestest.Shapes(t, Meta, func(c config.Common) (*render.Canvas, error) {
		m, err := New(c, json.RawMessage(`{"text":"Was du heute kannst besorgen, das verschiebe nicht auf morgen.","one_per_day":true}`))
		if err != nil {
			return nil, err
		}
		return m.Generate(context.Background(), &modules.Env{Now: time.Now()})
	})
}

// A quote of the day fills its tile: in a large tile a short line is set large, not at
// the base text size.
func TestOneLineFillsTheTile(t *testing.T) {
	c := render.NewCanvas(400, 500, render.FontRegular, 12)
	drawLine(c, "Be kind.", 12)
	top, bottom, left, right := c.H, -1, c.W, -1
	for y := 0; y < c.H; y++ {
		for x := 0; x < c.W; x++ {
			if b, _ := c.InkAt(x, y); b {
				top, bottom, left, right = min(top, y), max(bottom, y), min(left, x), max(right, x)
			}
		}
	}
	if bottom-top < 40 || right-left < 200 {
		t.Errorf("quote ink %dx%d px in a 400x500 tile — too small", right-left, bottom-top)
	}
	if right >= c.W || bottom >= c.H {
		t.Error("quote spills out of the tile")
	}
}
