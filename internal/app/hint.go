package app

import (
	"inkycal/internal/i18n"
	"inkycal/internal/problem"
	"inkycal/internal/render"
)

// hintKeys are the panel texts for each kind; kinds missing here get "hint.error".
// Placement has none: a tile without a place on the page has nowhere to show a hint.
var hintKeys = map[problem.Kind]string{
	problem.UnknownModule: "hint.unknown",
	problem.Missing:       "hint.setup",
	problem.BadSetting:    "hint.badsetting",
	problem.Style:         "hint.style",
	problem.TooSmall:      "hint.small",
	problem.Padding:       "hint.padding",
	problem.Denied:        "hint.denied",
	problem.NotFound:      "hint.notfound",
	problem.Unavailable:   "hint.unavailable",
	problem.Offline:       "hint.offline",
	problem.Timeout:       "hint.timeout",
	problem.NotCalendar:   "hint.notcal",
	problem.BadResponse:   "hint.badresponse",
}

// errorHint says in one line what went wrong with a module and what to do about it.
func errorHint(module string, err error, lang string) string {
	key, ok := hintKeys[problem.Classify(err)]
	if !ok {
		key = "hint.error"
	}
	return i18n.T(lang, key, i18n.T(lang, module))
}

// drawHint writes the hint left-aligned and vertically centred into a failed module's area.
func drawHint(c *render.Canvas, hint string) {
	st := render.Style{Font: render.FontRegular, Size: max(12, min(22, c.H/3))}
	capH := c.CapHeight(st)
	c.Text(0, (c.H+capH)/2, c.Fit(hint, st, c.W), st, render.InkBlack)
}
