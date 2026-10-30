package modules

import (
	"encoding/json"
	"errors"
	"fmt"
	"reflect"

	"inkycal/internal/config"
	"inkycal/internal/problem"
)

// Def is a module as the app registers it: what the builder shows and how a tile builds it.
type Def struct {
	Meta Meta
	New  Factory
	cfg  reflect.Type
}

// Config is the type a tile's config decodes into (for modulestest.CheckMeta).
func (d Def) Config() reflect.Type { return d.cfg }

// Define makes a module's Def. Its factory decodes the tile's config into a C (an empty
// config is {}), runs build and then checks Meta's minimum inner size. Every module
// checks in this order: a config that does not decode, a setting build refuses
// (Missing, Bad), a tile too small. build must not do I/O; errors never name the
// module, the caller puts the tile's name in front.
func Define[C any](m Meta, build func(c config.Common, cfg C) (Module, error)) Def {
	return Def{Meta: m, cfg: reflect.TypeFor[C](), New: func(c config.Common, raw json.RawMessage) (Module, error) {
		var cfg C
		if len(raw) > 0 {
			if err := json.Unmarshal(raw, &cfg); err != nil {
				return nil, problem.Mark(problem.BadSetting, fmt.Errorf("config: %w", err))
			}
		}
		mod, err := build(c, cfg)
		if err != nil {
			return nil, err
		}
		if err := CheckMinSize(c, m); err != nil {
			return nil, err
		}
		return mod, nil
	}}
}

// Missing is a required setting that is empty.
func Missing(key string) error {
	return problem.Mark(problem.Missing, errors.New(key+" is missing"))
}

// Bad is a setting with a value the module cannot use.
func Bad(format string, a ...any) error {
	return problem.Mark(problem.BadSetting, fmt.Errorf(format, a...))
}
