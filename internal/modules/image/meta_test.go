package image

import (
	"testing"

	"inkycal/internal/config"
	"inkycal/internal/modules/modulestest"
)

func TestMeta(t *testing.T) {
	modulestest.CheckMeta(t, ImageMeta, NewImage, rawLook{}, struct {
		Path string `json:"path"`
	}{})
	modulestest.CheckMeta(t, SlideshowMeta, NewSlideshow, rawLook{}, struct {
		Path     string         `json:"path"`
		Interval config.FlexInt `json:"interval"`
	}{})
	modulestest.CheckMeta(t, ServerMeta, NewServer, rawLook{}, struct {
		Path     string            `json:"path"`
		PathBody config.StringList `json:"path_body"`
	}{})
}
