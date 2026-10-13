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
	l := computeLayout(904, 624, 24, true, 5)
	want := layout{HeaderH: 36, CellW: 129, CellH: 46, GridX: 0, GridY: 36, EventsY: 36 + 5*46 + 24, EventsH: 624 - (36 + 5*46 + 24), NumSize: 27, Body: 24, Grid: true}
	if l != want {
		t.Errorf("layout = %+v, want %+v", l, want)
	}
	x, y := l.cell(1, 5)
	if x != 129*5 || y != 36+46 {
		t.Errorf("cell(1,5) = %d,%d", x, y)
	}
	noEvents := computeLayout(904, 624, 24, false, 5)
	if noEvents.EventsH != 0 || noEvents.CellH != (624-36)/5 {
		t.Errorf("no events layout = %+v", noEvents)
	}
	// Six weeks still leave room for seven event rows (1.5 lines each).
	six := computeLayout(904, 624, 24, true, 6)
	if six.EventsH < 7*24*3/2 {
		t.Errorf("six weeks squeeze the event list: %+v", six)
	}
}

// A flat band leaves the month grid unreadably small: the tile shows only the event list.
func TestFlatBandDropsTheGrid(t *testing.T) {
	l := computeLayout(700, 100, 12, true, 5)
	if l.Grid || l.EventsY != 0 || l.EventsH != 100 {
		t.Errorf("flat band: %+v", l)
	}
	if l := computeLayout(904, 624, 24, true, 5); !l.Grid {
		t.Error("a normal tile lost its grid")
	}
	if l := computeLayout(700, 100, 12, false, 5); !l.Grid {
		t.Error("without the event list the grid must stay")
	}
}
