// Package todoist shows open Todoist tasks, most urgent first.
package todoist

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"

	"inkycal/internal/config"
	"inkycal/internal/modules"
	"inkycal/internal/render"
)

type Module struct {
	common       config.Common
	token        string
	filter       []string
	showPriority bool
}

type rawConfig struct {
	APIKey        string            `json:"api_key"`
	ProjectFilter config.StringList `json:"project_filter"`
	ShowPriority  *config.FlexBool  `json:"show_priority"`
}

func New(c config.Common, raw json.RawMessage) (modules.Module, error) {
	var rc rawConfig
	if err := json.Unmarshal(raw, &rc); err != nil {
		return nil, fmt.Errorf("todoist: config: %w", err)
	}
	if strings.TrimSpace(rc.APIKey) == "" {
		return nil, errors.New("todoist: api_key is missing")
	}
	if err := modules.CheckMinSize(c, Meta); err != nil {
		return nil, err
	}
	m := &Module{common: c, token: rc.APIKey, filter: []string(rc.ProjectFilter), showPriority: true}
	if rc.ShowPriority != nil {
		m.showPriority = bool(*rc.ShowPriority)
	}
	return m, nil
}

func (m *Module) Generate(ctx context.Context, env *modules.Env) (*render.Canvas, error) {
	var (
		wg             sync.WaitGroup
		projects       []project
		tasks          []task
		staleP, staleT bool
		errP, errT     error
	)
	wg.Add(2)
	go func() {
		defer wg.Done()
		projects, staleP, errP = fetchAll[project](ctx, env.Fetch, "projects", m.token)
	}()
	go func() {
		defer wg.Done()
		tasks, staleT, errT = fetchAll[task](ctx, env.Fetch, "tasks", m.token)
	}()
	wg.Wait()
	if err := errors.Join(errP, errT); err != nil {
		return nil, err
	}
	env.Stale = env.Stale || staleP || staleT

	rows, err := buildRows(projects, tasks, m.filter, m.showPriority, env.Now, m.common.Language)
	if err != nil {
		return nil, err
	}
	w, h := m.common.InnerSize()
	c := render.NewCanvas(w, h, render.FontDefault, int(m.common.FontSize))
	drawRows(c, rows, int(m.common.FontSize), m.common.Language)
	return c, nil
}
