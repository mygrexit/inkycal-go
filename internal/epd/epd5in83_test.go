package epd

import "testing"

func init() {
	traced["epd_5_in_83"] = blackOnly(NewEPD5in83)
	traced["epd_5_in_83_colour"] = NewEPD5in83Colour
}

func TestPack5in83(t *testing.T) {
	// pixels: white, black, red, red-over-black, then 4 white
	got := pack5in83([]byte{0b10101111}, []byte{0b00110000})
	want := []byte{0x30, 0x44, 0x33, 0x33}
	if string(got) != string(want) {
		t.Errorf("got % x, want % x", got, want)
	}
}
