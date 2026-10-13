package app

import (
	"path/filepath"
	"testing"

	"inkycal/internal/config"
)

// A settings.json in the shape upstream's web generator writes, with every module and
// its upstream keys, must load and every ported module must accept its config.
// Webshot is deliberately not ported; the app shows a hint for it instead of failing.
func TestUpstreamGeneratorSettings(t *testing.T) {
	s, err := config.Load(filepath.Join("testdata", "upstream-generator.json"))
	if err != nil {
		t.Fatal(err)
	}
	notPorted := map[string]bool{"Webshot": true, "Tindie": true}
	for _, e := range s.Modules {
		factory, ok := Registry[e.Name]
		if !ok {
			if !notPorted[e.Name] {
				t.Errorf("%s: not registered", e.Name)
			}
			continue
		}
		common, err := config.ParseCommon(e.Config)
		if err != nil {
			t.Errorf("%s: common keys: %v", e.Name, err)
			continue
		}
		if _, err := factory(common, e.Config); err != nil {
			t.Errorf("%s: %v", e.Name, err)
		}
	}
}
