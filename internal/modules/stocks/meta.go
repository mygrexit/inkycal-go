package stocks

import "inkycal/internal/modules"

var Meta = modules.Meta{
	Name: "Stocks", Title: "Stocks", Description: "Prices, day change and a one-month sparkline from Yahoo Finance.",
	MinW: 150, MinH: 20,
	Fields: []modules.Field{
		{Key: "tickers", Label: "Tickers", Help: "e.g. AAPL, ^GSPC, BTC-EUR", Kind: modules.KindTextList, Required: true},
	},
}
