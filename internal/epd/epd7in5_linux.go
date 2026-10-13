//go:build linux

package epd

import "time"

func init() {
	openers["epd_7_in_5"] = openSingle(func(b Bus, s func(time.Duration)) Display { return NewEPD7in5(b, s) })
	openers["epd_7_in_5_colour"] = openSingle(func(b Bus, s func(time.Duration)) Display { return NewEPD7in5Colour(b, s) })
	openers["epd_7_in_5_v2"] = openSingle(func(b Bus, s func(time.Duration)) Display { return NewEPD7in5V2(b, s) })
	openers["epd_7_in_5_v3"] = openSingle(func(b Bus, s func(time.Duration)) Display { return NewEPD7in5V3(b, s) })
	openers["epd_7_in_5_v3_colour"] = openSingle(func(b Bus, s func(time.Duration)) Display { return NewEPD7in5V3Colour(b, s) })
}
