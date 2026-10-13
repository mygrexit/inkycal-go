package feeds

import (
	"testing"

	"inkycal/internal/modules/modulestest"
)

func TestMeta(t *testing.T) { modulestest.CheckMeta(t, Meta, New, rawConfig{}) }
