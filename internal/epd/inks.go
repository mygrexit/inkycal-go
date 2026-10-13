package epd

import "strings"

// Inks tells which inks a model shows: "bw", "bwr" (the colour layer is red or, on some
// "(C)" panels, yellow) or "gray16" (IT8951, where the colour layer turns dark grey).
func Inks(model string) string {
	switch {
	case model == "9_in_7" || model == "10_in_3" || model == "7_in_8":
		return "gray16"
	case strings.Contains(model, "colour") || model == "epd5in83b_V2" || strings.HasPrefix(model, "image_file"):
		return "bwr"
	}
	return "bw"
}

var diagonals = map[string]float64{
	"epd_12_in_48": 12.48, "epd_13_in_3": 13.3, "epd_4_in_2": 4.2, "epd_5_in_83": 5.83,
	"epd5in83b": 5.83, "epd_7_in_5": 7.5, "9_in_7": 9.7, "10_in_3": 10.3, "7_in_8": 7.8,
	"image_file_12_in_48": 12.48, "image_file": 7.5,
}

// Diagonal is the panel's diagonal in inches (0 if unknown), for true-size previews.
func Diagonal(model string) float64 {
	best := ""
	for prefix := range diagonals {
		if strings.HasPrefix(model, prefix) && len(prefix) > len(best) {
			best = prefix
		}
	}
	return diagonals[best]
}
