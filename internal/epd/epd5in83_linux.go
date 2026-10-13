//go:build linux

package epd

func init() {
	openers["epd_5_in_83"] = openSingle(NewEPD5in83)
	openers["epd_5_in_83_colour"] = openSingle(NewEPD5in83Colour)
}
