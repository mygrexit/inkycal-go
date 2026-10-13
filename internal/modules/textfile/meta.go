package textfile

import "inkycal/internal/modules"

var Meta = modules.Meta{
	Name: "TextToDisplay", Title: "Text & notes", Description: "Your own text, a web address or a file — or one line per day.", Group: "info", Icon: "📝",
	MinW: 100, MinH: 40,
	Fields: []modules.Field{
		{Key: "source", Label: "Where does the text come from?", Kind: modules.KindSelect, Options: []any{"text", "url", "file"},
			Labels: map[string]string{"text": "I write it here", "url": "A web address", "file": "A file on the Pi"}, Default: "text", Required: true},
		{Key: "text", Label: "Your text", Help: "# for a heading, - for a list", Kind: modules.KindMultiline, Required: true,
			ShowIf: &modules.Cond{Key: "source", Values: []any{"text"}}},
		{Key: "filepath", Label: "Address or file", Kind: modules.KindText, Required: true,
			ShowIf: &modules.Cond{Key: "source", Values: []any{"url", "file"}}},
		{Key: "one_per_day", Label: "Show one line per day (quote of the day)", Kind: modules.KindBool, Default: false},
	},
}
