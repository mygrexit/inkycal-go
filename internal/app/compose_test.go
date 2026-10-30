package app

import (
	"context"
	"encoding/json"
	"inkycal/internal/problem"
	"testing"
	"time"
)

func TestComposeWithoutCycle(t *testing.T) {
	o, d, p, _ := testOptions(t, "Dot", "Fail")
	got, err := Compose(context.Background(), o, o.Now())
	if err != nil {
		t.Fatal(err)
	}
	if w, h := d.Size(); got.Canvas.W != h || got.Canvas.H != w {
		t.Errorf("canvas %dx%d for panel %dx%d", got.Canvas.W, got.Canvas.H, w, h)
	}
	if len(got.Modules) != 2 || got.Modules[0].Err != nil || got.Modules[1].Err == nil {
		t.Errorf("statuses %+v", got.Modules)
	}
	// Compose must not touch the display, the alarm or the state.
	if d.shows != 0 || len(p.alarms) != 0 {
		t.Errorf("side effects: shows=%d alarms=%v", d.shows, p.alarms)
	}
	if got.Took <= 0 || got.Took > time.Minute {
		t.Errorf("took %v", got.Took)
	}
}

// Compose takes settings leniently whoever built them: a broken style in settings made
// in memory fails its tile, not the page.
func TestComposeIsLenientForSettingsInMemory(t *testing.T) {
	o, _, _, _ := testOptions(t, "Dot", "Dot")
	o.Settings.Modules[1].Config = json.RawMessage(`{"size":[480,200],"padding_x":10,"padding_y":10,"style":{"scale":3}}`)
	got, err := Compose(context.Background(), o, o.Now())
	if err != nil {
		t.Fatal(err)
	}
	if got.Modules[0].Err != nil || problem.Classify(got.Modules[1].Err) != problem.Style {
		t.Errorf("statuses %+v", got.Modules)
	}
}
