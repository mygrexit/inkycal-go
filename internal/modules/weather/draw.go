package weather

import (
	"inkycal/internal/render"
	"inkycal/internal/timefmt"
)

// drawDay lays out the today band like a tear-off calendar leaf: the day's date
// on the left, the day's weather in pictograms on the right, a timeline below it.
//
//	13          ☁   14°     ≋ ▂▄▆
//	Dienstag         8°     ☀↑ 7:54
//	Oktober                 ☀↓ 18:48
//	                6  8   11   14   17   20
//	             ☁     ☁   ☂    ☂    ☁    ☁
//	             8°   11°  14°  13°  10°   9°
func drawDay(c *render.Canvas, v dayView, lang string) {
	w, h := c.W, c.H
	// The design is 904×250. A relatively narrower area gets the band scaled to its
	// width and centred vertically instead of overlapping columns.
	if w*250 < h*904 {
		band := c.Derive(w, max(1, w*250/904)) // same theme fonts
		band.SetFont(render.FontDefault, 14)
		drawDay(band, v, lang)
		c.Paste(band, 0, (h-band.H)/2)
		return
	}
	u := func(f float64) int { return int(f * float64(h) / 250) } // design grid is 250 px tall

	// Date block.
	num := render.Style{Font: render.FontDisplay, Size: c.SizeForCap(render.FontDisplay, u(118))}
	c.Text(-u(4), u(118), timefmt.Format(v.Date, "D", lang), num, render.InkColour)
	weekday := render.Style{Font: render.FontDisplay, Size: u(38)}
	c.Text(0, u(178), timefmt.Format(v.Date, "dddd", lang), weekday, render.InkBlack)
	month := render.Style{Font: render.FontRegular, Size: u(27)}
	c.Text(0, u(222), timefmt.Format(v.Date, "MMMM YYYY", lang), month, render.InkBlack)

	// Weather block.
	x0 := u(300)
	right := w
	c.Icon(x0, 0, u(124), u(124), iconGlyph(v.Icon), render.FontWeather, render.InkBlack, 0.95)

	hi := render.Style{Font: render.FontBold, Size: c.SizeForCap(render.FontBold, u(50))}
	lo := render.Style{Font: render.FontRegular, Size: c.SizeForCap(render.FontRegular, u(30))}
	xt := x0 + u(146)
	c.Text(xt, u(62), v.Hi, hi, render.InkBlack)
	c.Text(xt, u(112), v.Lo, lo, render.InkBlack)

	// Wind gauge and sun times, right-aligned column.
	small := render.Style{Font: render.FontDefault, Size: c.SizeForCap(render.FontDefault, u(20))}
	colX := right - u(170)
	iconW := u(44)
	drawWind(c, colX, u(6), iconW, u(36), v.Wind)
	c.Icon(colX, u(46), iconW, u(36), glyphSunrise, render.FontWeather, render.InkBlack, 0.95)
	c.Text(colX+iconW+u(14), u(74), v.Sunrise, small, render.InkBlack)
	c.Icon(colX, u(88), iconW, u(36), glyphSunset, render.FontWeather, render.InkBlack, 0.95)
	c.Text(colX+iconW+u(14), u(116), v.Sunset, small, render.InkBlack)

	// Timeline: the observation plus the coming 3-hour slots of today.
	label := render.Style{Font: render.FontRegular, Size: c.SizeForCap(render.FontRegular, u(15))}
	temp := render.Style{Font: render.FontDefault, Size: c.SizeForCap(render.FontDefault, u(19))}
	colW := (right - x0) / maxSlots
	for i, s := range v.Slots {
		cx := x0 + i*colW + colW/2
		ink := render.InkBlack
		c.TextCenter(cx, u(164), s.Label, label, ink)
		c.Icon(cx-u(28), u(172), u(56), u(46), iconGlyph(s.Icon), render.FontWeather, ink, 0.9)
		c.TextCenter(cx, u(246), s.Temp, temp, ink)
	}
}

// drawWind draws the wind glyph and a three-step gauge; a strong-wind day is red.
func drawWind(c *render.Canvas, x, y, iconW, h, level int) {
	c.Icon(x, y, iconW, h, glyphWind, render.FontWeather, render.InkBlack, 0.95)
	ink := render.InkBlack
	if level >= 3 {
		ink = render.InkColour
	}
	barW, gap := max(h/5, 4), max(h/8, 3)
	bx := x + iconW + h/3
	for i := 0; i < 3; i++ {
		bh := h * (i + 2) / 5
		by := y + h - bh - h/10
		if i < level {
			c.FillRect(bx, by, barW, bh, ink)
		} else {
			c.RoundedRect(bx, by, barW, bh, 0, render.InkBlack)
		}
		bx += barW + gap
	}
}
