package app

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"testing"
)

func TestCatalogMatchesRegistry(t *testing.T) {
	seen := map[string]bool{}
	for _, m := range Catalog {
		if _, ok := Registry[m.Name]; !ok {
			t.Errorf("Catalog has %q, Registry does not", m.Name)
		}
		seen[m.Name] = true
	}
	for name := range Registry {
		if !seen[name] {
			t.Errorf("Registry has %q without Meta in Catalog", name)
		}
	}
	if !sort.SliceIsSorted(Catalog, func(i, j int) bool { return Catalog[i].Title < Catalog[j].Title }) {
		t.Error("Catalog is not sorted by title")
	}
}

// Every key upstream's generator writes is editable in the builder, or listed here as
// read by upstream only.
var upstreamOnlyKeys = map[string]map[string]bool{
	"Fullweather": {"orientation": true, "keep_history": true, "font": true, "chart_title": true, "weekly_title": true, "icon_outline": true},
}

func TestCatalogCoversUpstreamKeys(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("testdata", "upstream-generator.json"))
	if err != nil {
		t.Fatal(err)
	}
	var s struct {
		Modules []struct {
			Name   string         `json:"name"`
			Config map[string]any `json:"config"`
		} `json:"modules"`
	}
	if err := json.Unmarshal(data, &s); err != nil {
		t.Fatal(err)
	}
	fields := map[string]map[string]bool{}
	for _, m := range Catalog {
		fields[m.Name] = map[string]bool{}
		for _, f := range m.Fields {
			fields[m.Name][f.Key] = true
		}
	}
	common := map[string]bool{"size": true, "padding_x": true, "padding_y": true, "fontsize": true, "language": true}
	for _, m := range s.Modules {
		if fields[m.Name] == nil {
			continue // not ported (Webshot)
		}
		for k := range m.Config {
			if !common[k] && !fields[m.Name][k] && !upstreamOnlyKeys[m.Name][k] {
				t.Errorf("%s: upstream key %q is not in Meta", m.Name, k)
			}
		}
	}
}
