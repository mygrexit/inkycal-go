// Package timefmt formats times with Arrow (Python) tokens, as used in Inkycal's settings.json.
package timefmt

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

// tokens are matched longest first.
var tokens = []string{"YYYY", "YY", "MMMM", "MMM", "MM", "M", "DD", "D", "dddd", "ddd", "d", "HH", "H", "hh", "h", "mm", "m", "ss", "s", "A", "a"}

// Format renders t using Arrow tokens (e.g. "ddd D MMM", "HH:mm"). Text in [brackets] is literal.
func Format(t time.Time, layout, lang string) string {
	l := lookup(lang)
	var b strings.Builder
	for i := 0; i < len(layout); {
		if layout[i] == '[' {
			if end := strings.IndexByte(layout[i:], ']'); end > 0 {
				b.WriteString(layout[i+1 : i+end])
				i += end + 1
				continue
			}
		}
		matched := ""
		for _, tok := range tokens {
			if strings.HasPrefix(layout[i:], tok) {
				matched = tok
				break
			}
		}
		if matched == "" {
			b.WriteByte(layout[i])
			i++
			continue
		}
		b.WriteString(render(t, matched, l))
		i += len(matched)
	}
	return b.String()
}

func render(t time.Time, tok string, l locale) string {
	wd := (int(t.Weekday()) + 6) % 7 // Monday = 0
	h12 := t.Hour() % 12
	if h12 == 0 {
		h12 = 12
	}
	switch tok {
	case "YYYY":
		return fmt.Sprintf("%04d", t.Year())
	case "YY":
		return fmt.Sprintf("%02d", t.Year()%100)
	case "MMMM":
		return l.months[t.Month()-1]
	case "MMM":
		return l.monthsShort[t.Month()-1]
	case "MM":
		return fmt.Sprintf("%02d", int(t.Month()))
	case "M":
		return strconv.Itoa(int(t.Month()))
	case "DD":
		return fmt.Sprintf("%02d", t.Day())
	case "D":
		return strconv.Itoa(t.Day())
	case "dddd":
		return l.days[wd]
	case "ddd":
		return l.daysShort[wd]
	case "d":
		return strconv.Itoa(wd + 1)
	case "HH":
		return fmt.Sprintf("%02d", t.Hour())
	case "H":
		return strconv.Itoa(t.Hour())
	case "hh":
		return fmt.Sprintf("%02d", h12)
	case "h":
		return strconv.Itoa(h12)
	case "mm":
		return fmt.Sprintf("%02d", t.Minute())
	case "m":
		return strconv.Itoa(t.Minute())
	case "ss":
		return fmt.Sprintf("%02d", t.Second())
	case "s":
		return strconv.Itoa(t.Second())
	case "A":
		if t.Hour() < 12 {
			return "AM"
		}
		return "PM"
	case "a":
		if t.Hour() < 12 {
			return "am"
		}
		return "pm"
	}
	return tok
}

// HumanizeDays renders a duration in days like Arrow's humanize(only_distance=True).
func HumanizeDays(days int, lang string) string {
	l := lookup(lang)
	switch {
	case days <= 1:
		return l.day
	case days < 7:
		return fmt.Sprintf(l.days2, days)
	case days < 14:
		return l.week
	default:
		return fmt.Sprintf(l.weeks, days/7)
	}
}
