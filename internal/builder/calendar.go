package builder

import (
	"context"
	"encoding/json"
	"net/http"
	"sort"
	"strings"
	"time"

	"inkycal/internal/ical"
	"inkycal/internal/timefmt"
)

// guides tell where each provider hides the secret iCal address of a calendar.
var guides = []map[string]string{
	{"name": "Google Calendar", "steps": "On a computer open calendar.google.com → ⚙ Settings → click your calendar on the left → \"Integrate calendar\" → copy \"Secret address in iCal format\"."},
	{"name": "Apple iCloud", "steps": "On iPhone: Calendar app → Calendars → ⓘ next to the calendar → turn on \"Public Calendar\" → Share Link (webcal:// works as is). The calendar becomes readable by anyone with the link."},
	{"name": "Outlook / Microsoft 365", "steps": "outlook.com → ⚙ Settings → Calendar → Shared calendars → \"Publish a calendar\" → choose the calendar and \"Can view all details\" → Publish → copy the ICS link."},
	{"name": "Nextcloud", "steps": "Calendar app → ⋯ next to the calendar → Share link → ⋯ → \"Copy subscription link\"."},
	{"name": "Waste collection", "steps": "Most towns offer the collection dates as an iCal/ICS subscription on their waste page (look for \"Abfuhrkalender\", \"iCal\", \"ICS\" or \"Kalender abonnieren\"). Copy that link."},
}

type calendarTest struct {
	OK     bool                `json:"ok"`
	Events int                 `json:"events"`
	Next   []map[string]string `json:"next,omitempty"`
	Error  string              `json:"error,omitempty"`
	Hint   string              `json:"hint,omitempty"`
}

// testCalendar loads one calendar address and says in plain words whether it works.
func (s *Server) testCalendar(w http.ResponseWriter, r *http.Request) {
	var req struct {
		URL string `json:"url"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 8<<10)).Decode(&req); err != nil || strings.TrimSpace(req.URL) == "" {
		http.Error(w, "send {\"url\": \"…\"}", http.StatusBadRequest)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()
	now := time.Now()
	events, _, err := ical.Load(ctx, s.live, []string{strings.TrimSpace(req.URL)}, nil, now, now.AddDate(0, 2, 0), time.Local)
	if err != nil {
		msg := queryString.ReplaceAllString(err.Error(), "?…")
		writeJSON(w, calendarTest{Error: msg, Hint: explain(msg)})
		return
	}
	sort.Slice(events, func(i, j int) bool { return events[i].Start.Before(events[j].Start) })
	out := calendarTest{OK: true, Events: len(events)}
	for _, e := range events[:min(3, len(events))] {
		out.Next = append(out.Next, map[string]string{"date": timefmt.Format(e.Start.In(time.Local), "ddd D MMM", "en"), "title": e.Title})
	}
	if len(events) == 0 {
		out.Hint = "The calendar works but has no events in the next two months."
	}
	writeJSON(w, out)
}
