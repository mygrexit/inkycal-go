package todoist

import (
	"slices"
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
	rows, err := buildRows(testProjects, testTasks, nil, true, now)
	if err != nil {
		t.Fatal(err)
	}
	want := []row{
		{Project: "Inbox", Due: "12-Oct-26", Priority: "● ", Name: "Brief"},
		{Project: "Inbox", Priority: "○ ", Name: "Milch"},
		{Project: "Work", Due: "8-Oct-26", Overdue: true, Priority: "● ", PriorityColour: true, Name: "Steuer"},
		{Project: "Work", Due: "TODAY", Name: "Meeting vorbereiten"},
	}
	if !slices.Equal(rows, want) {
		t.Errorf("rows =\n%+v\nwant\n%+v", rows, want)
	}
}

func TestBuildRowsWithoutPriority(t *testing.T) {
	now := time.Date(2026, 10, 10, 9, 0, 0, 0, berlin(t))
	rows, err := buildRows(testProjects, testTasks, nil, false, now)
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range rows {
		if r.Priority != "" || r.PriorityColour {
			t.Errorf("priority shown although disabled: %+v", r)
		}
	}
}

func TestBuildRowsFilter(t *testing.T) {
	now := time.Date(2026, 10, 10, 9, 0, 0, 0, berlin(t))
	rows, err := buildRows(testProjects, testTasks, []string{"Work"}, true, now)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 2 || rows[0].Project != "Work" || rows[1].Project != "Work" {
		t.Errorf("filtered rows = %+v", rows)
	}
	if _, err := buildRows(testProjects, testTasks, []string{"Nope"}, true, now); err == nil {
		t.Error("filter without matching project accepted")
	}
}
