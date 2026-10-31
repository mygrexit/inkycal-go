package fullweather

import (
	"testing"

	"inkycal/internal/modules/modulestest"
)

// The builder asks for a place to search; the view itself goes by the coordinates the
// search writes, so the device does not insist on the name.
func TestMeta(t *testing.T) { modulestest.CheckMeta(t, Def, "location") }
