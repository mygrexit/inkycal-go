package stocks

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"inkycal/internal/config"
	"inkycal/internal/fetch"
	"inkycal/internal/modules"
	"inkycal/internal/modules/modulestest"
	"inkycal/internal/render"
)

func TestShapes(t *testing.T) {
	chartServer(t)
	modulestest.Shapes(t, Meta, func(c config.Common) (*render.Canvas, error) {
		m, err := New(c, json.RawMessage(`{"tickers":"AAPL, TSLA, ^GDAXI, SAP.DE, EURUSD=X"}`))
		if err != nil {
			return nil, err
		}
		return m.Generate(context.Background(), &modules.Env{Now: time.Now(), Loc: time.UTC, Fetch: fetch.New("")})
	})
}
