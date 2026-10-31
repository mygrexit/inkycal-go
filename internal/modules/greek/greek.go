// Package greek shows a Greek word of the day for children: the word with its
// article, a transliteration, the translation (German, else English) and, where one
// exists, a pictogram.
package greek

import (
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"image"
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
		return nil, fmt.Errorf("word list: %w", err)
	}
	if len(ws) == 0 {
		return nil, fmt.Errorf("word list is empty")
	}
	return ws, nil
}

type Module struct {
	common config.Common
	words  []word
}

// Def takes no module keys.
var Def = modules.Define(Meta, func(c config.Common, _ struct{}) (modules.Module, error) {
	ws, err := loadWords()
	if err != nil {
		return nil, err
	}
	return &Module{common: c, words: ws}, nil
})

// pick returns the word of now's calendar day; consecutive days walk through the list.
func pick(ws []word, now time.Time) word {
	day := modules.DayNumber(now)
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

// swatches: the three colours the panel can show are drawn instead of named.
var swatches = map[string]render.Ink{"rot": render.InkColour, "schwarz": render.InkBlack}

func (m *Module) Generate(_ context.Context, env *modules.Env) (*render.Canvas, error) {
	w, h := m.common.InnerSize()
	c := env.Canvas(w, h, render.FontDefault, int(m.common.FontSize))
	draw(c, pick(m.words, env.Now), m.common.Language)
	return c, nil
}

// draw sets the pictogram, the Greek word (article in red, as gender colour coding)
// with its transliteration, and the translation — German, else English.
func draw(c *render.Canvas, wd word, lang string) {
	meaning := wd.En
	if i18n.German(lang) || meaning == "" {
		meaning = wd.De
	}
	l := plan(c, wd, meaning)
	drawPic(c, wd, l.Pic)
	x := l.Word.At.X
	if l.Article != "" {
		x += c.Text(x, l.Word.At.Y, l.Article, l.Word.Style, render.InkColour) + c.Advance(" ", l.Word.Style)
	}
	c.Text(x, l.Word.At.Y, l.Word.Text, l.Word.Style, render.InkBlack)
	for _, t := range []textBox{l.Tr, l.Meaning} {
		c.Text(t.At.X, t.At.Y, t.Text, t.Style, render.InkBlack)
	}
}

// textBox is one line: baseline start, look, and the area it covers (accents above the
// capitals and descenders below included, roughly a quarter of the cap height each).
type textBox struct {
	Text  string
	At    image.Point
	Style render.Style
	Box   image.Rectangle
}

type layout struct {
	Pic               image.Rectangle // empty when the word has no pictogram
	Article           string          // drawn in colour before Word.Text
	Word, Tr, Meaning textBox
}

func line(c *render.Canvas, text string, st render.Style, x, base, width int) textBox {
	capH := c.CapHeight(st)
	return textBox{Text: text, At: image.Pt(x, base), Style: st,
		Box: image.Rect(x, base-capH*5/4, x+width, base+capH/4+1)}
}

func hasPic(wd word) bool {
	_, swatch := swatches[wd.De]
	return swatch || wd.De == "weiß" || isDigits(wd.Pic) || (wd.Pic != "" && render.HasGlyphs(render.FontEmoji, wd.Pic))
}

func isDigits(s string) bool {
	return s != "" && strings.IndexFunc(s, func(r rune) bool { return !unicode.IsDigit(r) }) < 0
}

// plan lays the word out as a row (pictogram, word over transliteration, translation on
// the right) in wide tiles and stacked and centred in tall or square ones.
func plan(c *render.Canvas, wd word, meaning string) layout {
	if c.W*10 >= c.H*22 {
		if l, ok := planRow(c, wd, meaning); ok {
			return l
		}
	}
	return planStack(c, wd, meaning)
}

func styles(c *render.Canvas, capGreek, capTr, capMeaning int) (greek, tr, mean render.Style) {
	return render.Style{Font: render.FontDisplay, Size: c.SizeForCap(render.FontDisplay, capGreek)},
		render.Style{Font: render.FontRegular, Size: c.SizeForCap(render.FontRegular, capTr)},
		render.Style{Font: render.FontDefault, Size: c.SizeForCap(render.FontDefault, capMeaning)}
}

func wordWidth(c *render.Canvas, article, rest string, st render.Style) int {
	w := c.Advance(rest, st)
	if article != "" {
		w += c.Advance(article, st) + c.Advance(" ", st)
	}
	return w
}

func planRow(c *render.Canvas, wd word, meaning string) (layout, bool) {
	w, h := c.W, c.H
	article, rest := splitArticle(wd.El)
	l := layout{Article: article}
	x0 := 0
	if hasPic(wd) {
		p := min(h, w/5)
		l.Pic = image.Rect(0, (h-p)/2, p, (h-p)/2+p)
		x0 = p + h/4
	}
	gap := h / 5
	for capG := h / 3; capG >= max(10, h/6); capG-- {
		greek, tr, mean := styles(c, capG, max(8, h/8), max(9, h/6))
		ww, mw := wordWidth(c, article, rest, greek), c.Advance(meaning, mean)
		if x0+ww+h/3 > w-mw {
			continue
		}
		capTr := c.CapHeight(tr)
		block := capG*5/4 + gap + capTr + capTr/4
		base := (h-block)/2 + capG*5/4
		l.Word = line(c, rest, greek, x0, base, ww)
		l.Tr = line(c, wd.Tr, tr, x0, base+gap+capTr, c.Advance(wd.Tr, tr))
		l.Meaning = line(c, meaning, mean, w-mw, base, mw)
		return l, true
	}
	return l, false
}

func planStack(c *render.Canvas, wd word, meaning string) layout {
	w, h := c.W, c.H
	article, rest := splitArticle(wd.El)
	pic := 0
	if hasPic(wd) {
		pic = min(w*55/100, h*35/100)
	}
	avail := w - w/12 // a little air at the sides
	var greek, tr, mean render.Style
	var capG, capTr, capM, total, gap int
	for capG = max(10, min(h/7, w/5)); ; capG-- {
		greek, tr, mean = styles(c, capG, max(8, capG*2/5), max(9, capG/2))
		capTr, capM = c.CapHeight(tr), c.CapHeight(mean)
		gap = max(4, capG/2)
		text := capG*5/4 + capG/4 + gap/2 + capTr*3/2 + gap + capM*3/2
		p := min(pic, h-text-gap)
		if p < capG*3/2 {
			p = 0
		}
		fits := wordWidth(c, article, rest, greek) <= avail && text <= h
		if fits || capG <= 10 {
			pic = p
			total = text
			if p > 0 {
				total += p + gap
			}
			break
		}
	}
	y := (h - total) / 2
	l := layout{Article: article}
	if pic > 0 {
		l.Pic = image.Rect((w-pic)/2, y, (w-pic)/2+pic, y+pic)
		y += pic + gap
	}
	centred := func(text string, st render.Style, base int) textBox {
		text = c.Fit(text, st, avail)
		tw := c.Advance(text, st)
		return line(c, text, st, (w-tw)/2, base, tw)
	}
	ww := min(w, wordWidth(c, article, rest, greek))
	y += capG * 5 / 4
	l.Word = line(c, rest, greek, (w-ww)/2, y, ww)
	y += capG/4 + gap/2 + capTr*5/4
	l.Tr = centred(wd.Tr, tr, y)
	y += capTr/4 + gap + capM*5/4
	l.Meaning = centred(meaning, mean, y)
	return l
}

// drawPic draws the pictogram into its box: colour words as swatches, numbers as
// figures, everything else as its emoji.
func drawPic(c *render.Canvas, wd word, r image.Rectangle) {
	if r.Empty() {
		return
	}
	p := r.Dx()
	cx, cy := r.Min.X+p/2, r.Min.Y+p/2
	switch ink, ok := swatches[wd.De]; {
	case ok:
		c.FillCircle(cx, cy, p*3/10, ink)
	case wd.De == "weiß":
		for t := range 3 {
			c.RoundedRect(r.Min.X+p/5+t, r.Min.Y+p/5+t, p*3/5-2*t, p*3/5-2*t, p*3/10, render.InkBlack)
		}
	case isDigits(wd.Pic):
		c.Icon(r.Min.X, r.Min.Y, p, p, wd.Pic, render.FontDisplay, render.InkBlack, 0.7)
	default:
		c.Icon(r.Min.X, r.Min.Y, p, p, wd.Pic, render.FontEmoji, render.InkBlack, 0.85)
	}
}
