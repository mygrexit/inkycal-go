// Package schedule computes wake-up times and calibration slots.
package schedule

import (
	"fmt"
	"sort"
	"strings"
	"time"
)

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

// ParseTimes reads "HH:MM" wall-clock times into sorted, distinct minutes since midnight.
func ParseTimes(list []string) ([]int, error) {
	seen := map[int]bool{}
	var out []int
	for _, s := range list {
		t, err := time.Parse("15:04", strings.TrimSpace(s))
		if err != nil {
			return nil, fmt.Errorf("update time %q: want HH:MM", s)
		}
		if m := t.Hour()*60 + t.Minute(); !seen[m] {
			seen[m] = true
			out = append(out, m)
		}
	}
	sort.Ints(out)
	return out, nil
}

// NextWakeAt returns the next of the given times of day (minutes since midnight, sorted)
// that is at least minSleep in the future, looking ahead into the following days.
func NextWakeAt(now time.Time, times []int, minSleep time.Duration) time.Time {
	if len(times) == 0 {
		return NextWake(now, 0, minSleep)
	}
	y, m, d := now.Date()
	for day := range 3 {
		for _, t := range times {
			wake := time.Date(y, m, d+day, t/60, t%60, 0, 0, now.Location())
			if wake.Sub(now) >= minSleep {
				return wake
			}
		}
	}
	return NextWake(now, 0, minSleep)
}
