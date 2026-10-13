// Package modules defines what a dashboard module is.
package modules

import (
	"context"
	"encoding/json"
	"time"

	"inkycal/internal/config"
	"inkycal/internal/fetch"
	"inkycal/internal/render"
)

// Env is passed to every module run. Each module gets its own copy.
type Env struct {
	Now   time.Time // already in Loc
	Loc   *time.Location
	Fetch *fetch.Client
	Theme *render.Theme // the module's fonts; nil = the defaults
	Stale bool          // set by the module when it rendered cached data
}

// Canvas is a blank canvas drawing with the module's theme fonts.
func (e *Env) Canvas(w, h int, id render.FontID, size int) *render.Canvas {
	return render.NewThemedCanvas(w, h, id, size, e.Theme)
}

// Module renders one section of the dashboard.
type Module interface {
	// Generate returns a canvas of the module's inner size (config size minus padding).
	Generate(ctx context.Context, env *Env) (*render.Canvas, error)
}

// Factory builds a module from the shared keys and its raw JSON config.
type Factory func(c config.Common, raw json.RawMessage) (Module, error)
