//go:build linux

package epd

func init() {
	openers["epd_7_in_5"] = openSingle(NewEPD7in5)
	openers["epd_7_in_5_colour"] = openSingle(NewEPD7in5Colour)
	openers["epd_7_in_5_v2"] = openSingle(NewEPD7in5V2)
	openers["epd_7_in_5_v3"] = openSingle(NewEPD7in5V3)
	openers["epd_7_in_5_v3_colour"] = openSingle(NewEPD7in5V3Colour)
}
