package weather

import "inkycal/internal/render"

// grid is the 7-column layout of inkycal_weather.py:
//
//	| current icon | temp/hum/wind | moon/sunrise/sunset | fc1 | fc2 | fc3 | fc4 |
type grid struct {
	ColW, RowH, IconSmall, LineGap int
	Cols                           [7]int
	Rows                           [3]int
	BoxY, BoxH                     int // the borders; the rows sit centred inside
}

func computeGrid(w, h int) grid {
	g := grid{ColW: w / 7}
	if float64(w)/float64(h) >= 4 {
		g.RowH = h / 3
	} else {
		g.RowH = int(float64(h) * (1 - float64(h)/float64(w)) / 3)
	}
	spacingTop := (w % g.ColW) / 2
	g.IconSmall = g.ColW / 3
	for i := range g.Cols {
		g.Cols[i] = spacingTop + i*g.ColW
	}
	g.LineGap = (h - spacingTop - 3*g.RowH) / 4
	// Unlike Python (content flush with the border's top edge), centre the rows vertically.
	contentH := 3*g.RowH + 2*g.LineGap
	g.Rows[0] = (h - contentH) / 2
	g.BoxY = min(spacingTop, g.Rows[0])
	g.BoxH = h - 2*g.BoxY
	g.Rows[1] = g.Rows[0] + g.LineGap + g.RowH
	g.Rows[2] = g.Rows[1] + g.LineGap + g.RowH
	return g
}

func draw(c *render.Canvas, v view, g grid) {
	const fill = 0.9
	text := render.TextOpts{}
	tempText := render.TextOpts{}
	if v.TempNegative {
		tempText.Ink = render.InkColour
	}
	small := g.ColW - g.IconSmall
	iconY := func(row int) int { return g.Rows[row] + (g.RowH-g.IconSmall)/2 } // square icons, centred on their text

	// Column 1: current weather icon over the full height.
	c.Icon(g.Cols[0], 0, g.ColW, c.H, v.CurrentIcon, render.FontWeather, render.InkColour, fill)

	// Column 2: temperature, humidity, wind.
	c.Icon(g.Cols[1], g.Rows[0], g.IconSmall, g.RowH, glyphThermometer, render.FontWeather, render.InkColour, fill)
	c.Write(g.Cols[1]+g.IconSmall, g.Rows[0], small, g.RowH, v.Temperature, tempText)
	c.Icon(g.Cols[1], g.Rows[1], g.IconSmall, g.RowH, glyphHumidity, render.FontWeather, render.InkColour, fill)
	c.Write(g.Cols[1]+g.IconSmall, g.Rows[1], small, g.RowH, v.Humidity, text)
	c.Icon(g.Cols[1], iconY(2), g.IconSmall, g.IconSmall, glyphWind, render.FontWeather, render.InkColour, fill)
	c.Write(g.Cols[1]+g.IconSmall, g.Rows[2], small, g.RowH, v.Wind, text)

	// Column 3: moon phase, sunrise, sunset.
	c.Icon(g.Cols[2], g.Rows[0], g.ColW, g.RowH, v.Moon, render.FontWeather, render.InkColour, fill)
	c.Icon(g.Cols[2], iconY(1), g.IconSmall, g.IconSmall, glyphSunrise, render.FontWeather, render.InkColour, fill)
	c.Write(g.Cols[2]+g.IconSmall, g.Rows[1], small, g.RowH, v.Sunrise, text)
	c.Icon(g.Cols[2], iconY(2), g.IconSmall, g.IconSmall, glyphSunset, render.FontWeather, render.InkColour, fill)
	c.Write(g.Cols[2]+g.IconSmall, g.Rows[2], small, g.RowH, v.Sunset, text)

	// Columns 4-7: forecasts.
	for i, fc := range v.Forecasts {
		x := g.Cols[3+i]
		c.Write(x, g.Rows[0], g.ColW, g.RowH, fc.Stamp, text)
		c.Icon(x, g.Rows[0]+g.RowH, g.ColW, g.RowH+2*g.LineGap, fc.Icon, render.FontWeather, render.InkColour, fill)
		c.Write(x, g.Rows[2], g.ColW, g.RowH, fc.Temp, text)
	}

	render.Border(c, g.Cols[0], g.BoxY, g.ColW*3-3, g.BoxH-1, 5, 0, 0, render.InkBlack)
	for i := 3; i < 7; i++ {
		render.Border(c, g.Cols[i], g.BoxY, g.ColW-3, g.BoxH-1, 5, 0, 0, render.InkBlack)
	}
}
