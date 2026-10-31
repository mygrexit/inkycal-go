package problem_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"inkycal/internal/config"
	"inkycal/internal/fetch"
	"inkycal/internal/ical"
	"inkycal/internal/modules"
	"inkycal/internal/modules/fullweather"
	"inkycal/internal/modules/textfile"
	"inkycal/internal/modules/todoist"
	"inkycal/internal/modules/weather"
	"inkycal/internal/problem"
)

// The kinds come from the real producers, not from hand-written messages: a producer
// that changes its wording or stops marking its error fails here.

func server(t *testing.T, h http.HandlerFunc) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	return srv
}

func client() *fetch.Client {
	c := fetch.New("")
	c.Attempts = 1
	return c
}

func status(code int, body string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) { http.Error(w, body, code) }
}

func common(t *testing.T, raw string) config.Common {
	t.Helper()
	c, err := config.ParseCommon(json.RawMessage(raw))
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func load(t *testing.T, body string) *config.Settings {
	t.Helper()
	p := filepath.Join(t.TempDir(), "settings.json")
	if err := os.WriteFile(p, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	s, err := config.Load(p)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func get(t *testing.T, h http.HandlerFunc) error {
	t.Helper()
	_, err := client().Get(context.Background(), server(t, h).URL+"/x?key=SECRET", nil)
	return err
}

func TestClassifyRealProducers(t *testing.T) {
	small := common(t, `{"size":[480,200],"padding_x":10,"padding_y":10}`)
	closed := httptest.NewServer(http.NotFoundHandler())
	closed.Close()
	const grid = `{"model":"m","modules":[{"position":1,"name":"A","config":{"cell":[0,0,12,4]}},`

	cases := []struct {
		name string
		err  func(t *testing.T) error
		want problem.Kind
	}{
		{"HTTP 401", func(t *testing.T) error { return get(t, status(401, `{"error":"Unauthorized"}`)) }, problem.Denied},
		{"HTTP 401 whose body says 'is missing'", func(t *testing.T) error { return get(t, status(401, "token is missing")) }, problem.Denied},
		{"HTTP 404", func(t *testing.T) error { return get(t, status(404, "nope")) }, problem.NotFound},
		{"HTTP 503", func(t *testing.T) error { return get(t, status(503, "down")) }, problem.Unavailable},
		{"HTTP 429", func(t *testing.T) error { return get(t, status(429, "slow down")) }, problem.Unavailable},
		{"connection refused", func(t *testing.T) error {
			_, err := client().Get(context.Background(), closed.URL, nil)
			return err
		}, problem.Offline},
		{"timeout", func(t *testing.T) error {
			srv := server(t, func(w http.ResponseWriter, r *http.Request) { <-r.Context().Done() })
			ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
			defer cancel()
			_, err := client().Get(ctx, srv.URL, nil)
			return err
		}, problem.Timeout},
		{"response is not JSON", func(t *testing.T) error {
			var v struct{}
			_, err := client().GetJSON(context.Background(), server(t, status(200, "<html>")).URL, nil, &v)
			return err
		}, problem.BadResponse},
		{"calendar address returns a web page", func(t *testing.T) error {
			srv := server(t, func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "text/html")
				w.Write([]byte("<!doctype html><html><body>Sign in</body></html>"))
			})
			now := time.Now()
			_, _, err := ical.Load(context.Background(), client(), []string{srv.URL}, nil, now, now.AddDate(0, 1, 0), time.UTC)
			return err
		}, problem.NotCalendar},
		{"Open-Meteo without a place", func(t *testing.T) error {
			_, err := weather.New(small, json.RawMessage(`{"provider":"open-meteo"}`))
			return err
		}, problem.Missing},
		{"OpenWeatherMap without a location", func(t *testing.T) error {
			_, err := weather.New(small, json.RawMessage(`{"api_key":"k"}`))
			return err
		}, problem.Missing},
		{"Todoist without a key", func(t *testing.T) error {
			_, err := todoist.New(small, json.RawMessage(`{}`))
			return err
		}, problem.Missing},
		{"setting of the wrong type", func(t *testing.T) error {
			_, err := weather.New(small, json.RawMessage(`{"units":5}`))
			return err
		}, problem.BadSetting},
		{"local file does not exist", func(t *testing.T) error {
			m, err := textfile.New(small, json.RawMessage(`{"filepath":"/no/such/file.txt"}`))
			if err != nil {
				return err
			}
			_, err = m.Generate(context.Background(), &modules.Env{Now: time.Now(), Loc: time.UTC})
			return err
		}, problem.NotFound},
		{"tile smaller than the module needs", func(t *testing.T) error {
			return modules.CheckMinSize(common(t, `{"size":[60,40],"padding_x":5,"padding_y":5}`), modules.Meta{Name: "Weather", MinW: 200, MinH: 100})
		}, problem.TooSmall},
		{"padding larger than the tile", func(t *testing.T) error {
			_, err := config.ParseCommon(json.RawMessage(`{"size":[40,40],"padding_x":30,"padding_y":30}`))
			return err
		}, problem.Padding},
		{"broken style (device)", func(t *testing.T) error {
			return load(t, grid+`{"position":2,"name":"B","config":{"cell":[0,4,12,4],"style":{"scale":3}}}]}`).Modules[1].Err
		}, problem.Style},
		{"broken style (builder)", func(t *testing.T) error {
			_, err := config.Parse([]byte(grid+`{"position":2,"name":"B","config":{"cell":[0,4,12,4],"style":{"scale":3}}}]}`), "settings")
			return err
		}, problem.Style},
		{"overlapping cell (device)", func(t *testing.T) error {
			return load(t, grid+`{"position":2,"name":"B","config":{"cell":[0,2,12,4]}}]}`).Modules[1].Err
		}, problem.Placement},
		{"missing cell (builder)", func(t *testing.T) error {
			_, err := config.Parse([]byte(grid+`{"position":2,"name":"B","config":{}}]}`), "settings")
			return err
		}, problem.Placement},
		{"two calendars: 401 first, a web page second", func(t *testing.T) error {
			denied := server(t, status(401, "Unauthorized"))
			page := server(t, func(w http.ResponseWriter, r *http.Request) { w.Write([]byte("<!doctype html><html></html>")) })
			now := time.Now()
			_, _, err := ical.Load(context.Background(), client(), []string{denied.URL, page.URL}, nil, now, now.AddDate(0, 1, 0), time.UTC)
			return err
		}, problem.Denied},
		{"calendar address returns plain text", func(t *testing.T) error {
			srv := server(t, func(w http.ResponseWriter, r *http.Request) { w.Write([]byte("hello")) })
			now := time.Now()
			_, _, err := ical.Load(context.Background(), client(), []string{srv.URL}, nil, now, now.AddDate(0, 1, 0), time.UTC)
			return err
		}, problem.BadResponse},
		{"Fullweather without a place", func(t *testing.T) error {
			_, err := fullweather.New(small, json.RawMessage(`{"api_key":"k"}`))
			return err
		}, problem.Missing},
		{"setting of a flexible type with a wrong value", func(t *testing.T) error {
			_, err := config.ParseCommon(json.RawMessage(`{"size":[480,200],"padding_x":"wide"}`))
			return err
		}, problem.BadSetting},
		{"settings file that is not JSON (builder)", func(t *testing.T) error {
			_, err := config.Parse([]byte(`{"model": `), "settings")
			return err
		}, problem.BadSetting},
		{"broken theme (builder)", func(t *testing.T) error {
			_, err := config.Parse([]byte(`{"model":"m","theme":{"font":"comic"},"modules":[]}`), "settings")
			return err
		}, problem.Style},
		{"page without a display model: no tile advice", func(t *testing.T) error {
			_, err := config.Parse([]byte(`{"modules":[]}`), "settings")
			return err
		}, problem.Unknown},
		{"anything else", func(t *testing.T) error { return errors.New("boom") }, problem.Unknown},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := c.err(t)
			if err == nil {
				t.Fatal("no error")
			}
			if got := problem.Classify(err); got != c.want {
				t.Errorf("%v: kind %v, want %v", err, got, c.want)
			}
		})
	}
}

func TestMarkKeepsTheText(t *testing.T) {
	base := errors.New("weather: location is missing")
	err := problem.Mark(problem.Missing, base)
	if err.Error() != base.Error() || !errors.Is(err, base) {
		t.Errorf("Mark changed the error: %q", err)
	}
	if problem.Mark(problem.Missing, nil) != nil {
		t.Error("Mark(nil) must stay nil")
	}
}

// A dial that times out means the Pi has no way out (no route, captive Wi-Fi), not a
// slow service: that is "offline". Only answers that never come are "timeout".
func TestDialTimeoutIsOffline(t *testing.T) {
	dial := &net.OpError{Op: "dial", Net: "tcp", Err: timeoutError{}}
	if got := problem.Classify(fmt.Errorf("GET x: %w", dial)); got != problem.Offline {
		t.Errorf("dial timeout: %v, want offline", got)
	}
	read := &net.OpError{Op: "read", Net: "tcp", Err: timeoutError{}}
	if got := problem.Classify(fmt.Errorf("GET x: %w", read)); got != problem.Timeout {
		t.Errorf("read timeout: %v, want timeout", got)
	}
}

type timeoutError struct{}

func (timeoutError) Error() string   { return "i/o timeout" }
func (timeoutError) Timeout() bool   { return true }
func (timeoutError) Temporary() bool { return true }
