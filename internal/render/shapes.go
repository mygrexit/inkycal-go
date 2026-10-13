package render

import "math"

// HLine draws a 1px horizontal line from x0 to x1 (inclusive).
func (c *Canvas) HLine(x0, x1, y int, ink Ink) {
	if x0 > x1 {
		x0, x1 = x1, x0
	}
	for x := x0; x <= x1; x++ {
		c.set(x, y, ink)
	}
}

// VLine draws a 1px vertical line from y0 to y1 (inclusive).
func (c *Canvas) VLine(x, y0, y1 int, ink Ink) {
	if y0 > y1 {
		y0, y1 = y1, y0
	}
	for y := y0; y <= y1; y++ {
		c.set(x, y, ink)
	}
}

// RoundedRect draws a 1px outline covering pixels x..x+w-1, y..y+h-1 with corner radius r.
func (c *Canvas) RoundedRect(x, y, w, h, r int, ink Ink) {
	if w <= 0 || h <= 0 {
		return
	}
	if max := min(w, h) / 2; r > max {
		r = max
	}
	x1, y1 := x+w-1, y+h-1
	c.HLine(x+r, x1-r, y, ink)
	c.HLine(x+r, x1-r, y1, ink)
	c.VLine(x, y+r, y1-r, ink)
	c.VLine(x1, y+r, y1-r, ink)
	if r == 0 {
		return
	}
	corners := [4][2]int{{x + r, y + r}, {x1 - r, y + r}, {x + r, y1 - r}, {x1 - r, y1 - r}}
	for i, cc := range corners {
		for dy := 0; dy <= r; dy++ {
			for dx := 0; dx <= r; dx++ {
				if math.Abs(math.Hypot(float64(dx), float64(dy))-float64(r)) > 0.5 {
					continue
				}
				px, py := cc[0]-dx, cc[1]-dy
				if i == 1 || i == 3 {
					px = cc[0] + dx
				}
				if i >= 2 {
					py = cc[1] + dy
				}
				c.set(px, py, ink)
			}
		}
	}
}

// FillCircle fills a disc of radius r around (cx, cy).
func (c *Canvas) FillCircle(cx, cy, r int, ink Ink) {
	for dy := -r; dy <= r; dy++ {
		for dx := -r; dx <= r; dx++ {
			if dx*dx+dy*dy <= r*r {
				c.set(cx+dx, cy+dy, ink)
			}
		}
	}
}

// Border is Python's draw_border: the box (w, h) shrunk by shrinkX/shrinkY, centred, with rounded corners.
func Border(c *Canvas, x, y, w, h, r int, shrinkX, shrinkY float64, ink Ink) {
	bw := int(float64(w) * (1 - shrinkX))
	bh := int(float64(h) * (1 - shrinkY))
	c.RoundedRect(x+(w-bw)/2, y+(h-bh)/2, bw+1, bh+1, r, ink)
}

// Line draws a straight segment with a pen w pixels wide: a square pen up to 2 px
// (crisp sparklines), a round one above (smooth curves without notches at joints).
func (c *Canvas) Line(x0, y0, x1, y1, w int, ink Ink) {
	dx, dy := x1-x0, y1-y0
	if dx < 0 {
		dx = -dx
	}
	if dy > 0 {
		dy = -dy
	}
	sx, sy := 1, 1
	if x0 > x1 {
		sx = -1
	}
	if y0 > y1 {
		sy = -1
	}
	e := dx + dy
	for {
		if w <= 2 {
			for py := 0; py < max(w, 1); py++ {
				for px := 0; px < max(w, 1); px++ {
					c.set(x0+px, y0+py, ink)
				}
			}
		} else {
			c.FillCircle(x0, y0, w/2, ink)
		}
		if x0 == x1 && y0 == y1 {
			return
		}
		e2 := 2 * e
		if e2 >= dy {
			e += dy
			x0 += sx
		}
		if e2 <= dx {
			e += dx
			y0 += sy
		}
	}
}
