package todoist

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"testing"
	"time"

	"inkycal/internal/fetch"
	"inkycal/internal/modules"
	"inkycal/internal/modules/modulestest"
)

func TestNewRequiresAPIKey(t *testing.T) {
	common := modulestest.Common(t, `{"size":[480,200],"padding_x":10,"padding_y":10,"fontsize":14}`)
	if _, err := New(common, json.RawMessage(`{"project_filter":""}`)); err == nil {
		t.Error("missing api_key accepted")
	}
}

func TestGenerate(t *testing.T) {
	loc := berlin(t)
	now := time.Date(2026, 10, 10, 9, 0, 0, 0, loc)
	serveTodoist(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/projects":
			io.WriteString(w, `{"results":[{"id":"p1","name":"Inbox"}],"next_cursor":null}`)
		case "/tasks":
			io.WriteString(w, `{"results":[{"content":"Milch kaufen","project_id":"p1","priority":4,"due":{"date":"2026-10-09"}}],"next_cursor":null}`)
		default:
			http.NotFound(w, r)
		}
	})
	common := modulestest.Common(t, `{"size":[480,200],"padding_x":10,"padding_y":10,"fontsize":14,"language":"en"}`)
	m, err := New(common, json.RawMessage(`{"api_key":"tok","project_filter":"Inbox","show_priority":true}`))
	if err != nil {
		t.Fatal(err)
	}
	env := &modules.Env{Now: now, Loc: loc, Fetch: fetch.New("")}

	c, err := m.Generate(context.Background(), env)
	if err != nil {
		t.Fatal(err)
	}
	if c.W != 460 || c.H != 180 {
		t.Fatalf("canvas = %dx%d", c.W, c.H)
	}
	colour, black := false, false
	for y := 0; y < c.H; y++ {
		for x := 0; x < c.W; x++ {
			b, col := c.InkAt(x, y)
			black, colour = black || b, colour || col
		}
	}
	if !colour || !black {
		t.Errorf("ink black=%v colour=%v, want both (project name red, task black)", black, colour)
	}
}
