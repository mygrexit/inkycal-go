package modulestest

import (
	"encoding/json"
	"testing"

	"inkycal/internal/config"
	"inkycal/internal/render"
)

// Common parses a module's JSON config into the shared keys; a parse error fails the test.
func Common(t *testing.T, raw string) config.Common {
	t.Helper()
	c, err := config.ParseCommon(json.RawMessage(raw))
	if err != nil {
		t.Fatal(err)
	}
	return c
}

// InkIn reports black and colour ink in the rectangle x0..x1-1, y0..y1-1 (clipped to the canvas).
func InkIn(c *render.Canvas, x0, y0, x1, y1 int) (black, colour bool) {
	for y := max(0, y0); y < min(c.H, y1); y++ {
		for x := max(0, x0); x < min(c.W, x1); x++ {
			b, col := c.InkAt(x, y)
			black, colour = black || b, colour || col
		}
	}
	return black, colour
}
