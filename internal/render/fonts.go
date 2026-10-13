package render

import (
	"embed"
	"fmt"
	"sync"

	"golang.org/x/image/font/opentype"
)

// FontID selects one of the embedded fonts.
type FontID int

const (
	FontDefault  FontID = iota // NotoSans SemiCondensed Medium (Inkycal's default)
	FontRegular                // NotoSans SemiCondensed
	FontWeather                // Weather Icons
	FontMaterial               // Material Icons
	FontBold                   // NotoSans SemiCondensed Bold
	FontDisplay                // Noto Serif Display SemiBold (Latin + Greek)
	FontEmoji                  // Noto Emoji, monochrome outlines
)

//go:embed fonts/*.ttf
var fontFS embed.FS

var fontFiles = map[FontID]string{
	FontDefault:  "fonts/NotoSans-SemiCondensedMedium.ttf",
	FontRegular:  "fonts/NotoSans-SemiCondensed.ttf",
	FontWeather:  "fonts/weathericons-regular-webfont.ttf",
	FontMaterial: "fonts/MaterialIcons.ttf",
	FontBold:     "fonts/NotoSans-SemiCondensedBold.ttf",
	FontDisplay:  "fonts/NotoSerifDisplay-SemiBold.ttf",
	FontEmoji:    "fonts/NotoEmoji.ttf",
}

var (
	fontMu    sync.Mutex
	fontCache = map[FontID]*opentype.Font{}
)

// loadFont parses an embedded font once. Parsed fonts are safe for concurrent use; faces are not.
func loadFont(id FontID) *opentype.Font {
	fontMu.Lock()
	defer fontMu.Unlock()
	if f, ok := fontCache[id]; ok {
		return f
	}
	data, err := fontFS.ReadFile(fontFiles[id])
	if err != nil {
		panic(fmt.Sprintf("embedded font %d missing: %v", id, err))
	}
	f, err := opentype.Parse(data)
	if err != nil {
		panic(fmt.Sprintf("embedded font %d broken: %v", id, err))
	}
	fontCache[id] = f
	return f
}
