//go:build linux

package epd

func init() {
	openers["epd_5_in_83_V2"] = openSingle(NewEPD5in83V2)
	openers["epd5in83b_V2"] = openSingle(NewEPD5in83bV2)
}
