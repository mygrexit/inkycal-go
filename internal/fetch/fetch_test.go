package fetch

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
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

func TestUserAgentDefaultAndOverride(t *testing.T) {
	var got []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = append(got, r.UserAgent())
	}))
	defer srv.Close()
	c := New("")
	c.Get(context.Background(), srv.URL, nil)
	c.Get(context.Background(), srv.URL, http.Header{"User-Agent": {"custom"}})
	if len(got) != 2 || got[0] != "inkycal-go" || got[1] != "custom" {
		t.Errorf("user agents %q", got)
	}
}

func TestPruneDropsOldEntriesOnly(t *testing.T) {
	dir := t.TempDir()
	c := New(dir)
	c.store("old", []byte("x"))
	c.store("new", []byte("y"))
	past := time.Now().Add(-30 * 24 * time.Hour)
	if err := os.Chtimes(filepath.Join(dir, "old"), past, past); err != nil {
		t.Fatal(err)
	}
	c.Prune(14 * 24 * time.Hour)
	if _, _, ok := c.load("old"); ok {
		t.Error("old entry kept")
	}
	if _, _, ok := c.load("new"); !ok {
		t.Error("fresh entry pruned")
	}
	New("").Prune(time.Hour) // no cache: no-op
}

// Private calendar addresses carry their secret in the path, not the query.
func TestRedactHidesPathTokens(t *testing.T) {
	cases := map[string]string{
		"https://calendar.google.com/calendar/ical/me%40gmail.com/private-0a1b2c3d4e5f6a7b8c9d/basic.ics":   "https://calendar.google.com/calendar/ical/me%40gmail.com/private-…/basic.ics",
		"webcal://p52-caldav.icloud.com/published/2/MTIzNDU2Nzg5MDEyMzQ1Njc4OTBhYmNkZWZn":                   "webcal://p52-caldav.icloud.com/published/2/…",
		"https://outlook.office365.com/owa/calendar/abc@x.de/8f2e9c71d4a54b6f9e3a1c0d2b7e8f90/calendar.ics": "https://outlook.office365.com/owa/calendar/abc@x.de/…/calendar.ics",
		"https://api.x/weather?appid=SECRET":                             "https://api.x/weather?…",
		"https://api.open-meteo.com/v1/forecast":                         "https://api.open-meteo.com/v1/forecast",
		"https://example.com/family/calendar.ics":                        "https://example.com/family/calendar.ics",
		"https://cal.example/feeds/8f2e9c71d4a54b6f9e3a1c0d2b7e8f90.ics": "https://cal.example/feeds/….ics",
	}
	for in, want := range cases {
		if got := Redact(in); got != want {
			t.Errorf("Redact(%s)\n got %s\nwant %s", in, got, want)
		}
	}
	msg := `calendar 1: GET https://calendar.google.com/calendar/ical/x/private-0a1b2c3d4e5f/basic.ics: HTTP 404; also "https://a.b/c?key=SECRET"`
	if got := RedactText(msg); strings.Contains(got, "0a1b2c3d4e5f") || strings.Contains(got, "SECRET") || !strings.Contains(got, "HTTP 404") {
		t.Errorf("RedactText: %s", got)
	}
	for _, msg := range []string{
		"GET webcal://p52-caldav.icloud.com/published/2/MTIzNDU2Nzg5MDEyMzQ1Njc4OTBhYmNkZWZn: HTTP 404", // token before ':'
		"GET HTTPS://api.x/w?appid=SECRET: HTTP 401",                                                    // upper-case scheme
		"bad path /data/2.5/weather?appid=SECRET",                                                       // no scheme at all
	} {
		if got := RedactText(msg); strings.Contains(got, "SECRET") || strings.Contains(got, "MTIzNDU2") {
			t.Errorf("RedactText(%q) = %q", msg, got)
		}
	}
}

// A URL that does not even parse must not reach the log with its key.
func TestBadURLDoesNotLeakKey(t *testing.T) {
	c := New("")
	c.Attempts = 1
	_, err := c.Get(context.Background(), "https://api.x/w?appid=SECRET\n", nil)
	if err == nil || strings.Contains(err.Error(), "SECRET") {
		t.Fatalf("error leaks key or is nil: %v", err)
	}
}
