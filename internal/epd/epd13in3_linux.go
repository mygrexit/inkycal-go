//go:build linux

package epd

func init() {
	openers["epd_13_in_3"] = openSingle(NewEPD13in3)
	openers["epd_13_in_3_colour"] = openSingle(NewEPD13in3b)
}
