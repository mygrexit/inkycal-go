package textfile

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"inkycal/internal/modules"
	"inkycal/internal/modules/modulestest"
	"inkycal/internal/render"
)

// BenchmarkLineOfDayTall sets a quote of the day on a tall tile, where the size search runs longest.
func BenchmarkLineOfDayTall(b *testing.B) {
	common := modulestest.Common(&testing.T{}, `{"size":[480,600],"padding_x":10,"padding_y":10,"fontsize":24,"language":"de"}`)
	m, err := Def.New(common, json.RawMessage(`{"text":"Was du heute kannst besorgen, das verschiebe nicht auf morgen, sagt man, und doch liegt so vieles noch herum.","one_per_day":true}`))
	if err != nil {
		b.Fatal(err)
	}
	env := &modules.Env{Now: time.Date(2026, 10, 30, 7, 0, 0, 0, time.UTC), Loc: time.UTC}
	for b.Loop() {
		if _, err := m.Generate(context.Background(), env); err != nil {
			b.Fatal(err)
		}
	}
}

// lineSize must pick what the former scan from the largest size down picked, so the
// panel looks exactly as before.
func TestLineSizeMatchesScan(t *testing.T) {
	texts := []string{
		"Carpe diem.",
		"Was du heute kannst besorgen, das verschiebe nicht auf morgen.",
		"Was du heute kannst besorgen, das verschiebe nicht auf morgen, sagt man, und doch liegt so vieles noch herum.",
		"Donaudampfschifffahrtsgesellschaftskapitänsmütze",
		"A B C D E F G H I J K L M N O P Q R S T U V W X Y Z",
	}
	for _, text := range texts {
		for _, w := range []int{120, 200, 300, 480, 640, 800} {
			for _, h := range []int{40, 60, 100, 200, 400, 600} {
				for _, body := range []int{8, 12, 24} {
					c := render.NewCanvas(w, h, render.FontRegular, body)
					floor := min(body, 10)
					top := max(floor, min(c.H/2, c.W/3))
					want := floor
					for size := top; size >= floor; size-- {
						if _, _, ok := lineLayout(c, text, render.Style{Font: render.FontRegular, Size: size}); ok {
							want = size
							break
						}
					}
					if got := lineSize(c, text, floor, top); got != want {
						t.Errorf("%dx%d body %d %q: size %d, scan picked %d", w, h, body, text, got, want)
					}
				}
			}
		}
	}
}
