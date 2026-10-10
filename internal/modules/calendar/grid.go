package calendar

import "time"

// monthGrid mirrors Python's calendar.monthcalendar: weeks of 7 days, 0 = outside the month.
func monthGrid(year int, month time.Month, weekStart time.Weekday) [][7]int {
	first := time.Date(year, month, 1, 0, 0, 0, 0, time.UTC)
	days := first.AddDate(0, 1, -1).Day()
	col := (int(first.Weekday()) - int(weekStart) + 7) % 7
	var weeks [][7]int
	var week [7]int
	for d := 1; d <= days; d++ {
		week[col] = d
		col++
		if col == 7 {
			weeks = append(weeks, week)
			week = [7]int{}
			col = 0
		}
	}
	if col > 0 {
		weeks = append(weeks, week)
	}
	return weeks
}

// weekStartDate returns midnight of the first day of now's week.
func weekStartDate(now time.Time, weekStart time.Weekday) time.Time {
	offset := (int(now.Weekday()) - int(weekStart) + 7) % 7
	y, m, d := now.Date()
	return time.Date(y, m, d-offset, 0, 0, 0, 0, now.Location())
}

// layout holds the section sizes of inkycal_calendar.py.
type layout struct {
	MonthH, WeekdaysH int
	CellW, CellH      int
	GridX, GridY      int
	EventsH           int // includes the space of unused grid rows
	NumSize           int // font size of the day numbers
}

func computeLayout(w, h, lineHeight int, showEvents bool, weeks int) layout {
	l := layout{
		MonthH:    int(float64(h) * 0.10),
		WeekdaysH: int(float64(lineHeight) * 1.25),
	}
	calendarH := h - l.MonthH - l.WeekdaysH
	if showEvents {
		calendarH = int(float64(h) * 0.6)
		l.EventsH = h - l.MonthH - l.WeekdaysH - calendarH
	}
	l.CellW = w / 7
	l.CellH = calendarH / 6
	l.GridX = (w % 7) / 2
	l.GridY = l.MonthH + l.WeekdaysH + (h%6)/2
	l.NumSize = max(6, int(float64(min(l.CellW, l.CellH))*0.5))
	if showEvents {
		l.EventsH += (6 - weeks) * l.CellH
	}
	return l
}

func (l layout) cell(week, col int) (x, y int) {
	return l.GridX + l.CellW*col, l.GridY + l.CellH*week
}
