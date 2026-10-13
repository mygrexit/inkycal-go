package todoist

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"inkycal/internal/fetch"
)

// serveTodoist points todoistBaseURL at a test server with the given handler.
func serveTodoist(t *testing.T, h http.HandlerFunc) {
	t.Helper()
	srv := httptest.NewServer(h)
	old := todoistBaseURL
	todoistBaseURL = srv.URL
	t.Cleanup(func() {
		todoistBaseURL = old
		srv.Close()
	})
}

func TestFetchAllFollowsCursor(t *testing.T) {
	serveTodoist(t, func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer tok" {
			t.Errorf("Authorization = %q", got)
		}
		if r.URL.Path != "/tasks" || r.URL.Query().Get("limit") != "200" {
			t.Errorf("request = %s", r.URL)
		}
		switch r.URL.Query().Get("cursor") {
		case "":
			io.WriteString(w, `{"results":[{"content":"A","project_id":"p1","priority":4,"due":{"date":"2026-10-10"}}],"next_cursor":"c2"}`)
		case "c2":
			io.WriteString(w, `{"results":[{"content":"B","project_id":"p1","priority":1,"due":null}],"next_cursor":null}`)
		default:
			http.Error(w, "bad cursor", http.StatusBadRequest)
		}
	})

	tasks, stale, err := fetchAll[task](context.Background(), fetch.New(""), "tasks", "tok")
	if err != nil {
		t.Fatal(err)
	}
	if stale {
		t.Error("stale without cache")
	}
	if len(tasks) != 2 || tasks[0].Content != "A" || tasks[1].Content != "B" {
		t.Fatalf("tasks = %+v", tasks)
	}
	if tasks[0].Due == nil || tasks[0].Due.Date != "2026-10-10" || tasks[0].Priority != 4 || tasks[1].Due != nil {
		t.Errorf("fields = %+v / %+v", tasks[0], tasks[1])
	}
}

func TestFetchAllReportsHTTPError(t *testing.T) {
	serveTodoist(t, func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
	})
	if _, _, err := fetchAll[project](context.Background(), fetch.New(""), "projects", "bad"); err == nil {
		t.Error("401 not reported")
	}
}
