package weather

import (
	"testing"

	"inkycal/internal/modules/modulestest"
)

// The builder asks for a place to search; Open-Meteo itself needs only the coordinates
// the search writes, so the device does not insist on the name.
func TestMeta(t *testing.T) { modulestest.CheckMeta(t, Def, "location") }
