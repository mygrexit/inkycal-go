package modules_test

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"inkycal/internal/config"
	"inkycal/internal/modules"
	"inkycal/internal/problem"
	"inkycal/internal/render"
)

type stub struct{}

func (stub) Generate(context.Context, *modules.Env) (*render.Canvas, error) { return nil, nil }

type cfg struct {
	Key   string         `json:"key"`
	Count config.FlexInt `json:"count"`
}

var def = modules.Define(modules.Meta{Name: "Stub", MinW: 100, MinH: 50}, func(c config.Common, rc cfg) (modules.Module, error) {
	if rc.Key == "" {
		return nil, modules.Missing("key")
	}
	if rc.Count < 0 {
		return nil, modules.Bad("count %d: want 0 or more", rc.Count)
	}
	return stub{}, nil
})

func common(w, h int) config.Common {
	return config.Common{Size: [2]int{w + 20, h + 20}, PaddingX: 10, PaddingY: 10}
}

// Every module checks in the same order, so a tile with several problems gets the
// same hint whatever module it runs.
func TestDefineChecksInOrder(t *testing.T) {
	cases := []struct {
		name, raw string
		w, h      int
		want      problem.Kind
	}{
		{"config does not decode", `{"key":"x","count":"many"}`, 10, 10, problem.BadSetting},
		{"missing beats too small", `{}`, 10, 10, problem.Missing},
		{"bad value beats too small", `{"key":"x","count":-1}`, 10, 10, problem.BadSetting},
		{"too small", `{"key":"x"}`, 99, 50, problem.TooSmall},
		{"empty config is {}", ``, 100, 50, problem.Missing},
	}
	for _, c := range cases {
		_, err := def.New(common(c.w, c.h), json.RawMessage(c.raw))
		if got := problem.Classify(err); got != c.want {
			t.Errorf("%s: %v (%v), want %v", c.name, got, err, c.want)
		}
		if err != nil && strings.Contains(strings.ToLower(err.Error()), "stub") {
			t.Errorf("%s: %q names the module; the caller does", c.name, err)
		}
	}
	if m, err := def.New(common(100, 50), json.RawMessage(`{"key":"x","count":2}`)); err != nil || m == nil {
		t.Errorf("valid tile: %v", err)
	}
}

func TestDefineKeepsMeta(t *testing.T) {
	if def.Meta.Name != "Stub" || def.Config() == nil || def.Config().Name() != "cfg" {
		t.Errorf("%+v", def)
	}
}

func TestMissingAndBadText(t *testing.T) {
	if err := modules.Missing("api_key"); err.Error() != "api_key is missing" || !errors.Is(err, err) {
		t.Errorf("%v", err)
	}
}
