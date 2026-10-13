package render

import "golang.org/x/image/font/sfnt"

// HasGlyphs reports whether the font can draw every rune of s.
func HasGlyphs(id FontID, s string) bool {
	var buf sfnt.Buffer
	f := loadFont(id)
	for _, r := range s {
		if i, err := f.GlyphIndex(&buf, r); err != nil || i == 0 {
			return false
		}
	}
	return true
}
