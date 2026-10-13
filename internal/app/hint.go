package app

import (
	"strings"

	"inkycal/internal/i18n"
	"inkycal/internal/render"
)

// errorHint says in one line what went wrong with a module and what to do about it.
func errorHint(module string, err error, lang string) string {
	msg := err.Error()
	name := i18n.T(lang, module)
	switch {
	case strings.Contains(msg, "HTTP 401") || strings.Contains(msg, "HTTP 403"):
		return i18n.T(lang, "hint.denied", name)
	case strings.Contains(msg, "no such host") || strings.Contains(msg, "connection refused") ||
		strings.Contains(msg, "network is unreachable") || strings.Contains(msg, "i/o timeout"):
		return i18n.T(lang, "hint.offline", name)
	}
	return i18n.T(lang, "hint.error", name)
}

// drawHint writes the hint left-aligned and vertically centred into a failed module's area.
func drawHint(c *render.Canvas, hint string) {
	st := render.Style{Font: render.FontRegular, Size: max(12, min(22, c.H/3))}
	capH := c.CapHeight(st)
	c.Text(0, (c.H+capH)/2, c.Fit(hint, st, c.W), st, render.InkBlack)
}
