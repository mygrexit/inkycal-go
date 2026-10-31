// Package ical loads iCalendar feeds/files (RFC 5545) and expands recurring events.
// Parsing is our own (small, tolerant like Python's icalendar); recurrence rules are
// expanded with rrule-go, a port of the dateutil code Python's Inkycal relies on.
package ical

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/teambition/rrule-go"

	"inkycal/internal/fetch"
	"inkycal/internal/problem"
)

type Event struct {
	Title      string
	Start, End time.Time // in the requested location; End is exclusive
	AllDay     bool
	Source     int // index of the calendar it came from: URLs first, then files
}

// windowsZones maps Outlook/Exchange TZIDs to IANA names.
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

// mapTZID resolves a TZID: quotes are stripped (RFC 5545 allows quoted values), Windows names are
// mapped, IANA names are loaded (also with a "/mozilla.org/…/Europe/Berlin" style prefix), and
// anything else (e.g. Outlook's "Customized Time Zone") is taken as the device's local zone.
func mapTZID(s string) *time.Location {
	s = strings.TrimSpace(strings.Trim(strings.TrimSpace(s), `"`))
	if name, ok := windowsZones[s]; ok {
		s = name
	}
	if loc, err := time.LoadLocation(s); err == nil && s != "" && s != "Local" {
		return loc
	}
	if parts := strings.Split(s, "/"); len(parts) >= 2 {
		if loc, err := time.LoadLocation(strings.Join(parts[len(parts)-2:], "/")); err == nil {
			return loc
		}
	}
	return time.Local
}

func midnight(t time.Time) time.Time {
	y, m, d := t.Date()
	return time.Date(y, m, d, 0, 0, 0, 0, t.Location())
}

// prop is one content line: NAME;PARAM=VALUE:value.
type prop struct {
	name   string
	params map[string]string
	value  string
}

// unfold joins RFC 5545 folded lines (continuations start with a space or tab).
func unfold(r io.Reader) ([]string, error) {
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 64*1024), 4<<20)
	var lines []string
	for sc.Scan() {
		l := strings.TrimRight(sc.Text(), "\r")
		if (strings.HasPrefix(l, " ") || strings.HasPrefix(l, "\t")) && len(lines) > 0 {
			lines[len(lines)-1] += l[1:]
			continue
		}
		lines = append(lines, l)
	}
	return lines, sc.Err()
}

// parseLine splits a content line; colons and semicolons inside quoted parameter values are kept.
func parseLine(l string) (prop, bool) {
	inQuote := false
	colon := -1
	for i, r := range l {
		if r == '"' {
			inQuote = !inQuote
		} else if r == ':' && !inQuote {
			colon = i
			break
		}
	}
	if colon < 0 {
		return prop{}, false
	}
	head, value := l[:colon], l[colon+1:]
	var parts []string
	inQuote, last := false, 0
	for i, r := range head {
		if r == '"' {
			inQuote = !inQuote
		} else if r == ';' && !inQuote {
			parts = append(parts, head[last:i])
			last = i + 1
		}
	}
	parts = append(parts, head[last:])
	p := prop{name: strings.ToUpper(parts[0]), params: map[string]string{}, value: value}
	for _, kv := range parts[1:] {
		if k, v, ok := strings.Cut(kv, "="); ok {
			p.params[strings.ToUpper(k)] = strings.Trim(v, `"`)
		}
	}
	return p, true
}

var textUnescape = strings.NewReplacer(`\n`, " ", `\N`, " ", `\,`, ",", `\;`, ";", `\\`, `\`)

// parseTime reads a DATE or DATE-TIME value: UTC ("Z"), with TZID, or floating (taken in loc).
func parseTime(v string, params map[string]string, loc *time.Location) (t time.Time, date bool, err error) {
	v = strings.TrimSpace(v)
	if params["VALUE"] == "DATE" || len(v) == 8 {
		t, err = time.ParseInLocation("20060102", v, loc)
		return t, true, err
	}
	if strings.HasSuffix(v, "Z") {
		t, err = time.Parse("20060102T150405Z", v)
		return t, false, err
	}
	in := loc
	if tz, ok := params["TZID"]; ok {
		in = mapTZID(tz)
	}
	t, err = time.ParseInLocation("20060102T150405", v, in)
	return t, false, err
}

// parseDuration reads an RFC 5545 duration such as P1D, PT1H30M, P2W or -PT15M.
func parseDuration(v string) (time.Duration, error) {
	v = strings.TrimSpace(v)
	sign := time.Duration(1)
	if strings.HasPrefix(v, "-") {
		sign, v = -1, v[1:]
	}
	v = strings.TrimPrefix(v, "+")
	if !strings.HasPrefix(v, "P") {
		return 0, fmt.Errorf("duration %q", v)
	}
	var d time.Duration
	inTime := false
	num := ""
	for _, r := range v[1:] {
		switch {
		case r == 'T':
			inTime = true
		case r >= '0' && r <= '9':
			num += string(r)
		default:
			n, err := strconv.Atoi(num)
			if err != nil {
				return 0, fmt.Errorf("duration %q", v)
			}
			unit := map[rune]time.Duration{'W': 7 * 24 * time.Hour, 'D': 24 * time.Hour}[r]
			if inTime {
				unit = map[rune]time.Duration{'H': time.Hour, 'M': time.Minute, 'S': time.Second}[r]
			}
			if unit == 0 {
				return 0, fmt.Errorf("duration %q", v)
			}
			d += time.Duration(n) * unit
			num = ""
		}
	}
	return sign * d, nil
}

// vevent is one parsed VEVENT before expansion.
type vevent struct {
	uid, summary string
	start, end   time.Time
	allDay       bool
	hasEnd       bool
	dur          time.Duration
	rrule        string
	rdates       []time.Time
	exdates      []time.Time
	recurrenceID time.Time
}

func parseEvents(r io.Reader, loc *time.Location) ([]*vevent, error) {
	lines, err := unfold(r)
	if err != nil {
		return nil, err
	}
	var out []*vevent
	var cur *vevent
	depth := 0 // nesting inside the VEVENT (VALARM etc.)
	for _, l := range lines {
		p, ok := parseLine(l)
		if !ok {
			continue
		}
		switch {
		case p.name == "BEGIN" && strings.EqualFold(p.value, "VEVENT") && cur == nil:
			cur, depth = &vevent{}, 0
			continue
		case p.name == "BEGIN" && cur != nil:
			depth++
			continue
		case p.name == "END" && cur != nil && depth > 0:
			depth--
			continue
		case p.name == "END" && strings.EqualFold(p.value, "VEVENT") && cur != nil:
			if !cur.start.IsZero() { // without DTSTART there is nothing to show
				out = append(out, cur)
			}
			cur = nil
			continue
		}
		if cur == nil || depth > 0 {
			continue
		}
		// A broken property drops only itself, never the event or the calendar.
		switch p.name {
		case "UID":
			cur.uid = strings.TrimSpace(p.value)
		case "SUMMARY":
			cur.summary = strings.TrimSpace(textUnescape.Replace(p.value))
		case "DTSTART":
			if t, date, err := parseTime(p.value, p.params, loc); err == nil {
				cur.start, cur.allDay = t, date
			}
		case "DTEND":
			if t, _, err := parseTime(p.value, p.params, loc); err == nil {
				cur.end, cur.hasEnd = t, true
			}
		case "DURATION":
			if d, err := parseDuration(p.value); err == nil {
				cur.dur = d
			}
		case "RRULE":
			cur.rrule = p.value
		case "RDATE", "EXDATE":
			if p.params["VALUE"] == "PERIOD" {
				continue
			}
			for _, v := range strings.Split(p.value, ",") {
				t, _, err := parseTime(v, p.params, loc)
				if err != nil {
					continue
				}
				if p.name == "RDATE" {
					cur.rdates = append(cur.rdates, t)
				} else {
					cur.exdates = append(cur.exdates, t)
				}
			}
		case "RECURRENCE-ID":
			if t, _, err := parseTime(p.value, p.params, loc); err == nil {
				cur.recurrenceID = t
			}
		}
	}
	return out, nil
}

// length is the event's duration: DTEND, else DURATION, else one day for a date and
// zero for a date-time (RFC 5545 3.6.1; Python shows such events too).
func (e *vevent) length() time.Duration {
	switch {
	case e.hasEnd:
		return e.end.Sub(e.start)
	case e.dur != 0:
		return e.dur
	case e.allDay:
		return 24 * time.Hour
	}
	return 0
}

// endFor is the exclusive end of an occurrence starting at s; dates keep whole days across DST.
func (e *vevent) endFor(s time.Time) time.Time {
	if e.allDay {
		days := int((e.length() + 12*time.Hour) / (24 * time.Hour))
		return s.AddDate(0, 0, max(days, 1))
	}
	return s.Add(e.length())
}

// occurrences lists the starts of e in [from-length, to), honouring RRULE, RDATE and EXDATE.
// DTSTART is always the first occurrence (RFC 5545), even if the rule would skip it.
func (e *vevent) occurrences(from, to time.Time) ([]time.Time, error) {
	if e.rrule == "" && len(e.rdates) == 0 {
		return []time.Time{e.start}, nil
	}
	set := rrule.Set{}
	if e.rrule != "" {
		opt, err := rrule.StrToROptionInLocation(e.rrule, e.start.Location())
		if err != nil {
			return nil, err
		}
		opt.Dtstart = e.start
		r, err := rrule.NewRRule(*opt)
		if err != nil {
			return nil, err
		}
		set.RRule(r)
	}
	set.RDate(e.start)
	for _, t := range e.rdates {
		set.RDate(t)
	}
	for _, t := range e.exdates {
		set.ExDate(t)
	}
	return set.Between(from.Add(-e.length()-time.Second), to, true), nil
}

// Parse returns events overlapping [from, to), sorted by start.
func Parse(r io.Reader, from, to time.Time, loc *time.Location) ([]Event, error) {
	vevents, err := parseEvents(r, loc)
	if err != nil {
		return nil, err
	}
	// Overrides (RECURRENCE-ID) replace single occurrences of their series.
	type key struct {
		uid string
		at  int64
	}
	overrides := map[key]bool{}
	for _, e := range vevents {
		if !e.recurrenceID.IsZero() && e.uid != "" {
			overrides[key{e.uid, e.recurrenceID.Unix()}] = true
		}
	}
	var out []Event
	add := func(e *vevent, s time.Time) {
		en := e.endFor(s)
		s, en = s.In(loc), en.In(loc)
		allDay := e.allDay
		if !allDay && s.Equal(midnight(s)) && en.Equal(midnight(en)) && en.Sub(s) >= 24*time.Hour {
			allDay = true // same rule as Python's iCalendar.all_day
		}
		overlaps := s.Before(to) && (en.After(from) || (en.Equal(s) && !s.Before(from)))
		if overlaps {
			out = append(out, Event{Title: e.summary, Start: s, End: en, AllDay: allDay})
		}
	}
	for _, e := range vevents {
		if !e.recurrenceID.IsZero() {
			add(e, e.start) // the moved/changed occurrence itself
			continue
		}
		starts, err := e.occurrences(from, to)
		if err != nil {
			slog.Warn("calendar: skipping event with an unreadable repeat rule", "title", e.summary, "err", err)
			continue
		}
		for _, s := range starts {
			if e.uid != "" && overrides[key{e.uid, s.Unix()}] {
				continue
			}
			add(e, s)
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Start.Before(out[j].Start) })
	return out, nil
}

// checkCalendar refuses bodies that are not iCalendar, typically a login page a wrong
// (non-secret) calendar address leads to; parsing those would just show no events.
func checkCalendar(body []byte) error {
	if len(bytes.TrimSpace(body)) == 0 {
		return nil // some exports answer nothing when there are no events
	}
	head := body[:min(len(body), 4096)]
	if bytes.Contains(body, []byte("BEGIN:VCALENDAR")) || bytes.Contains(bytes.ToUpper(head), []byte("BEGIN:VCALENDAR")) {
		return nil
	}
	if lower := bytes.ToLower(head); bytes.Contains(lower, []byte("<html")) || bytes.Contains(lower, []byte("<!doctype html")) {
		return problem.Mark(problem.NotCalendar, errors.New("not an iCalendar file (got HTML)"))
	}
	return problem.Mark(problem.BadResponse, errors.New("not an iCalendar file"))
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
			if err := checkCalendar(res.Body); err != nil {
				results[i].err = fmt.Errorf("calendar %d: %w", i+1, err)
				return
			}
			ev, err := Parse(bytes.NewReader(res.Body), from, to, loc)
			if err != nil {
				results[i].err = fmt.Errorf("calendar %d: %w", i+1, err)
				return
			}
			for j := range ev {
				ev[j].Source = i
			}
			results[i] = result{events: ev, stale: res.Stale}
		}(i, u)
	}
	wg.Wait()

	// A dead calendar must not hide the others: keep what loaded, fail only if nothing did.
	var all []Event
	var errs []error
	stale := false
	for _, r := range results {
		if r.err != nil {
			errs = append(errs, r.err)
			continue
		}
		all = append(all, r.events...)
		stale = stale || r.stale
	}
	if len(errs) > 0 {
		if len(errs) == len(results) && len(files) == 0 {
			return nil, false, errors.Join(errs...)
		}
		slog.Warn("calendar: some sources failed, showing the others", "err", errors.Join(errs...))
		stale = true
	}
	for fi, path := range files {
		f, err := os.Open(path)
		if err != nil {
			return nil, false, err
		}
		ev, err := Parse(f, from, to, loc)
		f.Close()
		if err != nil {
			return nil, false, fmt.Errorf("%s: %w", path, err)
		}
		for j := range ev {
			ev[j].Source = len(urls) + fi
		}
		all = append(all, ev...)
	}
	sort.SliceStable(all, func(i, j int) bool { return all[i].Start.Before(all[j].Start) })
	return all, stale, nil
}
