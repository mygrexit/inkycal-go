// Package config reads Inkycal's settings.json (format compatible with the Python version).
package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"inkycal/internal/layout"
	"inkycal/internal/problem"
	"inkycal/internal/schedule"
)

// SearchPaths are tried in order when no explicit settings path is given.
var SearchPaths = []string{"/boot/settings.json", "/boot/firmware/settings.json"}

type Settings struct {
	Model               string        `json:"model"`
	UpdateInterval      FlexInt       `json:"update_interval"`
	UpdateTimes         StringList    `json:"update_times"` // "HH:MM" wake times; overrides update_interval
	Orientation         FlexInt       `json:"orientation"`
	InfoSection         FlexBool      `json:"info_section"`
	InfoSectionHeight   FlexInt       `json:"info_section_height"`
	CalibrationHours    []int         `json:"calibration_hours"`
	CalibrationCycles   FlexInt       `json:"calibration_cycles"`
	DisableCalibration  FlexBool      `json:"disable_calibration"` // upstream only logs it; here it really skips
	BorderAroundModules FlexBool      `json:"border_around_modules"`
	Dividers            FlexBool      `json:"dividers"`
	ImageHash           FlexBool      `json:"image_hash"`
	UsePiSugar          FlexBool      `json:"use_pi_sugar"`
	ShutdownAfterRun    FlexBool      `json:"shutdown_after_run"`
	Grid                *layout.Grid  `json:"grid"`       // grid layouts: cells per module instead of stacking
	ColourInk           string        `json:"colour_ink"` // "red" or "yellow": what the colour layer looks like (builder preview)
	Theme               *Style        `json:"theme"`      // page look; modules may override it with "style"
	Modules             []ModuleEntry `json:"modules"`

	// Warnings are problems Load worked around for the whole page (a broken theme).
	Warnings []string `json:"-"`

	dir string // directory of the loaded file, for relative font paths
}

// Dir is the directory settings.json was loaded from ("" when parsed from memory).
func (s *Settings) Dir() string { return s.dir }

// SetDir sets the directory relative font paths start from (for settings parsed from memory).
func (s *Settings) SetDir(dir string) { s.dir = dir }

type ModuleEntry struct {
	Position int             `json:"position"`
	Name     string          `json:"name"`
	Config   json.RawMessage `json:"config"`
}

// Common holds the keys every module config shares.
type Common struct {
	Size     [2]int       `json:"size"`
	PaddingX FlexInt      `json:"padding_x"`
	PaddingY FlexInt      `json:"padding_y"`
	FontSize FlexInt      `json:"fontsize"`
	Language string       `json:"language"`
	Cell     *layout.Cell `json:"cell"`  // grid layouts: [col, row, colSpan, rowSpan]; size then comes from the cell
	Style    *Style       `json:"style"` // the module's own look over the page theme
}

// InnerSize is the drawable area: size minus padding on both sides.
func (c Common) InnerSize() (w, h int) {
	return c.Size[0] - 2*int(c.PaddingX), c.Size[1] - 2*int(c.PaddingY)
}

// ParseCommon extracts the shared module keys and applies defaults. Modules on a grid
// (Cell set) get their size from the cell: call WithSize before using it.
func ParseCommon(raw json.RawMessage) (Common, error) {
	c, err := decodeCommon(raw)
	if err != nil || c.Cell != nil {
		return c, err
	}
	return c, c.checkSize()
}

// decodeCommon reads the shared module keys and fills in defaults; on an error the keys
// stay as far as they decoded, without defaults.
func decodeCommon(raw json.RawMessage) (Common, error) {
	var c Common
	if err := json.Unmarshal(raw, &c); err != nil {
		return c, err
	}
	if c.FontSize == 0 {
		c.FontSize = 14
	}
	if c.Language == "" {
		c.Language = "en"
	}
	return c, nil
}

// WithSize sets the section size (grid layouts) and checks it against the padding.
func (c Common) WithSize(w, h int) (Common, error) {
	c.Size = [2]int{w, h}
	return c, c.checkSize()
}

func (c Common) checkSize() error {
	if w, h := c.InnerSize(); w <= 0 || h <= 0 {
		return problem.Mark(problem.Padding, fmt.Errorf("module size %v too small for padding %d/%d", c.Size, c.PaddingX, c.PaddingY))
	}
	return nil
}

// Load reads and defaults a settings file for the panel. It is lenient: a broken theme
// falls back to the default look, and a broken tile only fails itself (see Tiles), so
// one bad tile does not stop the panel refreshing.
func Load(path string) (*Settings, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	s, err := decode(data, path)
	if err != nil {
		return nil, err
	}
	if err := s.Theme.validate(); err != nil {
		s.Warnings = append(s.Warnings, fmt.Sprintf("theme: %v – using the default look", err))
		s.Theme = nil
	}
	s.dir = filepath.Dir(path)
	return s, nil
}

// Parse defaults and validates settings.json content; name appears in messages. Unlike
// Load it is strict: it rejects a broken theme and any tile that cannot be placed or has
// a broken style, so the builder never saves one.
func Parse(data []byte, name string) (*Settings, error) {
	s, err := decode(data, name)
	if err != nil {
		return nil, err
	}
	if err := s.Theme.validate(); err != nil {
		return nil, problem.Mark(problem.Style, fmt.Errorf("theme: %w", err))
	}
	_, tiles, err := s.Tiles()
	if err != nil {
		return nil, err
	}
	for i, t := range tiles {
		if t.rejected != nil {
			return nil, fmt.Errorf("module %d (%s): %w", i+1, s.Modules[i].Name, t.rejected)
		}
	}
	return s, nil
}

// decode reads settings.json content, fills in defaults and checks the page-wide keys.
func decode(data []byte, name string) (*Settings, error) {
	var s Settings
	if err := json.Unmarshal(data, &s); err != nil {
		return nil, problem.Mark(problem.BadSetting, fmt.Errorf("%s: %w", name, err))
	}
	if s.UpdateInterval <= 0 {
		s.UpdateInterval = 60
	}
	if s.InfoSection && s.InfoSectionHeight <= 0 {
		s.InfoSectionHeight = 70
	}
	if s.CalibrationCycles <= 0 {
		s.CalibrationCycles = 3
	}
	if s.Orientation != 0 && s.Orientation != 180 {
		return nil, fmt.Errorf("orientation must be 0 or 180, got %d", s.Orientation)
	}
	if s.Model == "" {
		return nil, errors.New("model is missing") // page-wide: no tile advice fits
	}
	if _, err := schedule.ParseTimes(s.UpdateTimes); err != nil {
		return nil, err
	}
	sort.SliceStable(s.Modules, func(i, j int) bool { return s.Modules[i].Position < s.Modules[j].Position })
	return &s, nil
}

// FindSettings returns explicit if set, else $INKYCAL_SETTINGS_PATH, else the first existing SearchPaths entry.
func FindSettings(explicit string) (string, error) {
	candidates := []string{}
	if explicit != "" {
		candidates = append(candidates, explicit)
	} else {
		if env := os.Getenv("INKYCAL_SETTINGS_PATH"); env != "" {
			candidates = append(candidates, env)
		}
		candidates = append(candidates, SearchPaths...)
	}
	for _, p := range candidates {
		if st, err := os.Stat(p); err == nil && !st.IsDir() {
			return p, nil
		}
	}
	return "", fmt.Errorf("no settings.json found (tried %v)", candidates)
}
