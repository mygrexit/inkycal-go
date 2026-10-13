package render

import (
	"testing"

	"golang.org/x/image/font/sfnt"
)

// Every embedded font must parse and cover the characters the modules write with it.
func TestFontsCoverTheirScripts(t *testing.T) {
	var buf sfnt.Buffer
	for _, c := range []struct {
		id   FontID
		text string
	}{
		{FontDefault, "Dienstag 13. Oktober 10:00–11:00 °äöüß"},
		{FontBold, "14° 8° Mi, 15. Okt"},
		{FontDisplay, "Oktober Dienstag η χελώνα το σπίτι ο σκύλος άέήίόύώϊΐ"},
		{FontEmoji, "🐢🐶🐱🦁🐘🍎🚗🌞⭐🏠⚽"},
	} {
		f := loadFont(c.id)
		for _, r := range c.text {
			if r == ' ' {
				continue
			}
			if i, err := f.GlyphIndex(&buf, r); err != nil || i == 0 {
				t.Errorf("font %d has no glyph for %q", c.id, r)
			}
		}
	}
}
