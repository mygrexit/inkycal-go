//go:build linux

package epd

func init() {
	openers["epd_4_in_2"] = openSingle(NewEPD4in2)
	openers["epd_4_in_2_colour"] = openSingle(NewEPD4in2Colour)
}
