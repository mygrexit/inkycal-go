package xkcd

import (
	"image"
	"strings"

	"inkycal/internal/picture"
	"inkycal/internal/render"
)

// maxUpscale caps how far a comic is enlarged, relative to its normal (1x) size.
const maxUpscale = 2

// inkFrom: grey values below this become ink. Scaled line art turns thin strokes into
// light grey; a plain 50 % threshold would drop them, so the curve is shifted towards
// paper before the nearest-colour mapping.
const inkFrom = 200

// sheet is what the layout needs to know about a comic.
type sheet struct {
	W, H   int // image pixels
	Scale  int // 2 for the _2x image
	Title  string
	Number string // "xkcd 1234"
	Alt    string // "" when hidden
}

// layout is where everything goes. Text baselines are absolute canvas coordinates.
type layout struct {
	Body      int             // body font size
	Side      bool            // text column beside the comic instead of title above / alt below
	Comic     image.Rectangle // comic position and size on the canvas
	TextX     int             // left edge of title and alt text
	TextW     int
	TitleBase int
	Title     string // fitted to the space left by the number
	Number    string
	NumberAt  image.Point // baseline start; right-aligned to the column on the title row
	Kicker    bool        // number on its own line above the title
	AltLines  []string
	AltBase   int // first alt line
	AltStep   int
}

type styles struct {
	title, number, alt render.Style
}

func newStyles(body int) styles {
	return styles{
		title:  render.Style{Font: render.FontDefault, Size: body},
		number: render.Style{Font: render.FontRegular, Size: max(8, body*5/6)},
		alt:    render.Style{Font: render.FontRegular, Size: body},
	}
}

// fitComic is the largest size of the comic inside boxW×boxH, keeping its aspect
// ratio and enlarging at most maxUpscale times (same rounding as picture.Fit).
func fitComic(s sheet, boxW, boxH int) (int, int) {
	limW := s.W * maxUpscale / max(1, s.Scale)
	limH := s.H * maxUpscale / max(1, s.Scale)
	boxW, boxH = min(boxW, limW), min(boxH, limH)
	if boxW < 1 || boxH < 1 {
		return 0, 0
	}
	w, h := boxW, s.H*boxW/max(1, s.W)
	if h > boxH {
		w, h = s.W*boxH/max(1, s.H), boxH
	}
	return max(1, w), max(1, h)
}

// wrapAlt breaks the alt text to width and keeps at most maxLines lines, ending the
// last one with "…" when text was cut.
func wrapAlt(c *render.Canvas, alt string, st render.Style, width, maxLines int) []string {
	if alt == "" || maxLines <= 0 || width <= 0 {
		return nil
	}
	id, size := c.Font()
	c.SetFont(st.Font, st.Size)
	lines := c.Wrap(alt, width)
	c.SetFont(id, size)
	for i, l := range lines { // a single over-long word stays on its own line
		lines[i] = c.Fit(l, st, width)
	}
	if len(lines) > maxLines {
		rest := strings.Join(lines[maxLines-1:], " ")
		lines = lines[:maxLines]
		lines[maxLines-1] = c.Fit(rest, st, width)
	}
	return lines
}

// plan lays out the comic. The default stacks title, comic and alt text, centred as a
// block; the comic sits at the block's left edge, so text and comic share it. On wide,
// short modules the text moves into a column right of the comic when that lets the
// comic grow noticeably.
func plan(c *render.Canvas, body int, s sheet) layout {
	st := newStyles(body)
	stack := planStack(c, body, st, s)
	if side, ok := planSide(c, body, st, s); ok && side.Comic.Dx() > stack.Comic.Dx()*23/20 {
		return side
	}
	return stack
}

// textWidth is how wide the text wants to be: the title row, and the alt text up to a
// comfortable measure.
func textWidth(c *render.Canvas, body int, st styles, s sheet) int {
	need := c.Advance(s.Title, st.title) + body + c.Advance(s.Number, st.number)
	if s.Alt != "" {
		need = max(need, min(body*32, c.Advance(s.Alt, st.alt)))
	}
	return need
}

// finishTitle sets the number right-aligned on the title row, or drops it when the
// title needs the room.
func finishTitle(c *render.Canvas, body int, st styles, s sheet, l *layout) {
	l.Body = body
	l.Number = s.Number
	numW := c.Advance(s.Number, st.number)
	room := l.TextW - numW - body
	if room < body*3 {
		l.Number, room = "", l.TextW
	}
	l.NumberAt = image.Pt(l.TextX+l.TextW-numW, l.TitleBase)
	l.Title = c.Fit(s.Title, st.title, room)
}

func planStack(c *render.Canvas, body int, st styles, s sheet) layout {
	w, h := c.W, c.H
	capT := c.CapHeight(st.title)
	capA := c.CapHeight(st.alt)
	_, descA := c.Metrics(st.alt)
	head := capT + body*2/3 // title baseline to comic top
	gapA := body * 3 / 4    // comic bottom to first alt cap
	step := body * 7 / 5
	altH := func(n int) int {
		if n == 0 {
			return 0
		}
		return gapA + capA + (n-1)*step + descA
	}
	// The alt text may take at most half of the module.
	maxN := 0
	if s.Alt != "" {
		for altH(maxN+1) <= h/2-head {
			maxN++
		}
	}
	need := textWidth(c, body, st, s)
	blockW := func(cw int) int { return max(cw, min(w, need)) }

	// More alt lines make the comic smaller and narrower, which can add lines: grow
	// until it settles (monotone, so this ends).
	n := 0
	var cw, ch int
	var lines []string
	for {
		cw, ch = fitComic(s, w, h-head-altH(n))
		lines = wrapAlt(c, s.Alt, st.alt, blockW(cw), maxN)
		if len(lines) <= n {
			break
		}
		n = len(lines)
	}
	n = len(lines)
	bw := blockW(cw)
	top := (h - (head + ch + altH(n))) / 2
	x0 := (w - bw) / 2
	l := layout{
		Comic:     image.Rect(x0, top+head, x0+cw, top+head+ch),
		TextX:     x0,
		TextW:     bw,
		TitleBase: top + capT,
		AltLines:  lines,
		AltBase:   top + head + ch + gapA + capA,
		AltStep:   step,
	}
	finishTitle(c, body, st, s, &l)
	return l
}

func planSide(c *render.Canvas, body int, st styles, s sheet) (layout, bool) {
	w, h := c.W, c.H
	gap := body * 3 / 2
	minText := body * 14
	cw, ch := fitComic(s, w-gap-minText, h)
	if cw == 0 {
		return layout{}, false
	}
	tw := min(textWidth(c, body, st, s), w-cw-gap)
	if tw < minText {
		return layout{}, false
	}
	capT := c.CapHeight(st.title)
	capA := c.CapHeight(st.alt)
	_, descA := c.Metrics(st.alt)
	gapA := body * 3 / 4
	step := body * 7 / 5
	// A title that does not fit beside the number gets the number as a kicker above it.
	kicker := 0
	if c.Advance(s.Title, st.title)+body+c.Advance(s.Number, st.number) > tw {
		kicker = c.CapHeight(st.number) + body*3/5
	}
	head := kicker + capT
	maxN := 0
	if s.Alt != "" && h-head-gapA-capA-descA >= 0 {
		maxN = (h-head-gapA-capA-descA)/step + 1
	}
	lines := wrapAlt(c, s.Alt, st.alt, tw, maxN)
	textH := head
	if len(lines) > 0 {
		textH += gapA + capA + (len(lines)-1)*step + descA
	}
	top := (h - max(ch, textH)) / 2
	x0 := (w - (cw + gap + tw)) / 2
	l := layout{
		Side:      true,
		Comic:     image.Rect(x0, top, x0+cw, top+ch),
		TextX:     x0 + cw + gap,
		TextW:     tw,
		TitleBase: top + head,
		AltLines:  lines,
		AltBase:   top + head + gapA + capA,
		AltStep:   step,
	}
	if kicker > 0 {
		l.Body, l.Kicker = body, true
		l.Title = c.Fit(s.Title, st.title, tw)
		l.Number = s.Number
		l.NumberAt = image.Pt(l.TextX, top+c.CapHeight(st.number))
		return l, true
	}
	finishTitle(c, body, st, s, &l)
	return l, true
}

// darken maps grey levels so that everything below inkFrom ends up below the
// nearest-colour midpoint, keeping thin scaled strokes visible.
func darken(img *image.RGBA) {
	var lut [256]uint8
	for v := range lut {
		if v < inkFrom {
			lut[v] = uint8(v * 127 / inkFrom)
		} else {
			lut[v] = uint8(128 + (v-inkFrom)*127/(255-inkFrom))
		}
	}
	p := img.Pix
	for i := 0; i+3 < len(p); i += 4 {
		p[i], p[i+1], p[i+2] = lut[p[i]], lut[p[i+1]], lut[p[i+2]]
	}
}

// draw sets the comic without dithering (it is line art: error diffusion only adds
// speckle along the anti-aliased strokes) and the text around it.
func draw(c *render.Canvas, l layout, img image.Image, pal picture.Palette) {
	if l.Comic.Dx() > 0 && l.Comic.Dy() > 0 {
		fitted := picture.Flatten(picture.Fit(img, l.Comic.Dx(), l.Comic.Dy())) // scale first: _2x comics are big
		darken(fitted)
		picture.Draw(c, fitted, l.Comic.Min.X, l.Comic.Min.Y, pal, false)
	}
	st := newStyles(l.Body)
	c.Text(l.TextX, l.TitleBase, l.Title, st.title, render.InkBlack)
	if l.Number != "" {
		c.Text(l.NumberAt.X, l.NumberAt.Y, l.Number, st.number, render.InkColour)
	}
	for i, line := range l.AltLines {
		c.Text(l.TextX, l.AltBase+i*l.AltStep, line, st.alt, render.InkBlack)
	}
}
