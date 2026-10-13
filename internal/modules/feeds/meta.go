package feeds

import "inkycal/internal/modules"

var Meta = modules.Meta{
	Name: "Feeds", Title: "News feeds", Description: "Headlines and summaries from RSS or Atom feeds.", Group: "info", Icon: "📰",
	MinW: 120, MinH: 40,
	Fields: []modules.Field{
		{Key: "feed_urls", Label: "Feed URLs", Kind: modules.KindURLList, Required: true},
		{Key: "shuffle_feeds", Label: "Shuffle", Kind: modules.KindBool, Default: true},
	},
}
