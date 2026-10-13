// Package modulestest checks a module's Meta against its factory and config structs.
package modulestest

import (
	"encoding/json"
	"reflect"
	"sort"
	"strings"
	"testing"

	"inkycal/internal/config"
	"inkycal/internal/modules"
)

// commonKeys are handled by config.Common, not by the module's Meta.
var commonKeys = map[string]bool{"size": true, "padding_x": true, "padding_y": true, "fontsize": true, "language": true, "cell": true}

// CheckMeta verifies that m lists exactly the JSON keys of the cfg structs (embedded
// structs included, common keys excluded), that its defaults plus sample values for
// required fields build at exactly the minimum size, and that 1 px less is refused.
func CheckMeta(t *testing.T, m modules.Meta, f modules.Factory, cfg ...any) {
	t.Helper()
	want := map[string]bool{}
	for _, c := range cfg {
		for _, k := range jsonKeys(reflect.TypeOf(c)) {
			if !commonKeys[k] {
				want[k] = true
			}
		}
	}
	got := map[string]bool{}
	for _, fl := range m.Fields {
		if got[fl.Key] {
			t.Errorf("%s: field %q listed twice", m.Name, fl.Key)
		}
		got[fl.Key] = true
		if fl.Kind == modules.KindSelect && len(fl.Options) == 0 {
			t.Errorf("%s: select %q has no options", m.Name, fl.Key)
		}
	}
	if diff := keyDiff(want, got); diff != "" {
		t.Errorf("%s: Meta fields differ from the config struct: %s", m.Name, diff)
	}
	build := func(w, h int) error {
		vals := m.Defaults()
		for _, fl := range m.Fields {
			if _, ok := vals[fl.Key]; !ok && fl.Required {
				vals[fl.Key] = sample(fl)
			}
		}
		vals["size"] = [2]int{w + 20, h + 20}
		vals["padding_x"], vals["padding_y"], vals["fontsize"], vals["language"] = 10, 10, 14, "en"
		raw, err := json.Marshal(vals)
		if err != nil {
			return err
		}
		c, err := config.ParseCommon(raw)
		if err != nil {
			return err
		}
		_, err = f(c, raw)
		return err
	}
	if err := build(m.MinW, m.MinH); err != nil {
		t.Errorf("%s: defaults at the minimum size %dx%d: %v", m.Name, m.MinW, m.MinH, err)
	}
	if build(m.MinW-1, m.MinH) == nil || build(m.MinW, m.MinH-1) == nil {
		t.Errorf("%s: accepted less than the minimum size %dx%d", m.Name, m.MinW, m.MinH)
	}
}

func sample(f modules.Field) any {
	switch f.Kind {
	case modules.KindURL, modules.KindURLList:
		return "https://example.com/x"
	case modules.KindNumber:
		return 1
	case modules.KindBool:
		return true
	case modules.KindSelect:
		return f.Options[0]
	}
	return "x"
}

func jsonKeys(t reflect.Type) []string {
	for t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	var keys []string
	for i := 0; i < t.NumField(); i++ {
		sf := t.Field(i)
		tag, _, _ := strings.Cut(sf.Tag.Get("json"), ",")
		if sf.Anonymous && tag == "" {
			keys = append(keys, jsonKeys(sf.Type)...)
			continue
		}
		if tag != "" && tag != "-" {
			keys = append(keys, tag)
		}
	}
	return keys
}

func keyDiff(want, got map[string]bool) string {
	var missing, extra []string
	for k := range want {
		if !got[k] {
			missing = append(missing, k)
		}
	}
	for k := range got {
		if !want[k] {
			extra = append(extra, k)
		}
	}
	sort.Strings(missing)
	sort.Strings(extra)
	if len(missing)+len(extra) == 0 {
		return ""
	}
	return "missing " + strings.Join(missing, ",") + "; extra " + strings.Join(extra, ",")
}
