package modules

import (
	"fmt"

	"inkycal/internal/config"
	"inkycal/internal/problem"
)

// FieldKind tells the builder which input to show for a setting.
type FieldKind string

const (
	KindText      FieldKind = "text"   // one line
	KindSecret    FieldKind = "secret" // API keys, tokens: masked in the UI, blanked on export
	KindURL       FieldKind = "url"
	KindURLList   FieldKind = "url_list"  // config.StringList of URLs
	KindTextList  FieldKind = "text_list" // config.StringList
	KindNumber    FieldKind = "number"
	KindBool      FieldKind = "bool"
	KindSelect    FieldKind = "select"    // Options; values keep their JSON type (24 stays a number)
	KindMultiline FieldKind = "multiline" // several lines of text
	KindPlace     FieldKind = "place"     // place name with a search; also writes latitude and longitude
)

// Cond shows a field only while another field has one of Values.
type Cond struct {
	Key    string `json:"key"`
	Values []any  `json:"values"`
}

// Field is one module setting as the builder edits it.
type Field struct {
	Key      string            `json:"key"`
	Label    string            `json:"label"`
	Help     string            `json:"help,omitempty"`
	Kind     FieldKind         `json:"kind"`
	Default  any               `json:"default,omitempty"`
	Options  []any             `json:"options,omitempty"`
	Required bool              `json:"required,omitempty"`
	ShowIf   *Cond             `json:"show_if,omitempty"`
	Advanced bool              `json:"advanced,omitempty"` // shown under "More options"
	Custom   bool              `json:"custom,omitempty"`   // select that also takes a typed value
	Labels   map[string]string `json:"labels,omitempty"`   // option → label
}

// Meta describes a module for the builder: its settings and minimum inner size.
type Meta struct {
	Name        string  `json:"name"`  // the settings.json module name
	Title       string  `json:"title"` // shown in the builder
	Description string  `json:"description"`
	Group       string  `json:"group"` // palette section: weather, calendar, info, pictures, custom
	Icon        string  `json:"icon"`  // one emoji for the palette
	MinW        int     `json:"min_w"` // inner pixels
	MinH        int     `json:"min_h"`
	Fields      []Field `json:"fields"`
}

// Defaults returns the fields that have a default, keyed like settings.json.
func (m Meta) Defaults() map[string]any {
	out := map[string]any{}
	for _, f := range m.Fields {
		if f.Default != nil {
			out[f.Key] = f.Default
		}
	}
	return out
}

// CheckMinSize refuses a module smaller than its Meta minimum (inner pixels).
func CheckMinSize(c config.Common, m Meta) error {
	if w, h := c.InnerSize(); w < m.MinW || h < m.MinH {
		return problem.Mark(problem.TooSmall, fmt.Errorf("inner size %dx%d too small (min %dx%d)", w, h, m.MinW, m.MinH))
	}
	return nil
}

// AutoFontSize suggests a body size for a module's inner area: about nine lines high
// and 40 characters wide, between 12 and 32 px. The builder writes it as "fontsize".
func AutoFontSize(innerW, innerH int) int {
	return max(12, min(32, innerH/9, innerW/40))
}
