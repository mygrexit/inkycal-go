package render

import (
	"errors"
	"fmt"
	"image"
	"log/slog"
	"maps"
	"os"
	"path/filepath"
	"sync"

	"golang.org/x/image/font"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/font/sfnt"
	"golang.org/x/image/math/fixed"
)

// FontSource is where a role's font comes from: an embedded file or one on disk.
type FontSource struct {
	Embedded string // file name in fonts/, e.g. "NotoSerif-Medium.ttf"
	File     string // custom font; relative paths start at Theme.Base
}

// Theme picks the fonts behind the roles modules draw with (FontDefault, FontRegular,
// FontBold, FontDisplay). Roles without an entry, and the icon fonts, stay as they are.
type Theme struct {
	Fonts map[FontID]FontSource
	Base  string // directory of settings.json, for custom font files
}

var pairs = map[string]map[FontID]FontSource{
	"serif": {
		FontDefault: {Embedded: "NotoSerif-Medium.ttf"},
		FontRegular: {Embedded: "NotoSerif-Regular.ttf"},
		FontBold:    {Embedded: "NotoSerif-Bold.ttf"},
		FontDisplay: {Embedded: "NotoSerifDisplay-SemiBold.ttf"},
	},
	"mono": {
		FontDefault: {Embedded: "NotoSansMono-Medium.ttf"},
		FontRegular: {Embedded: "NotoSansMono-Regular.ttf"},
		FontBold:    {Embedded: "NotoSansMono-Bold.ttf"},
		FontDisplay: {Embedded: "NotoSansMono-Bold.ttf"},
	},
}

// Pair returns the font set of a named pair: "sans" (the default look), "serif" or
// "mono". Unknown names give sans.
func Pair(name string) map[FontID]FontSource {
	if p, ok := pairs[name]; ok {
		out := make(map[FontID]FontSource, len(p))
		maps.Copy(out, p)
		return out
	}
	out := map[FontID]FontSource{}
	for _, id := range []FontID{FontDefault, FontRegular, FontBold, FontDisplay} {
		out[id] = defaultSource(id)
	}
	return out
}

func defaultSource(id FontID) FontSource {
	return FontSource{Embedded: filepath.Base(fontFiles[id])}
}

var (
	sourceMu    sync.Mutex
	sourceCache = map[string]*opentype.Font{}
	warned      = map[string]bool{}
)

const maxFontFile = 20 << 20

// loadSource parses a font once per source. Custom files are keyed by path, size and
// modification time and failures are not cached, so a long-running builder picks up a
// font that is uploaded or replaced later; each failure is logged once.
func loadSource(src FontSource, base string) (*opentype.Font, error) {
	key := "embed:" + src.Embedded
	path := ""
	if src.File != "" {
		path = src.File
		if !filepath.IsAbs(path) {
			path = filepath.Join(base, path)
		}
		st, err := os.Stat(path)
		switch {
		case err != nil:
		case !st.Mode().IsRegular():
			err = errors.New("not a regular file")
		case st.Size() > maxFontFile:
			err = errors.New("larger than 20 MiB")
		}
		if err != nil {
			return nil, fontFailed(path, err)
		}
		key = fmt.Sprintf("file:%s:%d:%d", path, st.Size(), st.ModTime().UnixNano())
	}
	sourceMu.Lock()
	defer sourceMu.Unlock()
	if f, ok := sourceCache[key]; ok {
		return f, nil
	}
	var data []byte
	var err error
	if path != "" {
		data, err = os.ReadFile(path)
	} else {
		data, err = fontFS.ReadFile("fonts/" + src.Embedded)
	}
	var f *opentype.Font
	if err == nil {
		f, err = opentype.Parse(data)
	}
	if err != nil {
		if path == "" {
			return nil, fmt.Errorf("font %s: %w", key, err)
		}
		return nil, fontFailedLocked(path, err)
	}
	sourceCache[key] = f
	return f, nil
}

func fontFailed(path string, err error) error {
	sourceMu.Lock()
	defer sourceMu.Unlock()
	return fontFailedLocked(path, err)
}

func fontFailedLocked(path string, err error) error {
	err = fmt.Errorf("font %s: %w", path, err)
	if msg := err.Error(); !warned[msg] {
		warned[msg] = true
		slog.Warn("font unusable, drawing with the default font", "err", err)
	}
	return err
}

// NewThemedCanvas is NewCanvas whose text uses the theme's fonts; nil means the defaults.
func NewThemedCanvas(w, h int, id FontID, size int, t *Theme) *Canvas {
	c := NewCanvas(w, h, id, size)
	c.theme = t
	return c
}

// Derive returns a blank canvas of another size with the same theme and font.
func (c *Canvas) Derive(w, h int) *Canvas {
	return NewThemedCanvas(w, h, c.fontID, c.fontSize, c.theme)
}

// themedFont returns the font the theme puts behind a role, or nil for the default.
func (c *Canvas) themedFont(id FontID) *opentype.Font {
	if c.theme == nil {
		return nil
	}
	src, ok := c.theme.Fonts[id]
	if !ok || (src.File == "" && src == defaultSource(id)) {
		return nil
	}
	f, err := loadSource(src, c.theme.Base)
	if err != nil {
		return nil
	}
	return f
}

func fontHas(f *opentype.Font, s string) bool {
	var buf sfnt.Buffer
	for _, r := range s {
		if r == ' ' {
			continue
		}
		if i, err := f.GlyphIndex(&buf, r); err != nil || i == 0 {
			return false
		}
	}
	return true
}

// fallbackFace draws with primary and takes every character it lacks from fallback,
// so a custom font without "ä" or Greek never shows empty boxes.
type fallbackFace struct {
	primary, fallback font.Face
	font              *opentype.Font
	buf               sfnt.Buffer
	known             map[rune]bool
}

func (f *fallbackFace) pick(r rune) font.Face {
	has, ok := f.known[r]
	if !ok {
		i, err := f.font.GlyphIndex(&f.buf, r)
		has = err == nil && i != 0
		f.known[r] = has
	}
	if has {
		return f.primary
	}
	return f.fallback
}

func (f *fallbackFace) Close() error {
	f.fallback.Close()
	return f.primary.Close()
}

func (f *fallbackFace) Glyph(dot fixed.Point26_6, r rune) (image.Rectangle, image.Image, image.Point, fixed.Int26_6, bool) {
	return f.pick(r).Glyph(dot, r)
}

func (f *fallbackFace) GlyphBounds(r rune) (fixed.Rectangle26_6, fixed.Int26_6, bool) {
	return f.pick(r).GlyphBounds(r)
}

func (f *fallbackFace) GlyphAdvance(r rune) (fixed.Int26_6, bool) {
	return f.pick(r).GlyphAdvance(r)
}

func (f *fallbackFace) Kern(r0, r1 rune) fixed.Int26_6 {
	if a := f.pick(r0); a == f.pick(r1) {
		return a.Kern(r0, r1)
	}
	return 0
}

func (f *fallbackFace) Metrics() font.Metrics { return f.primary.Metrics() }
