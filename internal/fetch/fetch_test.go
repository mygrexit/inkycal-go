package fetch

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func newTestClient(t *testing.T) *Client {
	c := New(t.TempDir())
	c.Backoff = time.Millisecond
	return c
}

func TestGetSuccessAndCacheFallback(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("hello"))
	}))
	c := newTestClient(t)
	res, err := c.Get(context.Background(), srv.URL+"/x", nil)
	if err != nil || string(res.Body) != "hello" || res.Stale {
		t.Fatalf("first get: %q %v %v", res.Body, res.Stale, err)
	}
	srv.Close() // network gone
	res, err = c.Get(context.Background(), srv.URL+"/x", nil)
	if err != nil || string(res.Body) != "hello" || !res.Stale {
		t.Fatalf("fallback: %q stale=%v err=%v", res.Body, res.Stale, err)
	}
}

func TestGetNoCacheReturnsError(t *testing.T) {
	srv := httptest.NewServer(http.NotFoundHandler())
	srv.Close()
	c := newTestClient(t)
	if _, err := c.Get(context.Background(), srv.URL, nil); err == nil {
		t.Fatal("expected error")
	}
}

func TestGet4xxIsNotRetriedAndIgnoresCache(t *testing.T) {
	var calls atomic.Int32
	status := http.StatusOK
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		w.WriteHeader(status)
		w.Write([]byte("body"))
	}))
	defer srv.Close()
	c := newTestClient(t)
	if _, err := c.Get(context.Background(), srv.URL, nil); err != nil {
		t.Fatal(err)
	}
	status = http.StatusUnauthorized
	calls.Store(0)
	_, err := c.Get(context.Background(), srv.URL, nil)
	var se *StatusError
	if !errors.As(err, &se) || se.Code != 401 {
		t.Fatalf("expected 401 StatusError, got %v", err)
	}
	if calls.Load() != 1 {
		t.Errorf("4xx retried: %d calls", calls.Load())
	}
}

func TestGetRetries5xx(t *testing.T) {
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if calls.Add(1) < 3 {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		w.Write([]byte("ok"))
	}))
	defer srv.Close()
	c := newTestClient(t)
	res, err := c.Get(context.Background(), srv.URL, nil)
	if err != nil || string(res.Body) != "ok" || calls.Load() != 3 {
		t.Fatalf("got %q err=%v calls=%d", res.Body, err, calls.Load())
	}
}

func TestHeadersSentAndCacheKeyedByAuth(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(r.Header.Get("Authorization")))
	}))
	c := newTestClient(t)
	a := http.Header{"Authorization": {"Bearer A"}}
	b := http.Header{"Authorization": {"Bearer B"}}
	c.Get(context.Background(), srv.URL, a)
	c.Get(context.Background(), srv.URL, b)
	srv.Close()
	res, _ := c.Get(context.Background(), srv.URL, a)
	if string(res.Body) != "Bearer A" {
		t.Errorf("cache mixed up tokens: %q", res.Body)
	}
}

func TestGetJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"temp": 21.5}`))
	}))
	defer srv.Close()
	var v struct {
		Temp float64 `json:"temp"`
	}
	stale, err := newTestClient(t).GetJSON(context.Background(), srv.URL, nil, &v)
	if err != nil || stale || v.Temp != 21.5 {
		t.Fatalf("%v %v %v", v, stale, err)
	}
}

func TestRedactHidesAPIKey(t *testing.T) {
	if got := redact("https://api.x/weather?appid=SECRET"); strings.Contains(got, "SECRET") {
		t.Errorf("key leaked: %s", got)
	}
}

func TestNetworkErrorDoesNotLeakKey(t *testing.T) {
	srv := httptest.NewServer(http.NotFoundHandler())
	srv.Close()
	c := New("")
	c.Attempts = 1
	_, err := c.Get(context.Background(), srv.URL+"/w?appid=SECRET", nil)
	if err == nil || strings.Contains(err.Error(), "SECRET") {
		t.Fatalf("error leaks key or is nil: %v", err)
	}
}
