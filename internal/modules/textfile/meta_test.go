package textfile

import (
	"testing"

	"inkycal/internal/modules/modulestest"
)

// Upstream settings have no "source": the device takes it from "text" or "filepath".
func TestMeta(t *testing.T) { modulestest.CheckMeta(t, Def, "source") }
