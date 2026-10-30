package image

import (
	"testing"

	"inkycal/internal/modules/modulestest"
)

func TestMeta(t *testing.T) {
	modulestest.CheckMeta(t, ImageDef)
	modulestest.CheckMeta(t, SlideshowDef)
	modulestest.CheckMeta(t, ServerDef)
}
