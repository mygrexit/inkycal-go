package image

import "inkycal/internal/modules"

// lookFields are the picture keys the three modules share (rawLook).
func lookFields(ditherDefault bool) []modules.Field {
	return []modules.Field{
		{Key: "palette", Label: "Colours", Kind: modules.KindSelect, Options: []any{"bw", "bwr", "bwy"}, Default: "bwr"},
		{Key: "dither", Label: "Dither", Kind: modules.KindBool, Default: ditherDefault},
		{Key: "autoflip", Label: "Rotate to fit", Kind: modules.KindBool, Default: false},
		{Key: "orientation", Label: "Rotate towards", Kind: modules.KindSelect, Options: []any{"vertical", "horizontal"}},
		{Key: "max_width_percent", Label: "Max width %", Kind: modules.KindNumber, Default: 100},
		{Key: "max_height_percent", Label: "Max height %", Kind: modules.KindNumber, Default: 100},
	}
}

var ImageMeta = modules.Meta{
	Name: "Inkyimage", Title: "Picture", Description: "One picture from a file or URL.",
	MinW: 40, MinH: 40,
	Fields: append([]modules.Field{
		{Key: "path", Label: "File or URL", Kind: modules.KindText, Required: true},
	}, lookFields(true)...),
}

var SlideshowMeta = modules.Meta{
	Name: "Slideshow", Title: "Slideshow", Description: "Pictures from a folder, a new one per interval.",
	MinW: 40, MinH: 40,
	Fields: append([]modules.Field{
		{Key: "path", Label: "Folder", Kind: modules.KindText, Required: true},
		{Key: "interval", Label: "Minutes per picture", Kind: modules.KindNumber, Default: 1440},
	}, lookFields(true)...),
}

var ServerMeta = modules.Meta{
	Name: "Inkyserver", Title: "Picture from a server", Description: "A picture rendered by a web service; {width} and {height} are filled in.",
	MinW: 40, MinH: 40,
	Fields: append([]modules.Field{
		{Key: "path", Label: "URL", Kind: modules.KindURL, Required: true},
		{Key: "path_body", Label: "POST body", Help: "Empty sends a GET request", Kind: modules.KindTextList},
	}, lookFields(false)...),
}
