package fetch

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

// With FreshFor a young cached copy is served without a request (the builder's
// live preview must not hammer calendars and weather APIs while you drag).
func TestFreshForServesYoungCacheWithoutRequest(t *testing.T) {
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		w.Write([]byte("body"))
	}))
	defer srv.Close()
	c := New(t.TempDir())
	c.FreshFor = time.Hour
	for i := 0; i < 3; i++ {
		res, err := c.Get(context.Background(), srv.URL, nil)
		if err != nil || string(res.Body) != "body" || res.Stale {
			t.Fatalf("get %d: %q stale=%v err=%v", i, res.Body, res.Stale, err)
		}
	}
	if n := hits.Load(); n != 1 {
		t.Errorf("%d requests, want 1", n)
	}
	// Without FreshFor every Get goes out, as on the device.
	c.FreshFor = 0
	c.Get(context.Background(), srv.URL, nil)
	if n := hits.Load(); n != 2 {
		t.Errorf("FreshFor 0: %d requests, want 2", n)
	}
}
