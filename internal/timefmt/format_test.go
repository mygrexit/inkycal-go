package timefmt

import (
	"testing"
	"time"
)

func TestFormat(t *testing.T) {
	ts := time.Date(2026, time.March, 5, 14, 7, 9, 0, time.UTC) // Thursday
	cases := []struct{ layout, lang, want string }{
		{"ddd D MMM", "en", "Thu 5 Mar"},
		{"ddd D MMM", "de", "Do 5 Mär"},
		{"dddd, D. MMMM YYYY", "de_DE", "Donnerstag, 5. März 2026"},
		{"HH:mm", "en", "14:07"},
		{"h:mm a", "en", "2:07 pm"},
		{"h A", "en", "2 PM"},
		{"D-MMM-YY", "en", "5-Mar-26"},
		{"DD.MM.YYYY HH:mm:ss", "de", "05.03.2026 14:07:09"},
		{"[Week] d", "en", "Week 4"},
		{"MMMM", "xx", "March"},
	}
	for _, c := range cases {
		if got := Format(ts, c.layout, c.lang); got != c.want {
			t.Errorf("Format(%q,%q) = %q, want %q", c.layout, c.lang, got, c.want)
		}
	}
}

func TestFormatMidnightAndNoon12h(t *testing.T) {
	if got := Format(time.Date(2026, 1, 1, 0, 5, 0, 0, time.UTC), "h:mm a", "en"); got != "12:05 am" {
		t.Errorf("midnight: %q", got)
	}
	if got := Format(time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC), "h a", "en"); got != "12 pm" {
		t.Errorf("noon: %q", got)
	}
}

func TestHumanizeDays(t *testing.T) {
	cases := []struct {
		days       int
		lang, want string
	}{
		{1, "en", "a day"}, {3, "en", "3 days"}, {7, "en", "a week"}, {15, "en", "2 weeks"},
		{1, "de", "ein Tag"}, {3, "de", "3 Tage"}, {8, "de", "eine Woche"}, {21, "de", "3 Wochen"},
	}
	for _, c := range cases {
		if got := HumanizeDays(c.days, c.lang); got != c.want {
			t.Errorf("HumanizeDays(%d,%q) = %q, want %q", c.days, c.lang, got, c.want)
		}
	}
}
