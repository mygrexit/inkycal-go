package epd

import "inkycal/internal/render"

// mergedBlack is the black plane a black/white panel shows: red pixels count as
// black, like upstream main.py's _merge_bands for models without colour.
func mergedBlack(f render.Frame) []byte {
	out := make([]byte, len(f.Black))
	for i := range out {
		out[i] = f.Black[i] &^ f.Red[i]
	}
	return out
}
