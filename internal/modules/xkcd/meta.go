package xkcd

import "inkycal/internal/modules"

var Meta = modules.Meta{
	Name: "Xkcd", Title: "xkcd", Description: "The latest or a random xkcd comic.",
	MinW: 160, MinH: 80,
	Fields: []modules.Field{
		{Key: "mode", Label: "Comic", Kind: modules.KindSelect, Options: []any{"latest", "random"}, Default: "latest"},
		{Key: "palette", Label: "Colours", Kind: modules.KindSelect, Options: []any{"bw", "bwr", "bwy"}, Default: "bw"},
		{Key: "alt", Label: "Show the alt text", Kind: modules.KindSelect, Options: []any{"no", "yes"}, Default: "no"},
		{Key: "filter", Label: "Skip comics that don't fit", Kind: modules.KindSelect, Options: []any{"no", "yes"}, Default: "no"},
	},
}
