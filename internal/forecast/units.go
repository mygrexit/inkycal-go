package forecast

// Fahrenheit converts °C to °F.
func Fahrenheit(c float64) float64 { return c*9.0/5.0 + 32.0 }

// Beaufort converts m/s to the Beaufort scale (thresholds from the Python wrapper).
func Beaufort(mps float64) int {
	thresholds := []float64{0.3, 1.6, 3.4, 5.5, 8.0, 10.8, 13.9, 17.2, 20.8, 24.5, 28.5, 32.7}
	for i, t := range thresholds {
		if mps < t {
			return i
		}
	}
	return 12
}

// WindLevel buckets a mean wind speed: Beaufort ≤3 calm (1), 4–5 breezy (2), ≥6 strong (3).
func WindLevel(mps float64) int {
	switch b := Beaufort(mps); {
	case b >= 6:
		return 3
	case b >= 4:
		return 2
	default:
		return 1
	}
}
