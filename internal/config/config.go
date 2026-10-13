// Package config reads Inkycal's settings.json (format compatible with the Python version).
package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"inkycal/internal/schedule"
	"os"
	"sort"
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
	Modules             []ModuleEntry `json:"modules"`
}

type ModuleEntry struct {
	Position int             `json:"position"`
	Name     string          `json:"name"`
	Config   json.RawMessage `json:"config"`
}

// Common holds the keys every module config shares.
type Common struct {
	Size     [2]int  `json:"size"`
	PaddingX FlexInt `json:"padding_x"`
	PaddingY FlexInt `json:"padding_y"`
	FontSize FlexInt `json:"fontsize"`
	Language string  `json:"language"`
}

// InnerSize is the drawable area: size minus padding on both sides.
func (c Common) InnerSize() (w, h int) {
	return c.Size[0] - 2*int(c.PaddingX), c.Size[1] - 2*int(c.PaddingY)
}

// ParseCommon extracts the shared module keys and applies defaults.
func ParseCommon(raw json.RawMessage) (Common, error) {
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
	w, h := c.InnerSize()
	if w <= 0 || h <= 0 {
		return c, fmt.Errorf("module size %v too small for padding %d/%d", c.Size, c.PaddingX, c.PaddingY)
	}
	return c, nil
}

// Load reads, defaults and validates a settings file.
func Load(path string) (*Settings, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var s Settings
	if err := json.Unmarshal(data, &s); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
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
		return nil, errors.New("model is missing")
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
