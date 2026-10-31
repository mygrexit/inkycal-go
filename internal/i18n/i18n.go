// Package i18n holds the few fixed words the modules draw. Dates and weekday names
// come from timefmt, weather descriptions from OpenWeatherMap. Unknown languages and
// missing words fall back to English.
package i18n

import (
	"fmt"
	"strings"
)

// Lang normalises "de_DE", "de-de" or "DE" to "de".
func Lang(lang string) string {
	lang = strings.ToLower(lang)
	if i := strings.IndexAny(lang, "-_"); i >= 0 {
		lang = lang[:i]
	}
	return lang
}

// German reports whether lang writes numbers and dates the German way (1.234,5 / 13. Okt).
func German(lang string) bool { return Lang(lang) == "de" }

var words = map[string]map[string]string{
	"en": {
		"today":            "Today",
		"tomorrow":         "Tomorrow",
		"due.overdue":      "overdue",
		"due.today":        "today",
		"due.tomorrow":     "tomorrow",
		"more":             "+ %d more",
		"more_line":        "+ 1 more line",
		"more_lines":       "+ %d more lines",
		"span_days":        "(%d days)",
		"both":             "both",
		"no_events":        "No events in the next four weeks",
		"all_done":         "All done",
		"no_news":          "No news",
		"no_data":          "no data",
		"feels_like":       "feels like %s",
		"hint.denied":      "%s: access denied – check the API key in settings.json",
		"hint.offline":     "%s: no connection",
		"hint.setup":       "%s: not set up yet – fill it in with the builder",
		"hint.unknown":     "%s: this module no longer exists – remove the tile",
		"hint.notcal":      "%s: the address shows a web page, not a calendar – use the iCal address",
		"hint.small":       "%s: tile too small – make it bigger in the builder",
		"hint.style":       "%s: invalid style – choose it again in the builder",
		"hint.badsetting":  "%s: a setting has the wrong format – enter it again in the builder",
		"hint.padding":     "%s: inner margin larger than the tile – make it smaller in the builder",
		"hint.notfound":    "%s: address or file not found – check it in the builder",
		"hint.unavailable": "%s: service not available right now – next try at the next update",
		"hint.timeout":     "%s: service does not answer – next try at the next update",
		"hint.badresponse": "%s: service sent data it cannot read – next try at the next update",
		"hint.error":       "%s: error – see journalctl -u inkycal-go",
	},
	"de": {
		"today":            "Heute",
		"tomorrow":         "Morgen",
		"due.overdue":      "überfällig",
		"due.today":        "heute",
		"due.tomorrow":     "morgen",
		"more":             "+ %d weitere",
		"more_line":        "+ 1 weitere Zeile",
		"more_lines":       "+ %d weitere Zeilen",
		"span_days":        "(%d Tage)",
		"both":             "beide",
		"no_events":        "Keine Termine in den nächsten vier Wochen",
		"all_done":         "Alles erledigt",
		"no_news":          "Keine Meldungen",
		"no_data":          "keine Daten",
		"feels_like":       "gefühlt %s",
		"hint.denied":      "%s: Zugang abgelehnt – API-Schlüssel in settings.json prüfen",
		"hint.offline":     "%s: keine Verbindung",
		"hint.setup":       "%s: noch nicht eingerichtet – im Builder ausfüllen",
		"hint.unknown":     "%s: dieses Modul gibt es nicht mehr – Kachel entfernen",
		"hint.notcal":      "%s: Adresse liefert eine Webseite statt eines Kalenders – iCal-Adresse verwenden",
		"hint.small":       "%s: Kachel zu klein – im Builder vergrößern",
		"hint.style":       "%s: ungültiger Stil – im Builder neu wählen",
		"hint.badsetting":  "%s: eine Einstellung hat das falsche Format – im Builder neu eingeben",
		"hint.padding":     "%s: Innenabstand größer als die Kachel – im Builder verkleinern",
		"hint.notfound":    "%s: Adresse oder Datei nicht gefunden – im Builder prüfen",
		"hint.unavailable": "%s: Dienst gerade nicht erreichbar – nächster Versuch beim nächsten Aufwachen",
		"hint.timeout":     "%s: Dienst antwortet nicht – nächster Versuch beim nächsten Aufwachen",
		"hint.badresponse": "%s: Dienst lieferte unlesbare Daten – nächster Versuch beim nächsten Aufwachen",
		"hint.error":       "%s: Fehler – Details mit journalctl -u inkycal-go",
		// module names in error hints
		"Weather":  "Wetter",
		"Calendar": "Kalender",
		"Agenda":   "Termine",
		"Feeds":    "Nachrichten",
		"Greek":    "Griechisch",
	},
}

// T returns the word for key in lang, formatted with args when given.
// A key without any translation is returned as is (module names stay English).
func T(lang, key string, args ...any) string {
	s, ok := words[Lang(lang)][key]
	if !ok {
		s, ok = words["en"][key]
	}
	if !ok {
		s = key
	}
	if len(args) > 0 {
		return fmt.Sprintf(s, args...)
	}
	return s
}
