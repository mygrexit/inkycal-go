package epd

import "time"

func init() {
	Models["epd_5_in_83"] = [2]int{600, 448}
	Models["epd_5_in_83_colour"] = [2]int{600, 448}
}

// NewEPD5in83 drives the Waveshare 5.83" (V1) black/white panel (Python: epd_5_in_83).
// Same UC8159 sequence as the 7.5" V1, see newUC8159.
func NewEPD5in83(bus Bus, sleep func(time.Duration)) Display {
	return newUC8159(bus, sleep, 600, 448, false, nil)
}

// NewEPD5in83Colour drives the Waveshare 5.83" (V1) black/white/red panel (Python: epd_5_in_83_colour).
func NewEPD5in83Colour(bus Bus, sleep func(time.Duration)) Display {
	return newUC8159(bus, sleep, 600, 448, true, []byte{0x03, 0x03})
}
