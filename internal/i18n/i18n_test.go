package i18n

import (
	"strings"
	"testing"
)

func TestT(t *testing.T) {
	cases := []struct{ lang, key, want string }{
		{"de", "today", "Heute"},
		{"de_DE", "today", "Heute"},
		{"DE-at", "today", "Heute"},
		{"en", "today", "Today"},
		{"fr", "today", "Today"}, // unknown language → English
		{"de", "Weather", "Wetter"},
		{"en", "Weather", "Weather"}, // no entry → the key itself
	}
	for _, c := range cases {
		if got := T(c.lang, c.key); got != c.want {
			t.Errorf("T(%q, %q) = %q, want %q", c.lang, c.key, got, c.want)
		}
	}
	if got := T("de", "more", 3); got != "+ 3 weitere" {
		t.Errorf("formatted: %q", got)
	}
}

// Every German word needs an English counterpart, or English users get the key.
func TestEnglishCoversGerman(t *testing.T) {
	for k := range words["de"] {
		if _, ok := words["en"][k]; !ok && !isModuleName(k) {
			t.Errorf("no English text for %q", k)
		}
	}
}

func isModuleName(k string) bool { return k != "" && k[0] >= 'A' && k[0] <= 'Z' }

// The panel hints show in the module's language: each one needs a German text too.
func TestGermanCoversHints(t *testing.T) {
	for k := range words["en"] {
		if _, ok := words["de"][k]; !ok && strings.HasPrefix(k, "hint.") {
			t.Errorf("no German text for %q", k)
		}
	}
}
