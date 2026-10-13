// Package stocks ports inkycal_stocks.py: price, day change and a one-month
// sparkline per ticker, from Yahoo Finance's chart endpoint.
package stocks

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"

	"inkycal/internal/config"
	"inkycal/internal/modules"
	"inkycal/internal/render"
)

type Module struct {
	common  config.Common
	tickers []string
}

// rawConfig: upstream's only module key. "TSLA, U, EURUSD=X" or a list.
type rawConfig struct {
	Tickers config.StringList `json:"tickers"`
}

func New(c config.Common, raw json.RawMessage) (modules.Module, error) {
	var rc rawConfig
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &rc); err != nil {
			return nil, fmt.Errorf("stocks: config: %w", err)
		}
	}
	if len(rc.Tickers) == 0 {
		return nil, errors.New("stocks: tickers is missing")
	}
	if err := modules.CheckMinSize(c, Meta); err != nil {
		return nil, err
	}
	return &Module{common: c, tickers: []string(rc.Tickers)}, nil
}

func (m *Module) Generate(ctx context.Context, env *modules.Env) (*render.Canvas, error) {
	results := fetchAll(ctx, env.Fetch, m.tickers)
	if err := allFailed(results); err != nil {
		return nil, err
	}
	rows := make([]row, len(results))
	for i, r := range results {
		if r.Err != nil {
			slog.Warn("stocks: ticker without data", "ticker", m.tickers[i], "err", r.Err)
		}
		env.Stale = env.Stale || r.Stale
		rows[i] = makeRow(m.tickers[i], r, m.common.Language)
	}
	w, h := m.common.InnerSize()
	c := render.NewCanvas(w, h, render.FontDefault, int(m.common.FontSize))
	drawRows(c, rows, int(m.common.FontSize), m.common.Language)
	return c, nil
}
