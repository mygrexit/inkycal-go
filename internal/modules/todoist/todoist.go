// Package todoist shows open Todoist tasks, most urgent first.
package todoist

import (
	"context"
	"errors"
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

var Def = modules.Define(Meta, func(c config.Common, rc rawConfig) (modules.Module, error) {
	if strings.TrimSpace(rc.APIKey) == "" {
		return nil, modules.Missing("api_key")
	}
	m := &Module{common: c, token: rc.APIKey, filter: []string(rc.ProjectFilter), showPriority: true}
	if rc.ShowPriority != nil {
		m.showPriority = bool(*rc.ShowPriority)
	}
	return m, nil
})

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
	c := env.Canvas(w, h, render.FontDefault, int(m.common.FontSize))
	drawRows(c, rows, int(m.common.FontSize), m.common.Language)
	return c, nil
}
