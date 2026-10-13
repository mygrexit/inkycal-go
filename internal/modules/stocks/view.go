package stocks

import (
	"math"
	"strconv"
	"strings"

	"inkycal/internal/i18n"
	"inkycal/internal/render"
)

// row is one ticker as drawn.
type row struct {
	Symbol string
	Name   string
	Price  string    // "" when the ticker could not be loaded
	Change string    // day change in percent
	Down   bool      // negative day: change in red
	Spark  []float64 // closes of the last month
}

// makeRow formats a fetched ticker; a failed one keeps its symbol and says so.
func makeRow(ticker string, r result, lang string) row {
	if r.Err != nil {
		return row{Symbol: ticker, Name: i18n.T(lang, "no_data")}
	}
	q := r.Quote
	pct := q.ChangePct()
	return row{
		Symbol: ticker,
		Name:   q.Name,
		Price:  priceLabel(q, lang),
		Change: changeLabel(pct, lang),
		Down:   math.Round(pct*100) < 0, // "–0,00 %" never shows up in red
		Spark:  q.Closes,
	}
}

// formatNum groups thousands and sets the decimal mark of the language:
// 1.234,56 (de) or 1,234.56 (en).
func formatNum(v float64, prec int, lang string) string {
	s := strconv.FormatFloat(math.Abs(v), 'f', prec, 64)
	intPart, frac, _ := strings.Cut(s, ".")
	group, dec := ",", "."
	if i18n.German(lang) {
		group, dec = ".", ","
	}
	var b strings.Builder
	if v < 0 && strings.Trim(s, "0.") != "" { // en dash: the Noto text faces lack U+2212
		b.WriteString("–")
	}
	for i, d := range intPart {
		if i > 0 && (len(intPart)-i)%3 == 0 {
			b.WriteString(group)
		}
		b.WriteRune(d)
	}
	if frac != "" {
		b.WriteString(dec + frac)
	}
	return b.String()
}

var currencySigns = map[string]string{"USD": "$", "EUR": "€", "GBP": "£", "JPY": "¥", "CNY": "¥", "INR": "₹", "KRW": "₩"}

// priceLabel is the price with its currency, the way each language writes it:
// "340,97 $" / "$340.97", "12,40 CHF" / "12.40 CHF". Index points carry none.
func priceLabel(q quote, lang string) string {
	n := formatNum(q.Price(), q.Precision, lang)
	if q.Index || q.Currency == "" {
		return n
	}
	sign, ok := currencySigns[strings.ToUpper(q.Currency)]
	switch {
	case ok && !i18n.German(lang):
		return sign + n
	case ok:
		return n + " " + sign
	}
	return n + " " + q.Currency
}

// changeLabel: "+1,29 %" (de) or "+1.29%" (en), always signed.
func changeLabel(pct float64, lang string) string {
	s := formatNum(pct, 2, lang)
	if !strings.HasPrefix(s, "–") {
		s = "+" + s
	}
	if i18n.German(lang) {
		return s + " %"
	}
	return s + "%"
}

// layout holds the column grid shared by all rows.
type layout struct {
	nameX, nameW    int // nameW 0: names dropped
	sparkX, sparkW  int // sparkW 0: sparkline dropped
	priceR, changeR int // right edges
}

// columns fits the grid into width w: symbol | name … | sparkline | price | change.
// When space runs out the name goes first, then the sparkline.
func columns(c *render.Canvas, rows []row, w, body int, sym, num, chg render.Style) layout {
	gap := body
	symW, priceW, changeW := 0, 0, 0
	for _, r := range rows {
		symW = max(symW, c.Advance(r.Symbol, sym))
		priceW = max(priceW, c.Advance(r.Price, num))
		changeW = max(changeW, c.Advance(r.Change, chg))
	}
	l := layout{changeR: w, priceR: w - changeW - gap}
	l.nameX = symW + gap
	numL := l.priceR - priceW
	l.sparkW = min(body*5, w/6)
	l.sparkX = numL - gap - l.sparkW
	l.nameW = l.sparkX - gap - l.nameX
	if l.nameW < body*4 {
		l.nameW = 0
		l.sparkX = max(l.nameX, numL-gap-l.sparkW)
		l.sparkW = min(l.sparkW, numL-gap-l.sparkX)
	}
	if l.sparkW < body*2 {
		l.sparkW = 0
	}
	return l
}

// drawRows stacks one ticker per line on a common grid; what does not fit is counted.
func drawRows(c *render.Canvas, rows []row, body int, lang string) {
	sym := render.Style{Font: render.FontDefault, Size: body}
	name := render.Style{Font: render.FontRegular, Size: body}
	num := render.Style{Font: render.FontDefault, Size: body}
	chg := render.Style{Font: render.FontRegular, Size: body}
	small := render.Style{Font: render.FontRegular, Size: body * 5 / 6}
	rowH := body * 8 / 5
	capH := c.CapHeight(sym)
	capacity := max(1, c.H/rowH)

	shown := rows
	if len(rows) > capacity {
		// Keep a line for "+ n more", unless that would leave no ticker at all.
		shown = rows[:max(1, capacity-1)]
	}
	l := columns(c, shown, c.W, body, sym, num, chg)
	for i, r := range shown {
		baseline := i*rowH + (rowH+capH)/2
		c.Text(0, baseline, c.Fit(r.Symbol, sym, l.nameX-body/2), sym, render.InkBlack)
		if l.nameW > 0 {
			c.Text(l.nameX, baseline, c.Fit(r.Name, name, l.nameW), name, render.InkBlack)
		} else if r.Price == "" {
			c.TextRight(l.changeR, baseline, r.Name, small, render.InkBlack)
		}
		if r.Price == "" {
			continue
		}
		if l.sparkW > 0 && len(r.Spark) > 1 {
			h := capH + capH/2
			sparkline(c, r.Spark, l.sparkX, baseline-capH-capH/4, l.sparkW, h, render.InkBlack)
		}
		c.TextRight(l.priceR, baseline, r.Price, num, render.InkBlack)
		ink := render.InkBlack
		if r.Down {
			ink = render.InkColour
		}
		c.TextRight(l.changeR, baseline, r.Change, chg, ink)
	}
	if rest := len(rows) - len(shown); rest > 0 && len(shown) < capacity {
		baseline := len(shown)*rowH + (rowH+capH)/2
		more := i18n.T(lang, "more", rest)
		c.Text(l.nameX, baseline, more, small, render.InkBlack)
	}
}

// sparkline draws vs scaled into the box x,y,w,h as a 2 px polyline.
func sparkline(c *render.Canvas, vs []float64, x, y, w, h int, ink render.Ink) {
	pts := sparkPoints(vs, w-2, h-2)
	for i := 1; i < len(pts); i++ {
		c.Line(x+pts[i-1][0], y+pts[i-1][1], x+pts[i][0], y+pts[i][1], 2, ink)
	}
}

// sparkPoints maps vs onto 0..w × 0..h, the highest value at the top.
// A flat series sits in the middle.
func sparkPoints(vs []float64, w, h int) [][2]int {
	lo, hi := vs[0], vs[0]
	for _, v := range vs {
		lo, hi = min(lo, v), max(hi, v)
	}
	pts := make([][2]int, len(vs))
	for i, v := range vs {
		px := 0
		if len(vs) > 1 {
			px = int(math.Round(float64(i) * float64(w) / float64(len(vs)-1)))
		}
		py := h / 2
		if hi > lo {
			py = int(math.Round((hi - v) / (hi - lo) * float64(h)))
		}
		pts[i] = [2]int{px, py}
	}
	return pts
}
