package fetch

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// A POST goes the same way as a GET: retried on 5xx, and when the server stays down
// the last answer to the same body comes from the cache.
func TestPostRetriesAndFallsBackToCache(t *testing.T) {
	var down bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		if r.Method != http.MethodPost || r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("%s %q", r.Method, r.Header.Get("Content-Type"))
		}
		if down {
			w.WriteHeader(http.StatusBadGateway)
			return
		}
		w.Write(append([]byte("echo "), body...))
	}))
	defer srv.Close()
	c := New(t.TempDir())
	c.Backoff = time.Millisecond
	h := http.Header{"Content-Type": {"application/json"}}
	if res, err := c.Post(context.Background(), srv.URL, h, []byte(`["a"]`)); err != nil || string(res.Body) != `echo ["a"]` {
		t.Fatalf("%q %v", res.Body, err)
	}
	down = true
	res, err := c.Post(context.Background(), srv.URL, h, []byte(`["a"]`))
	if err != nil || !res.Stale || string(res.Body) != `echo ["a"]` {
		t.Errorf("down: %q stale=%v %v, want the cached answer", res.Body, res.Stale, err)
	}
	// another body is another request: nothing cached for it
	if _, err := c.Post(context.Background(), srv.URL, h, []byte(`["b"]`)); err == nil {
		t.Error("a different body was served from the cache")
	} else if se, ok := err.(*StatusError); !ok || se.Error()[:5] != "POST " {
		t.Errorf("err %v, want a POST status error", err)
	}
}
