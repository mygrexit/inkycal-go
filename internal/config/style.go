package config

import (
	"encoding/json"
	"fmt"
	"inkycal/internal/problem"
)

// Colors are a tile's own inks: "white", "black" or "colour" (the panel's red or yellow).
// "" inherits from the page theme.
type Colors struct {
	Paper  string `json:"paper"`
	Ink    string `json:"ink"`
	Accent string `json:"accent"`
}

// Style is the page theme ("theme") or a module's own look ("style" in its config).
type Style struct {
	Font        string            `json:"font"`         // "sans", "serif", "mono", "custom"; "" inherits
	CustomFonts map[string]string `json:"custom_fonts"` // role ("text", "strong", "bold", "display") → font file
	Scale       float64           `json:"scale"`        // font size factor 0.5–2; 0 inherits
	Colors      Colors            `json:"colors"`
}

var (
	fontNames = map[string]bool{"": true, "sans": true, "serif": true, "mono": true, "custom": true}
	inkNames  = map[string]bool{"": true, "white": true, "black": true, "colour": true}
	fontRoles = map[string]bool{"text": true, "strong": true, "bold": true, "display": true}
)

func (s *Style) validate() error {
	if s == nil {
		return nil
	}
	if !fontNames[s.Font] {
		return fmt.Errorf("font %q: want sans, serif, mono or custom", s.Font)
	}
	for role := range s.CustomFonts {
		if !fontRoles[role] {
			return fmt.Errorf("font role %q: want text, strong, bold or display", role)
		}
	}
	if s.Scale != 0 && (s.Scale < 0.5 || s.Scale > 2) {
		return fmt.Errorf("scale %g: want 0.5–2", s.Scale)
	}
	for _, ink := range []string{s.Colors.Paper, s.Colors.Ink, s.Colors.Accent} {
		if !inkNames[ink] {
			return fmt.Errorf("colours: %q: want white, black or colour", ink)
		}
	}
	return nil
}

// Effective is a module's look: its own style where set, else the page theme.
func Effective(page, module *Style) Style {
	var out Style
	for _, s := range []*Style{page, module} {
		if s == nil {
			continue
		}
		if s.Font != "" {
			out.Font = s.Font
			if s.Font != "custom" {
				out.CustomFonts = nil // a font pair replaces the inherited custom fonts
			}
		}
		if s.Scale != 0 {
			out.Scale = s.Scale
		}
		if s.Colors.Paper != "" {
			out.Colors.Paper = s.Colors.Paper
		}
		if s.Colors.Ink != "" {
			out.Colors.Ink = s.Colors.Ink
		}
		if s.Colors.Accent != "" {
			out.Colors.Accent = s.Colors.Accent
		}
		for role, f := range s.CustomFonts {
			if out.CustomFonts == nil {
				out.CustomFonts = map[string]string{}
			}
			out.CustomFonts[role] = f
		}
	}
	return out
}

// validateStyles checks the page theme and every module's style.
func validateStyles(s *Settings) error {
	if err := s.Theme.validate(); err != nil {
		return problem.Mark(problem.Style, fmt.Errorf("theme: %w", err))
	}
	for i, m := range s.Modules {
		var c struct {
			Style *Style `json:"style"`
		}
		if json.Unmarshal(m.Config, &c) != nil {
			continue // reported per module when it runs
		}
		if err := c.Style.validate(); err != nil {
			return problem.Mark(problem.Style, fmt.Errorf("module %d (%s): style: %w", i+1, m.Name, err))
		}
	}
	return nil
}
