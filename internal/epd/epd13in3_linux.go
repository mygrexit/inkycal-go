//go:build linux

package epd

import "time"

func init() {
	openers["epd_13_in_3"] = openSingle(func(b Bus, s func(time.Duration)) Display { return NewEPD13in3(b, s) })
	openers["epd_13_in_3_colour"] = openSingle(func(b Bus, s func(time.Duration)) Display { return NewEPD13in3b(b, s) })
}
