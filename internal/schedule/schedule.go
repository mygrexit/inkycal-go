// Package schedule computes wake-up times and calibration slots.
package schedule

import "time"

// NextWake returns the next wall-clock slot aligned to intervalMin minutes since midnight
// (same as Python's countdown), but at least minSleep in the future so the RTC alarm is
// never set to a time that passes while the Pi is still shutting down.
func NextWake(now time.Time, intervalMin int, minSleep time.Duration) time.Time {
	if intervalMin <= 0 {
		intervalMin = 60
	}
	y, m, d := now.Date()
	since := now.Hour()*60 + now.Minute()
	slot := (since/intervalMin + 1) * intervalMin
	for {
		wake := time.Date(y, m, d, 0, slot, 0, 0, now.Location())
		if wake.Sub(now) >= minSleep {
			return wake
		}
		slot += intervalMin
	}
}

// CalibrationDue is true during a configured hour unless we already calibrated in that hour.
func CalibrationDue(now time.Time, hours []int, last time.Time) bool {
	inHour := false
	for _, h := range hours {
		if h == now.Hour() {
			inHour = true
			break
		}
	}
	if !inHour {
		return false
	}
	l := last.In(now.Location())
	ly, lm, ld := l.Date()
	ny, nm, nd := now.Date()
	return !(ly == ny && lm == nm && ld == nd && l.Hour() == now.Hour())
}
