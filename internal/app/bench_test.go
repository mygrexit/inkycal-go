package app

import (
	"context"
	"testing"
	"time"

	"inkycal/internal/render"
)

// nullDisplay takes frames without drawing them, so a benchmark measures the cycle, not PNG encoding.
type nullDisplay struct{ w, h int }

func (d nullDisplay) Size() (int, int)      { return d.w, d.h }
func (nullDisplay) Show(render.Frame) error { return nil }
func (nullDisplay) Clear(int) error         { return nil }

// BenchmarkRunCycle is one wake of the kitchen dashboard without network or panel:
// settings, four modules against local fixtures, layout and packing.
// Compare runs with benchstat; see `make bench`.
func BenchmarkRunCycle(b *testing.B) {
	loc, err := time.LoadLocation("Europe/Berlin")
	if err != nil {
		b.Fatal(err)
	}
	now := time.Date(2026, 10, 13, 6, 5, 0, 0, loc)
	o := previewOptions(b, now, true, nullDisplay{1304, 984})
	b.ReportAllocs()
	for b.Loop() {
		if _, err := RunCycle(context.Background(), o); err != nil {
			b.Fatal(err)
		}
	}
}
