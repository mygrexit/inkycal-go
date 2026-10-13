package timefmt

import "inkycal/internal/i18n"

type locale struct {
	months, monthsShort [12]string
	days, daysShort     [7]string // Monday first
	day, days2          string    // "a day", "%d days"
	week, weeks         string    // "a week", "%d weeks"
}

var locales = map[string]locale{
	"en": {
		months:      [12]string{"January", "February", "March", "April", "May", "June", "July", "August", "September", "October", "November", "December"},
		monthsShort: [12]string{"Jan", "Feb", "Mar", "Apr", "May", "Jun", "Jul", "Aug", "Sep", "Oct", "Nov", "Dec"},
		days:        [7]string{"Monday", "Tuesday", "Wednesday", "Thursday", "Friday", "Saturday", "Sunday"},
		daysShort:   [7]string{"Mon", "Tue", "Wed", "Thu", "Fri", "Sat", "Sun"},
		day:         "a day", days2: "%d days", week: "a week", weeks: "%d weeks",
	},
	"de": {
		months:      [12]string{"Januar", "Februar", "März", "April", "Mai", "Juni", "Juli", "August", "September", "Oktober", "November", "Dezember"},
		monthsShort: [12]string{"Jan", "Feb", "Mär", "Apr", "Mai", "Jun", "Jul", "Aug", "Sep", "Okt", "Nov", "Dez"},
		days:        [7]string{"Montag", "Dienstag", "Mittwoch", "Donnerstag", "Freitag", "Samstag", "Sonntag"},
		daysShort:   [7]string{"Mo", "Di", "Mi", "Do", "Fr", "Sa", "So"},
		day:         "ein Tag", days2: "%d Tage", week: "eine Woche", weeks: "%d Wochen",
	},
}

// lookup normalises "de_DE"/"de-de"/"DE" to "de" and falls back to English.
func lookup(lang string) locale {
	if l, ok := locales[i18n.Lang(lang)]; ok {
		return l
	}
	return locales["en"]
}
