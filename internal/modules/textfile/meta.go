package textfile

import "inkycal/internal/modules"

var Meta = modules.Meta{
	Name: "TextToDisplay", Title: "Text", Description: "A local or online text file, with simple headings and lists.",
	MinW: 100, MinH: 40,
	Fields: []modules.Field{
		{Key: "filepath", Label: "File or URL", Kind: modules.KindText, Required: true},
	},
}
