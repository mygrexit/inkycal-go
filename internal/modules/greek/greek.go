// Package greek shows a Greek word of the day for children: the word with its
// article, a transliteration, the translation (German, else English) and, where one
// exists, a pictogram.
package greek

import (
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"strings"
	"time"
	"unicode"

	"inkycal/internal/config"
	"inkycal/internal/i18n"
	"inkycal/internal/modules"
	"inkycal/internal/render"
)

//go:embed words.json
var wordsJSON []byte

type word struct {
	El  string `json:"el"`  // Greek with article
	Tr  string `json:"tr"`  // transliteration
	De  string `json:"de"`  // German with article
	En  string `json:"en"`  // English
	Cat string `json:"cat"` // category
	Pic string `json:"pic"` // emoji, digits or ""
}

func loadWords() ([]word, error) {
	var ws []word
	if err := json.Unmarshal(wordsJSON, &ws); err != nil {
		return nil, fmt.Errorf("greek: word list: %w", err)
	}
	if len(ws) == 0 {
		return nil, fmt.Errorf("greek: word list is empty")
	}
	return ws, nil
}

type Module struct {
	common config.Common
	words  []word
}

func New(c config.Common, _ json.RawMessage) (modules.Module, error) {
	if err := modules.CheckMinSize(c, Meta); err != nil {
		return nil, err
	}
	ws, err := loadWords()
	if err != nil {
		return nil, err
	}
	return &Module{common: c, words: ws}, nil
}

// pick returns the word of now's calendar day; consecutive days walk through the list.
func pick(ws []word, now time.Time) word {
	y, m, d := now.Date()
	day := time.Date(y, m, d, 12, 0, 0, 0, time.UTC).Unix() / 86400
	return ws[int(day%int64(len(ws)))]
}

// splitArticle separates the definite article (ο, η, το) from the word.
func splitArticle(el string) (article, rest string) {
	for _, a := range []string{"ο ", "η ", "το "} {
		if r, ok := strings.CutPrefix(el, a); ok {
			return strings.TrimSpace(a), r
		}
	}
	return "", el
}

func sizeForCap(c *render.Canvas, font render.FontID, capPx int) int {
	size := 8
	for size < 400 && c.CapHeight(render.Style{Font: font, Size: size + 1}) <= capPx {
		size++
	}
	return size
}

// swatches: the three colours the panel can show are drawn instead of named.
var swatches = map[string]render.Ink{"rot": render.InkColour, "schwarz": render.InkBlack}

func (m *Module) Generate(_ context.Context, env *modules.Env) (*render.Canvas, error) {
	w, h := m.common.InnerSize()
	c := render.NewCanvas(w, h, render.FontDefault, int(m.common.FontSize))
	draw(c, pick(m.words, env.Now), m.common.Language)
	return c, nil
}

// draw sets the pictogram on the left, the Greek word (article in red, as gender
// colour coding) with its transliteration, and the translation on the right.
//
//	[🐢]  η χελώνα                     die Schildkröte
//	      i chelóna
func draw(c *render.Canvas, wd word, lang string) {
	w, h := c.W, c.H
	x0 := 0
	pic := h
	switch ink, ok := swatches[wd.De]; {
	case ok:
		c.FillCircle(pic/2, h/2, pic*3/10, ink)
		x0 = pic + h/4
	case wd.De == "weiß":
		for t := 0; t < 3; t++ {
			c.RoundedRect(pic/5+t, h/5+t, pic*3/5-2*t, pic*3/5-2*t, pic*3/10, render.InkBlack)
		}
		x0 = pic + h/4
	case wd.Pic != "" && strings.IndexFunc(wd.Pic, func(r rune) bool { return !unicode.IsDigit(r) }) < 0:
		c.Icon(0, 0, pic, h, wd.Pic, render.FontDisplay, render.InkBlack, 0.7)
		x0 = pic + h/4
	case wd.Pic != "" && render.HasGlyphs(render.FontEmoji, wd.Pic):
		c.Icon(0, 0, pic, h, wd.Pic, render.FontEmoji, render.InkBlack, 0.85)
		x0 = pic + h/4
	}

	capGreek := h / 3
	capTr := h / 8
	gap := h / 5
	meaning := wd.En
	if i18n.German(lang) || meaning == "" {
		meaning = wd.De
	}
	trans := render.Style{Font: render.FontDefault, Size: sizeForCap(c, render.FontDefault, h/6)}
	transW := c.Advance(meaning, trans)
	article, rest := splitArticle(wd.El)

	// Shrink the Greek word until it fits next to the translation.
	var greek render.Style
	space := 0
	for capPx := capGreek; capPx > 10; capPx-- {
		greek = render.Style{Font: render.FontDisplay, Size: sizeForCap(c, render.FontDisplay, capPx)}
		space = c.Advance(" ", greek)
		width := c.Advance(rest, greek)
		if article != "" {
			width += c.Advance(article, greek) + space
		}
		if x0+width+h/3 <= w-transW {
			break
		}
	}
	tr := render.Style{Font: render.FontRegular, Size: sizeForCap(c, render.FontRegular, capTr)}
	capG := c.CapHeight(greek)
	block := capG + gap + c.CapHeight(tr)
	baseline := (h-block)/2 + capG

	x := x0
	if article != "" {
		x += c.Text(x, baseline, article, greek, render.InkColour) + space
	}
	c.Text(x, baseline, rest, greek, render.InkBlack)
	c.Text(x0, baseline+gap+c.CapHeight(tr), wd.Tr, tr, render.InkBlack)
	c.TextRight(w, baseline, meaning, trans, render.InkBlack)
}
