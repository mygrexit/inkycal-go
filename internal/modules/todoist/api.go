package todoist

import (
	"context"
	"fmt"
	"net/http"
	"net/url"

	"inkycal/internal/fetch"
)

// todoistBaseURL is a variable so tests can point it at an httptest server.
var todoistBaseURL = "https://api.todoist.com/api/v1"

// maxPages guards against a server that never stops returning cursors.
const maxPages = 50

type project struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type due struct {
	Date string `json:"date"`
}

type task struct {
	Content   string `json:"content"`
	ProjectID string `json:"project_id"`
	Priority  int    `json:"priority"`
	Due       *due   `json:"due"`
}

type page[T any] struct {
	Results    []T     `json:"results"`
	NextCursor *string `json:"next_cursor"`
}

// fetchAll reads every page of a list endpoint ("projects" or "tasks").
func fetchAll[T any](ctx context.Context, fc *fetch.Client, path, token string) ([]T, bool, error) {
	header := http.Header{}
	header.Set("Authorization", "Bearer "+token)
	var all []T
	stale := false
	cursor := ""
	for range maxPages {
		q := url.Values{}
		q.Set("limit", "200")
		if cursor != "" {
			q.Set("cursor", cursor)
		}
		var p page[T]
		s, err := fc.GetJSON(ctx, todoistBaseURL+"/"+path+"?"+q.Encode(), header, &p)
		if err != nil {
			return nil, false, fmt.Errorf("todoist: %s: %w", path, err)
		}
		stale = stale || s
		all = append(all, p.Results...)
		if p.NextCursor == nil || *p.NextCursor == "" {
			return all, stale, nil
		}
		cursor = *p.NextCursor
	}
	return nil, false, fmt.Errorf("todoist: %s: more than %d pages", path, maxPages)
}
