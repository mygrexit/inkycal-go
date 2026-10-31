// Package fetchtest points a fetch.Client at a local test server.
package fetchtest

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"inkycal/internal/fetch"
)

// Rewrite sends every request (api.openweathermap.org, api.todoist.com, …) to Target,
// keeping its path and query, so tests serve the real paths.
type Rewrite struct{ Target *url.URL }

func (t Rewrite) RoundTrip(r *http.Request) (*http.Response, error) {
	r2 := r.Clone(r.Context())
	r2.URL.Scheme = t.Target.Scheme
	r2.URL.Host = t.Target.Host
	r2.Host = t.Target.Host
	return http.DefaultTransport.RoundTrip(r2)
}

// Client is a fetch.Client whose requests all go to a test server serving h: one
// attempt each, its cache in a temporary directory.
func Client(t testing.TB, h http.Handler) *fetch.Client {
	t.Helper()
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	target, err := url.Parse(srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	fc := fetch.New(t.TempDir())
	fc.HTTP = &http.Client{Transport: Rewrite{Target: target}, Timeout: 10 * time.Second}
	fc.Attempts = 1
	return fc
}
