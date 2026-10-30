package greek

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"inkycal/internal/modules"
	"inkycal/internal/modules/modulestest"
)

// BenchmarkGenerateTall lays out the word on a tall tile, where the fitting loops run longest.
func BenchmarkGenerateTall(b *testing.B) {
	common := modulestest.Common(&testing.T{}, `{"size":[480,480],"padding_x":10,"padding_y":10,"fontsize":14,"language":"de"}`)
	m, err := New(common, json.RawMessage(`{}`))
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
