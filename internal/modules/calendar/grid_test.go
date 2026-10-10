package calendar

import (
	"testing"
	"time"
	_ "time/tzdata"
)

func berlin(t *testing.T) *time.Location {
	t.Helper()
	loc, err := time.LoadLocation("Europe/Berlin")
	if err != nil {
		t.Fatal(err)
	}
	return loc
}

func TestMonthGridMondayStart(t *testing.T) {
	weeks := monthGrid(2026, time.October, time.Monday) // 1 Oct 2026 is a Thursday
	if len(weeks) != 5 {
		t.Fatalf("weeks = %d, want 5", len(weeks))
	}
	if weeks[0] != [7]int{0, 0, 0, 1, 2, 3, 4} {
		t.Errorf("week 0 = %v", weeks[0])
	}
	if weeks[4] != [7]int{26, 27, 28, 29, 30, 31, 0} {
		t.Errorf("week 4 = %v", weeks[4])
	}
}

func TestMonthGridSundayStartAndFourWeeks(t *testing.T) {
	weeks := monthGrid(2026, time.October, time.Sunday)
	if weeks[0] != [7]int{0, 0, 0, 0, 1, 2, 3} {
		t.Errorf("sunday week 0 = %v", weeks[0])
	}
	feb := monthGrid(2021, time.February, time.Monday) // starts Monday, 28 days
	if len(feb) != 4 || feb[0] != [7]int{1, 2, 3, 4, 5, 6, 7} {
		t.Errorf("feb 2021 = %v", feb)
	}
}

func TestWeekStartDate(t *testing.T) {
	loc := berlin(t)
	sat := time.Date(2026, 10, 10, 9, 0, 0, 0, loc)
	if got := weekStartDate(sat, time.Monday); !got.Equal(time.Date(2026, 10, 5, 0, 0, 0, 0, loc)) {
		t.Errorf("monday start = %v", got)
	}
	if got := weekStartDate(sat, time.Sunday); !got.Equal(time.Date(2026, 10, 4, 0, 0, 0, 0, loc)) {
		t.Errorf("sunday start = %v", got)
	}
}

func TestComputeLayout(t *testing.T) {
	l := computeLayout(460, 380, 20, true, 5)
	want := layout{MonthH: 38, WeekdaysH: 25, CellW: 65, CellH: 38, GridX: 2, GridY: 64, EventsH: 127, NumSize: 19}
	if l != want {
		t.Errorf("layout = %+v, want %+v", l, want)
	}
	x, y := l.cell(1, 5)
	if x != 2+65*5 || y != 64+38 {
		t.Errorf("cell(1,5) = %d,%d", x, y)
	}
	noEvents := computeLayout(460, 380, 20, false, 5)
	if noEvents.EventsH != 0 || noEvents.CellH != (380-38-25)/6 {
		t.Errorf("no events layout = %+v", noEvents)
	}
}
