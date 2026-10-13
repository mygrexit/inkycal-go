package app

import (
	"strings"

	"inkycal/internal/render"
)

var moduleNamesDE = map[string]string{"Weather": "Wetter", "Calendar": "Kalender", "Agenda": "Termine", "Feeds": "Nachrichten", "Greek": "Griechisch"}

// errorHint says in one line what went wrong with a module and what to do about it.
func errorHint(module string, err error, lang string) string {
	msg := err.Error()
	de := lang == "de"
	name := module
	if n, ok := moduleNamesDE[module]; ok && de {
		name = n
	}
	switch {
	case strings.Contains(msg, "HTTP 401") || strings.Contains(msg, "HTTP 403"):
		if de {
			return name + ": Zugang abgelehnt – API-Schlüssel in settings.json prüfen"
		}
		return name + ": access denied – check the API key in settings.json"
	case strings.Contains(msg, "no such host") || strings.Contains(msg, "connection refused") ||
		strings.Contains(msg, "network is unreachable") || strings.Contains(msg, "i/o timeout"):
		if de {
			return name + ": keine Verbindung"
		}
		return name + ": no connection"
	}
	if de {
		return name + ": Fehler – Details mit journalctl -u inkycal-go"
	}
	return name + ": error – see journalctl -u inkycal-go"
}

// drawHint writes the hint left-aligned and vertically centred into a failed module's area.
func drawHint(c *render.Canvas, hint string) {
	st := render.Style{Font: render.FontRegular, Size: max(12, min(22, c.H/3))}
	capH := c.CapHeight(st)
	c.Text(0, (c.H+capH)/2, c.Fit(hint, st, c.W), st, render.InkBlack)
}
