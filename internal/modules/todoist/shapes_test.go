package todoist

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
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
	serveTodoist(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/projects":
			io.WriteString(w, `{"results":[{"id":"p1","name":"Inbox"}],"next_cursor":null}`)
		case "/tasks":
			io.WriteString(w, `{"results":[
				{"content":"Kita-Anmeldung abgeben","project_id":"p1","priority":4,"due":{"date":"2026-10-09"}},
				{"content":"Milch und Brot","project_id":"p1","priority":1,"due":{"date":"2026-10-10"}},
				{"content":"Winterreifen aufziehen lassen","project_id":"p1","priority":1,"due":{"date":"2026-10-14"}},
				{"content":"Geschenk für Oma besorgen","project_id":"p1","priority":2},
				{"content":"Fahrradschloss reparieren","project_id":"p1","priority":1}],"next_cursor":null}`)
		default:
			http.NotFound(w, r)
		}
	})
	modulestest.Shapes(t, Meta, func(c config.Common) (*render.Canvas, error) {
		m, err := Def.New(c, json.RawMessage(`{"api_key":"tok"}`))
		if err != nil {
			return nil, err
		}
		return m.Generate(context.Background(), &modules.Env{Now: time.Date(2026, 10, 10, 9, 0, 0, 0, loc), Loc: loc, Fetch: fetch.New("")})
	})
}
