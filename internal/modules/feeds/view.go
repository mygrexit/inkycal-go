package feeds

import (
	"inkycal/internal/i18n"
	"inkycal/internal/render"
)

// block is one item as typeset.
type block struct {
	Head    []string
	Summary []string
	Source  string // "" when only one feed is shown
	Y       int    // baseline of the first headline line
}

type metrics struct {
	head, sum, small render.Style
	step, gap        int // baseline to baseline; whitespace between items
	capH, desc       int
	maxSummary       int
}

func newMetrics(c *render.Canvas, body int) metrics {
	m := metrics{
		head:  render.Style{Font: render.FontDefault, Size: body},
		sum:   render.Style{Font: render.FontRegular, Size: body},
		small: render.Style{Font: render.FontRegular, Size: body * 5 / 6},
		step:  body * 7 / 5,
		gap:   body * 4 / 5,
	}
	m.capH = c.CapHeight(m.head)
	_, m.desc = c.Metrics(m.sum)
	// short bands: one summary line keeps more headlines; tall modules get up to three
	m.maxSummary = min(3, max(1, c.H/m.step/5))
	return m
}

// multiSource reports whether the entries come from more than one feed.
func multiSource(items []item) bool {
	for _, it := range items[min(1, len(items)):] {
		if it.Source != items[0].Source {
			return true
		}
	}
	return false
}

// layout stacks items from the top until the next would not fit above limit.
// The last item may lose summary lines to fit, but never its headline.
// used counts the items consumed, empty ones included.
func layout(c *render.Canvas, items []item, m metrics, limit int) (out []block, used int) {
	sources := multiSource(items)
	y := m.capH
	for i, it := range items {
		b := block{Y: y}
		srcW := 0
		if sources && it.Source != "" {
			b.Source = c.Fit(it.Source, m.small, c.W/4)
			if b.Source != "" {
				srcW = c.Advance(b.Source, m.small) + m.step
			}
		}
		b.Head = c.Clip(c.WrapStyle(it.Title, m.head, c.W-srcW, c.W), 2, m.head, c.W)
		b.Summary = c.Clip(c.WrapStyle(it.Summary, m.sum, c.W, c.W), m.maxSummary, m.sum, c.W)
		n := len(b.Head) + len(b.Summary)
		if n == 0 {
			used = i + 1
			continue
		}
		bottom := func(lines int) int { return y + (lines-1)*m.step + m.desc }
		if bottom(n) > limit {
			fit := 0 // lines that still fit
			if y+m.desc <= limit {
				fit = (limit-m.desc-y)/m.step + 1
			}
			if fit < max(1, len(b.Head)) {
				break
			}
			sumLines := fit - len(b.Head)
			b.Summary = c.Clip(c.WrapStyle(it.Summary, m.sum, c.W, c.W), sumLines, m.sum, c.W)
			return append(out, b), i + 1
		}
		out = append(out, b)
		used = i + 1
		y += n*m.step + m.gap
	}
	return out, used
}

// draw sets the entries as a list: headline in medium, summary in the regular
// face below, items separated by whitespace, the source small on the right of
// the headline when several feeds are mixed. What does not fit is counted.
func draw(c *render.Canvas, items []item, body int, lang string) {
	m := newMetrics(c, body)
	if len(items) == 0 {
		c.Text(0, m.capH, i18n.T(lang, "no_news"), m.sum, render.InkBlack)
		return
	}
	blocks, used := layout(c, items, m, c.H)
	if used < len(items) {
		blocks, used = layout(c, items, m, c.H-m.step)
	}
	for _, b := range blocks {
		y := b.Y
		if b.Source != "" {
			c.TextRight(c.W, y, b.Source, m.small, render.InkBlack)
		}
		for _, l := range b.Head {
			c.Text(0, y, l, m.head, render.InkBlack)
			y += m.step
		}
		for _, l := range b.Summary {
			c.Text(0, y, l, m.sum, render.InkBlack)
			y += m.step
		}
	}
	rest := len(items) - used
	if rest <= 0 {
		return
	}
	more := i18n.T(lang, "more", rest)
	y := m.capH
	if n := len(blocks); n > 0 {
		last := blocks[n-1]
		y = last.Y + (len(last.Head)+len(last.Summary))*m.step
	}
	c.Text(0, min(y, c.H-m.desc), more, m.small, render.InkBlack)
}
