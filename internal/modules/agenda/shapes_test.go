package agenda

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"inkycal/internal/config"
	"inkycal/internal/fetch"
	"inkycal/internal/modules"
	"inkycal/internal/modules/modulestest"
	"inkycal/internal/render"
)

func TestShapes(t *testing.T) {
	loc := berlin(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.Write([]byte(ics)) }))
	t.Cleanup(srv.Close)
	modulestest.Shapes(t, Meta, func(c config.Common) (*render.Canvas, error) {
		m, err := New(c, json.RawMessage(`{"ical_urls":"`+srv.URL+`"}`))
		if err != nil {
			return nil, err
		}
		return m.Generate(context.Background(), &modules.Env{Now: time.Date(2026, 10, 10, 8, 0, 0, 0, loc), Loc: loc, Fetch: fetch.New("")})
	})
}
