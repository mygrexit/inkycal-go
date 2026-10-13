package todoist

import (
	"slices"
	"testing"
	"time"
	_ "time/tzdata"

	"inkycal/internal/render"
)

func berlin(t *testing.T) *time.Location {
	t.Helper()
	loc, err := time.LoadLocation("Europe/Berlin")
	if err != nil {
		t.Fatal(err)
	}
	return loc
}

var (
	testProjects = []project{{ID: "p1", Name: "Inbox"}, {ID: "p2", Name: "Work"}}
	testTasks    = []task{
		{Content: "Steuer", ProjectID: "p2", Priority: 4, Due: &due{Date: "2026-10-08"}},
		{Content: "Meeting vorbereiten", ProjectID: "p2", Priority: 1, Due: &due{Date: "2026-10-10T15:00:00Z"}},
		{Content: "Milch", ProjectID: "p1", Priority: 2},
		{Content: "Altes Projekt", ProjectID: "p9", Priority: 3},
		{Content: "Kaputt", ProjectID: "p1", Priority: 1, Due: &due{Date: "not-a-date"}},
		{Content: "Brief", ProjectID: "p1", Priority: 3, Due: &due{Date: "2026-10-12T09:00:00"}},
	}
)

func TestBuildRows(t *testing.T) {
	now := time.Date(2026, 10, 10, 9, 0, 0, 0, berlin(t))
	rows, err := buildRows(testProjects, testTasks, nil, true, now, "de")
	if err != nil {
		t.Fatal(err)
	}
	// Most urgent first: overdue, today, upcoming, undated; then priority.
	want := []row{
		{Name: "Steuer", Due: "überfällig", Urgent: true, High: true},
		{Name: "Meeting vorbereiten", Due: "heute", Urgent: true},
		{Name: "Brief", Due: "Mo"},
		{Name: "Milch"},
	}
	if !slices.Equal(rows, want) {
		t.Errorf("rows =\n%+v\nwant\n%+v", rows, want)
	}
}

func TestDueLabel(t *testing.T) {
	now := time.Date(2026, 10, 10, 9, 0, 0, 0, berlin(t))
	day := func(d int) time.Time { return time.Date(2026, 10, 10+d, 0, 0, 0, 0, now.Location()) }
	for _, c := range []struct {
		offset int
		lang   string
		want   string
	}{{-1, "de", "überfällig"}, {0, "de", "heute"}, {1, "de", "morgen"}, {4, "de", "Mi"}, {9, "de", "19. Okt"}, {0, "en", "today"}, {1, "en", "tomorrow"}} {
		if got := dueLabel(day(c.offset), now, c.lang); got != c.want {
			t.Errorf("offset %d %s: %q, want %q", c.offset, c.lang, got, c.want)
		}
	}
}

func TestBuildRowsWithoutPriority(t *testing.T) {
	now := time.Date(2026, 10, 10, 9, 0, 0, 0, berlin(t))
	rows, err := buildRows(testProjects, testTasks, nil, false, now, "de")
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range rows {
		if r.High {
			t.Errorf("priority shown although disabled: %+v", r)
		}
	}
}

func TestBuildRowsFilter(t *testing.T) {
	now := time.Date(2026, 10, 10, 9, 0, 0, 0, berlin(t))
	rows, err := buildRows(testProjects, testTasks, []string{"Work"}, true, now, "de")
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 2 || rows[0].Name != "Steuer" || rows[1].Name != "Meeting vorbereiten" {
		t.Errorf("filtered rows = %+v", rows)
	}
	if _, err := buildRows(testProjects, testTasks, []string{"Nope"}, true, now, "de"); err == nil {
		t.Error("filter without matching project accepted")
	}
}

// A narrow tile with few tasks uses one full-width column instead of two cramped ones.
func TestDrawRowsOneColumnWhenNarrowOrShort(t *testing.T) {
	rows := []row{{Name: "Milch und Brot", Due: "heute", Urgent: true}, {Name: "Fahrradschloss"}}
	c := render.NewCanvas(240, 400, render.FontDefault, 16)
	drawRows(c, rows, 16, "de")
	// the due label sits at the right edge of the tile, not of a half-width column
	if _, col := inkInRect(c, 200, 0, 240, 30); !col {
		t.Error("due label not at the right edge: still two columns")
	}
}

func inkInRect(c *render.Canvas, x0, y0, x1, y1 int) (black, colour bool) {
	for y := y0; y < y1; y++ {
		for x := x0; x < x1; x++ {
			b, col := c.InkAt(x, y)
			black, colour = black || b, colour || col
		}
	}
	return
}
