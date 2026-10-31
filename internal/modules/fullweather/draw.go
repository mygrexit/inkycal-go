package fullweather

import (
	"math"
	"strconv"
	"time"

	"inkycal/internal/forecast"
	"inkycal/internal/render"
	"inkycal/internal/timefmt"
)

// Weather Icons glyphs beyond the ones the weather package exports.
const (
	glyphRaindrop = ""
	glyphHumidity = ""
	glyphUV       = "" // wi-hot
)

// Design heights (px at scale 1, i.e. a 904 px wide module).
const (
	headerH  = 230.0 // tear-off date and current conditions
	gapH     = 18.0  // whitespace above and below a hairline
	chartMin = 220.0
	daysH    = 196.0 // one row of day columns
	dateRowH = 26.0  // the date line under the weekday, dropped when space is short
)

// draw lays out the view in three bands separated by hairlines:
//
//	13          ☁  14°        ≋ 3–7 m/s
//	Dienstag       gefühlt 12°  ⸙ 0,4 mm
//	Oktober 2026  Leichter Regen   ◌ 82 %
//	──────────────────────────────────────────
//	15° ┆    ╱‾‾╲          ┆      ╱‾╲         5 mm
//	 9° ●───╱    ╲____     ┆ ____╱   ╲___
//	      ▒▒          ▒▒▒  ┆     ▒▒
//	     12   18        Mi  6   12   18
//	──────────────────────────────────────────
//	Heute   Mi     Do     Fr     Sa
//	  ☂     ☁      ☀      ☀      ☁
//	14° 8° 13° 7° ...
//
// Shorter areas drop the chart, then the day columns; everything scales with the
// width (904 px = 1) or, if that does not fit, with the height.
func draw(c *render.Canvas, v view) {
	w, h := float64(c.W), float64(c.H)
	u := w / 904
	switch {
	case h >= 520*u:
		s := math.Min(u, h/(headerH+4*gapH+chartMin+daysH))
		drawHeader(c, v, 0, s)
		y := px(headerH+gapH, s)
		c.HLine(0, c.W-1, y, render.InkBlack)
		y += px(gapH, s)
		rest := c.H - y
		dayMin := daysH
		if s < u { // height-limited: the day row gives up its date line first
			dayMin -= dateRowH
		}
		dh := clamp(int(float64(rest)*0.36), px(dayMin, s), px(1.5*daysH, s))
		ch := rest - dh - 2*px(gapH, s)
		drawChart(c, v, 0, y, c.W, ch, s)
		y += ch + px(gapH, s)
		c.HLine(0, c.W-1, y, render.InkBlack)
		y += px(gapH, s)
		drawDays(c, v, 0, y, c.W, c.H-y, s)
	case h >= 330*u:
		s := math.Min(u, h/(headerH+2*gapH+daysH))
		drawHeader(c, v, 0, s)
		y := px(headerH+gapH, s)
		c.HLine(0, c.W-1, y, render.InkBlack)
		y += px(gapH, s)
		drawDays(c, v, 0, y, c.W, c.H-y, s)
	default:
		s := math.Min(u, h/(headerH+6))
		drawHeader(c, v, (c.H-px(headerH, s))/2, s)
	}
}

func px(f, s float64) int { return int(math.Round(f * s)) }

func clamp(v, lo, hi int) int { return max(lo, min(v, hi)) }

func capStyle(c *render.Canvas, font render.FontID, capPx int) render.Style {
	return render.Style{Font: font, Size: c.SizeForCap(font, max(capPx, 4))}
}

// drawHeader draws the tear-off date like the weather band, the current conditions
// next to it and a column of details on the right.
func drawHeader(c *render.Canvas, v view, y0 int, s float64) {
	u := func(f float64) int { return px(f, s) }
	lang := v.Lang

	num := capStyle(c, render.FontDisplay, u(118))
	c.Text(-u(4), y0+u(118), timefmt.Format(v.Date, "D", lang), num, render.InkColour)
	weekday := render.Style{Font: render.FontDisplay, Size: max(u(38), 6)}
	c.Text(0, y0+u(178), timefmt.Format(v.Date, "dddd", lang), weekday, render.InkBlack)
	month := render.Style{Font: render.FontRegular, Size: max(u(27), 6)}
	c.Text(0, y0+u(222), timefmt.Format(v.Date, "MMMM YYYY", lang), month, render.InkBlack)

	// Details column, right-aligned block: wind, rain, humidity, UV.
	colX := c.W - u(210)
	small := capStyle(c, render.FontDefault, u(19))
	type row struct {
		glyph, text string
		ink         render.Ink
	}
	rows := []row{
		{forecast.GlyphWind, v.Wind, inkIf(v.WindStrong)},
		{glyphRaindrop, v.Precip, render.InkBlack},
		{glyphHumidity, v.Humidity, render.InkBlack},
	}
	if v.UV != "" {
		rows = append(rows, row{glyphUV, v.UV, inkIf(v.UVHigh)})
	}
	for i, r := range rows {
		base := y0 + u(34+44*float64(i))
		c.Icon(colX, base-u(30), u(40), u(38), r.glyph, render.FontWeather, render.InkBlack, 0.9)
		tx := colX + u(54)
		c.Text(tx, base, c.Fit(r.text, small, c.W-tx), small, r.ink)
	}

	// Current conditions.
	x0 := u(300)
	c.Icon(x0, y0, u(124), u(124), forecast.Glyph(v.Icon), render.FontWeather, render.InkBlack, 0.95)
	xt := x0 + u(146)
	big := capStyle(c, render.FontBold, u(50))
	c.Text(xt, y0+u(62), v.Temp, big, render.InkBlack)
	feels := capStyle(c, render.FontRegular, u(19))
	c.Text(xt, y0+u(104), c.Fit(v.FeelsLike, feels, colX-u(16)-xt), feels, render.InkBlack)

	desc := capStyle(c, render.FontDefault, u(21))
	c.Text(x0, y0+u(178), c.Fit(v.Description, desc, colX-u(16)-x0), desc, render.InkBlack)
	if v.Sunrise != "" {
		sun := capStyle(c, render.FontRegular, u(18))
		base := y0 + u(222)
		x := x0
		for _, p := range [][2]string{{forecast.GlyphSunrise, v.Sunrise}, {forecast.GlyphSunset, v.Sunset}} {
			c.Icon(x, base-u(28), u(36), u(34), p[0], render.FontWeather, render.InkBlack, 0.95)
			x += u(44)
			x += c.Text(x, base, p[1], sun, render.InkBlack) + u(26)
		}
	}
}

// bodyCap is the cap height of the configured body font size: the floor for small
// labels in the chart and day row, so a narrow module stays readable.
func bodyCap(c *render.Canvas) int {
	_, size := c.Font()
	return c.CapHeight(render.Style{Font: render.FontRegular, Size: size})
}

func inkIf(red bool) render.Ink {
	if red {
		return render.InkColour
	}
	return render.InkBlack
}

// tempStep picks the grid step: upstream's 3° (5 °F), coarser if the lines would
// crowd, finer if there would be fewer than three intervals.
func tempStep(lo, hi float64, fahrenheit bool, maxTicks int) float64 {
	steps := []float64{1, 2, 3, 5, 10, 20, 50}
	i := 2
	if fahrenheit {
		i = 3
	}
	n := func(st float64) int { return int(math.Ceil(hi/st) - math.Floor(lo/st)) }
	for i < len(steps)-1 && n(steps[i]) > maxTicks {
		i++
	}
	for i > 0 && n(steps[i]) < 3 && n(steps[i-1]) <= maxTicks {
		i--
	}
	return steps[i]
}

// precipScale is the top of the precipitation axis in mm per 3 hours: at least
// 5 mm, so a drizzle stays a low bar.
func precipScale(maxMM float64) float64 {
	for _, s := range []float64{5, 10, 20, 50, 100} {
		if maxMM <= s {
			return s
		}
	}
	return math.Ceil(maxMM/100) * 100
}

// drawChart plots the temperature (line) and precipitation (light bars) from now
// over the coming hours; the observation is the red dot at the left end.
func drawChart(c *render.Canvas, v view, x, y, w, h int, s float64) {
	u := func(f float64) int { return px(f, s) }
	samples := append([]sample{v.Now}, v.Samples...)
	if len(samples) < 2 || h < u(80) {
		return
	}
	t0, t1 := samples[0].Time, samples[len(samples)-1].Time

	bc := bodyCap(c)
	tick := capStyle(c, render.FontRegular, max(u(15), bc*4/5))
	dayLabel := capStyle(c, render.FontDefault, max(u(17), bc))
	tickCap := c.CapHeight(tick)

	lo, hi, rainMax := math.Inf(1), math.Inf(-1), 0.0
	for _, p := range samples {
		lo, hi, rainMax = math.Min(lo, p.Temp), math.Max(hi, p.Temp), math.Max(rainMax, p.Precip)
	}
	axisH := u(12) + tickCap + u(14) + c.CapHeight(dayLabel) + u(4)
	top := y + u(12)
	if v.MinMax {
		top += tickCap + u(10) // room for the max label above the curve
	}
	bottom := y + h - axisH
	if v.MinMax {
		bottom -= tickCap + u(10)
	}
	ph := bottom - top
	if ph < u(40) { // a big font in a small module leaves no room for a readable chart
		return
	}
	step := tempStep(lo, hi, v.Fahrenheit, max(2, ph/u(44)))
	tlo, thi := math.Floor(lo/step)*step, math.Ceil(hi/step)*step
	if thi-tlo < step {
		thi += step
	}

	deg := func(t float64) string { return strconv.Itoa(int(math.Round(t))) + "°" }
	labW := 0
	for t := tlo; t <= thi+step/2; t += step {
		labW = max(labW, c.Advance(deg(t), tick))
	}
	pmax := precipScale(rainMax)
	pLabel := strconv.Itoa(int(pmax)) + " mm"
	left := x + labW + u(12)
	right := x + w - c.Advance(pLabel, tick) - u(12)
	pw := right - left
	if pw < u(100) {
		return
	}

	xOf := func(t time.Time) int {
		return left + int(float64(pw)*float64(t.Sub(t0))/float64(t1.Sub(t0)))
	}
	yOf := func(t float64) int { return bottom - int(float64(ph)*(t-tlo)/(thi-tlo)) }

	// Temperature grid: dotted lines with labels on the left.
	for t := tlo; t <= thi+step/2; t += step {
		yy := yOf(t)
		c.TextRight(left-u(12), yy+tickCap/2, deg(t), tick, render.InkBlack)
		if t > tlo {
			dotted(c, left, right, yy, max(4, u(5)))
		}
	}
	c.TextRight(x+w, top+tickCap/2, pLabel, tick, render.InkBlack)

	// Precipitation: light bars over the three hours each amount covers.
	for _, p := range samples[1:] {
		if p.Precip <= 0 {
			continue
		}
		bx0, bx1 := xOf(p.Time.Add(-3*time.Hour)), xOf(p.Time)
		inset := max(1, (bx1-bx0)/8)
		bx0, bx1 = max(left, bx0+inset), bx1-inset
		by := bottom - max(2, int(float64(ph)*math.Min(p.Precip, pmax)/pmax))
		shade(c, bx0, by, bx1, bottom)
	}

	// Days: dotted separators at midnight, weekday names centred in their span,
	// hour ticks every three hours with labels every six.
	rowHour := bottom + u(12) + tickCap
	rowDay := rowHour + u(14) + c.CapHeight(dayLabel)
	hourFmt := "H"
	if v.Hour12 {
		hourFmt = "h a"
	}
	lastLabelEnd := math.MinInt
	// Start at a local full hour: Truncate works on absolute time and would put
	// ticks at :30 in zones like Asia/Kolkata.
	y0, m0, d0 := t0.Date()
	first := time.Date(y0, m0, d0, t0.Hour(), 0, 0, 0, t0.Location()).Add(time.Hour)
	for t := first; !t.After(t1); t = t.Add(time.Hour) {
		hr := t.Hour()
		if hr%3 != 0 {
			continue
		}
		xx := xOf(t)
		if hr == 0 {
			dottedV(c, xx, top, rowDay, max(4, u(5)))
			continue
		}
		c.VLine(xx, bottom, bottom+u(5), render.InkBlack)
		if hr%6 == 0 {
			lbl := timefmt.Format(t, hourFmt, "en")
			lw := c.Advance(lbl, tick)
			if xx-lw/2 > lastLabelEnd+u(8) && xx+lw/2 <= x+w {
				c.TextCenter(xx, rowHour, lbl, tick, render.InkBlack)
				lastLabelEnd = xx + lw/2
			}
		}
	}
	for d := startOfDay(t0); d.Before(t1); d = startOfDay(d.Add(26 * time.Hour)) {
		a, b := d, startOfDay(d.Add(26*time.Hour))
		if a.Before(t0) {
			a = t0
		}
		if b.After(t1) {
			b = t1
		}
		lbl := timefmt.Format(d, "dddd", v.Lang)
		xa, xb := xOf(a), xOf(b)
		if c.Advance(lbl, dayLabel)+u(16) > xb-xa {
			lbl = timefmt.Format(d, "ddd", v.Lang)
		}
		if c.Advance(lbl, dayLabel)+u(8) <= xb-xa {
			c.TextCenter((xa+xb)/2, rowDay, lbl, dayLabel, render.InkBlack)
		}
	}
	c.HLine(left, right, bottom, render.InkBlack)

	// Temperature line, then the observation on top.
	r := max(1, u(1.6))
	for i := 1; i < len(samples); i++ {
		a, b := samples[i-1], samples[i]
		c.Line(xOf(a.Time), yOf(a.Temp), xOf(b.Time), yOf(b.Temp), 2*r+1, render.InkBlack)
	}
	nx, ny := xOf(t0), yOf(samples[0].Temp)
	c.FillCircle(nx, ny, u(9), render.InkWhite)
	c.FillCircle(nx, ny, u(6.5), render.InkColour)

	if v.MinMax {
		iMin, iMax := 0, 0
		for i, p := range samples {
			if p.Temp < samples[iMin].Temp {
				iMin = i
			}
			if p.Temp > samples[iMax].Temp {
				iMax = i
			}
		}
		label := func(i int, above bool) {
			p := samples[i]
			lbl := deg(p.Temp)
			lw := c.Advance(lbl, tick)
			cx := clamp(xOf(p.Time), left+lw/2, right-lw/2)
			by := yOf(p.Temp) + u(12) + tickCap
			if above {
				by = yOf(p.Temp) - u(12)
			}
			c.TextCenter(cx, by, lbl, tick, render.InkBlack)
		}
		label(iMax, true)
		if iMin != iMax {
			label(iMin, false)
		}
	}
}

func startOfDay(t time.Time) time.Time {
	y, m, d := t.Date()
	return time.Date(y, m, d, 0, 0, 0, 0, t.Location())
}

func dotted(c *render.Canvas, x0, x1, y, step int) {
	for x := x0; x <= x1; x += step {
		c.Set(x, y, render.InkBlack)
	}
}

func dottedV(c *render.Canvas, x, y0, y1, step int) {
	for y := y0; y <= y1; y += step {
		c.Set(x, y, render.InkBlack)
	}
}

// shade fills a box with a sparse dot pattern (a light grey on e-paper) and a
// solid top edge so the amount stays readable.
func shade(c *render.Canvas, x0, y0, x1, y1 int) {
	if x1 <= x0 {
		return
	}
	for y := y0; y < y1; y++ {
		for x := x0; x < x1; x++ {
			if (x+y)%2 == 0 && y%2 == 0 {
				c.Set(x, y, render.InkBlack)
			}
		}
	}
	c.HLine(x0, x1-1, y0, render.InkBlack)
	c.HLine(x0, x1-1, y0+1, render.InkBlack)
}

// drawDays draws one centred column per day: weekday, date, icon, high and low,
// and the day's precipitation when there is some. Today's label is red.
func drawDays(c *render.Canvas, v view, x, y, w, h int, s float64) {
	n := min(len(v.Days), max(1, w/max(1, px(130, s))))
	if n == 0 {
		return
	}
	colW := w / n
	design, dateH := daysH, dateRowH
	if float64(h) < 0.98*daysH*s {
		design, dateH = daysH-dateRowH, 0
	}
	ds := math.Min(math.Min(float64(h)/design, s*1.3), float64(colW)/150)
	u := func(f float64) int { return px(f, ds) }
	y += max(0, (h-u(design))/2)

	bc := bodyCap(c)
	label := capStyle(c, render.FontDefault, max(u(21), bc))
	date := capStyle(c, render.FontRegular, max(u(15), bc*4/5))
	hiSt := capStyle(c, render.FontBold, u(25))
	loSt := capStyle(c, render.FontRegular, u(21))
	rain := capStyle(c, render.FontRegular, max(u(15), bc*4/5))
	for i, d := range v.Days[:n] {
		cx := x + i*colW + colW/2
		c.TextCenter(cx, y+u(22), c.Fit(d.Label, label, colW-u(8)), label, inkIf(d.Today))
		if dateH > 0 {
			c.TextCenter(cx, y+u(48), c.Fit(d.Date, date, colW-u(8)), date, render.InkBlack)
		}
		y := y - u(dateRowH-dateH)
		c.Icon(cx-u(40), y+u(60), u(80), u(66), forecast.Glyph(d.Icon), render.FontWeather, render.InkBlack, 0.92)
		hw, gap, lw := c.Advance(d.Hi, hiSt), u(12), c.Advance(d.Lo, loSt)
		hx := cx - (hw+gap+lw)/2
		c.Text(hx, y+u(160), d.Hi, hiSt, render.InkBlack)
		c.Text(hx+hw+gap, y+u(160), d.Lo, loSt, render.InkBlack)
		if d.Precip != "" {
			iw := u(22)
			tw := c.Advance(d.Precip, rain)
			rx := cx - (iw+u(4)+tw)/2
			c.Icon(rx, y+u(172), iw, u(22), glyphRaindrop, render.FontWeather, render.InkBlack, 0.9)
			c.Text(rx+iw+u(4), y+u(190), d.Precip, rain, render.InkBlack)
		}
	}
}
