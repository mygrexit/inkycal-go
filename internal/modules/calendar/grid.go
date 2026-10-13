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

// layout places the weekday header, the month grid and the event list.
type layout struct {
	HeaderH          int // weekday names and the rule under them
	CellW, CellH     int
	GridX, GridY     int
	EventsY, EventsH int // event list area below the grid
	NumSize          int // font size of the day numbers
	Body             int // font size of the event list
}

// computeLayout sizes the grid from the body font size: rows are about two lines
// tall, the event list gets everything below the weeks of this month.
func computeLayout(w, h, body int, showEvents bool, weeks int) layout {
	l := layout{Body: body, HeaderH: body * 3 / 2, CellW: w / 7, GridX: (w % 7) / 2}
	l.GridY = l.HeaderH
	if showEvents {
		l.CellH = min(body*2, (h/2-l.HeaderH)/6)
	} else {
		l.CellH = (h - l.HeaderH) / max(weeks, 1)
	}
	l.CellH = max(l.CellH, 8)
	l.NumSize = max(6, min(l.CellH*3/5, l.CellW/2))
	if showEvents {
		l.EventsY = l.GridY + weeks*l.CellH + body
		l.EventsH = max(0, h-l.EventsY)
	}
	return l
}

func (l layout) cell(week, col int) (x, y int) {
	return l.GridX + l.CellW*col, l.GridY + l.CellH*week
}
