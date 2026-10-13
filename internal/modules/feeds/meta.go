package feeds

import "inkycal/internal/modules"

var Meta = modules.Meta{
	Name: "Feeds", Title: "News feeds", Description: "Headlines and summaries from RSS or Atom feeds.",
	MinW: 50, MinH: 20,
	Fields: []modules.Field{
		{Key: "feed_urls", Label: "Feed URLs", Kind: modules.KindURLList, Required: true},
		{Key: "shuffle_feeds", Label: "Shuffle", Kind: modules.KindBool, Default: true},
	},
}
