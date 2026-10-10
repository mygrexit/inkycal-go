// Package ical loads iCalendar feeds/files and expands recurring events (via gocal).
package ical

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/apognu/gocal"

	"inkycal/internal/fetch"
)

type Event struct {
	Title      string
	Start, End time.Time // in the requested location; End is exclusive
	AllDay     bool
}

// windowsZones maps Outlook/Exchange TZIDs to IANA names (gocal would silently fall back to UTC).
var windowsZones = map[string]string{
	"W. Europe Standard Time":        "Europe/Berlin",
	"Central Europe Standard Time":   "Europe/Budapest",
	"Central European Standard Time": "Europe/Warsaw",
	"Romance Standard Time":          "Europe/Paris",
	"GMT Standard Time":              "Europe/London",
	"E. Europe Standard Time":        "Europe/Chisinau",
	"FLE Standard Time":              "Europe/Kiev",
	"Eastern Standard Time":          "America/New_York",
	"Central Standard Time":          "America/Chicago",
	"Mountain Standard Time":         "America/Denver",
	"Pacific Standard Time":          "America/Los_Angeles",
	"UTC":                            "UTC",
}

func init() {
	gocal.SetTZMapper(func(s string) (*time.Location, error) {
		if name, ok := windowsZones[s]; ok {
			return time.LoadLocation(name)
		}
		return nil, fmt.Errorf("no windows mapping for %q", s)
	})
}

func midnight(t time.Time) time.Time {
	y, m, d := t.Date()
	return time.Date(y, m, d, 0, 0, 0, 0, t.Location())
}

// Parse returns events overlapping [from, to), sorted by start.
func Parse(r io.Reader, from, to time.Time, loc *time.Location) ([]Event, error) {
	// gocal's own range check excludes events starting exactly at Start, so widen and filter ourselves.
	start, end := from.Add(-24*time.Hour), to.Add(24*time.Hour)
	p := gocal.NewParser(r)
	p.Start, p.End = &start, &end
	p.AllDayEventsTZ = loc
	p.Strict.Mode = gocal.StrictModeFailEvent // one broken event must not hide the whole calendar
	if err := p.Parse(); err != nil {
		return nil, err
	}
	var out []Event
	for _, e := range p.Events {
		if e.Start == nil {
			continue
		}
		s := e.Start.In(loc)
		allDay := e.RawStart.Params["VALUE"] == "DATE" || len(e.RawStart.Value) == 8
		en := s
		if e.End != nil {
			en = e.End.In(loc)
		}
		if allDay {
			// gocal reports all-day ends as 23:59:59.999 of the last day; make them exclusive midnights.
			en = midnight(en.Add(time.Millisecond))
			if !en.After(s) {
				en = s.AddDate(0, 0, 1)
			}
		} else if s.Equal(midnight(s)) && en.Equal(midnight(en)) && en.Sub(s) >= 24*time.Hour {
			allDay = true // same rule as Python's iCalendar.all_day
		}
		overlaps := s.Before(to) && (en.After(from) || (en.Equal(s) && !s.Before(from)))
		if !overlaps {
			continue
		}
		out = append(out, Event{Title: strings.TrimSpace(e.Summary), Start: s, End: en, AllDay: allDay})
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Start.Before(out[j].Start) })
	return out, nil
}

// Load fetches all URLs in parallel (webcal:// is treated as https://), reads all files,
// and returns the merged, sorted events. stale is true if any URL came from the cache.
func Load(ctx context.Context, fc *fetch.Client, urls, files []string, from, to time.Time, loc *time.Location) ([]Event, bool, error) {
	type result struct {
		events []Event
		stale  bool
		err    error
	}
	results := make([]result, len(urls))
	var wg sync.WaitGroup
	for i, u := range urls {
		wg.Add(1)
		go func(i int, u string) {
			defer wg.Done()
			if rest, ok := strings.CutPrefix(u, "webcal://"); ok {
				u = "https://" + rest
			}
			res, err := fc.Get(ctx, u, nil)
			if err != nil {
				results[i].err = fmt.Errorf("calendar %d: %w", i+1, err)
				return
			}
			ev, err := Parse(bytes.NewReader(res.Body), from, to, loc)
			if err != nil {
				results[i].err = fmt.Errorf("calendar %d: %w", i+1, err)
				return
			}
			results[i] = result{events: ev, stale: res.Stale}
		}(i, u)
	}
	wg.Wait()

	var all []Event
	stale := false
	for _, r := range results {
		if r.err != nil {
			return nil, false, r.err
		}
		all = append(all, r.events...)
		stale = stale || r.stale
	}
	for _, path := range files {
		f, err := os.Open(path)
		if err != nil {
			return nil, false, err
		}
		ev, err := Parse(f, from, to, loc)
		f.Close()
		if err != nil {
			return nil, false, fmt.Errorf("%s: %w", path, err)
		}
		all = append(all, ev...)
	}
	sort.SliceStable(all, func(i, j int) bool { return all[i].Start.Before(all[j].Start) })
	return all, stale, nil
}
