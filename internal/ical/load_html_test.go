package ical

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"inkycal/internal/fetch"
)

// A web page (often a login page) instead of a calendar is reported, not shown as "no events".
func TestLoadRejectsWebPages(t *testing.T) {
	for body, want := range map[string]string{
		"<!DOCTYPE html><html><body>Sign in</body></html>": "not an iCalendar file (got HTML)",
		"hello": "not an iCalendar file",
	} {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.Write([]byte(body)) }))
		now := time.Now()
		_, _, err := Load(context.Background(), fetch.New(""), []string{srv.URL}, nil, now, now.AddDate(0, 1, 0), time.UTC)
		srv.Close()
		if err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("%q: err %v, want %q", body, err, want)
		}
	}
}

// Some waste and CalDAV exports answer an empty body when there are no events.
func TestLoadAcceptsEmptyCalendar(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	defer srv.Close()
	now := time.Now()
	ev, _, err := Load(context.Background(), fetch.New(""), []string{srv.URL}, nil, now, now.AddDate(0, 1, 0), time.UTC)
	if err != nil || len(ev) != 0 {
		t.Errorf("empty body: %v %v", ev, err)
	}
}
