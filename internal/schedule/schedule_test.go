package schedule

import (
	"testing"
	"time"
)

func at(h, m, s int) time.Time { return time.Date(2026, 10, 10, h, m, s, 0, time.UTC) }

func TestNextWake(t *testing.T) {
	cases := []struct {
		now      time.Time
		interval int
		want     time.Time
	}{
		{at(10, 7, 30), 15, at(10, 15, 0)},
		{at(10, 15, 0), 15, at(10, 30, 0)},
		{at(23, 50, 0), 60, time.Date(2026, 10, 11, 0, 0, 0, 0, time.UTC)},
		{at(10, 0, 0), 240, at(12, 0, 0)},   // 4h slots since midnight: 0,4,8,12
		{at(10, 14, 30), 15, at(10, 30, 0)}, // 10:15 is only 30 s away → skip
	}
	for _, c := range cases {
		if got := NextWake(c.now, c.interval, 90*time.Second); !got.Equal(c.want) {
			t.Errorf("NextWake(%s, %d) = %s, want %s", c.now.Format("15:04:05"), c.interval, got, c.want)
		}
	}
}

func TestNextWakeAcrossDST(t *testing.T) {
	berlin, err := time.LoadLocation("Europe/Berlin")
	if err != nil {
		t.Skip("no tzdata")
	}
	now := time.Date(2026, 3, 29, 1, 30, 0, 0, berlin) // clocks jump 02:00 → 03:00
	got := NextWake(now, 60, time.Minute)
	if got.Hour() != 3 || got.Sub(now) != 30*time.Minute {
		t.Errorf("got %s (in %s)", got, got.Sub(now))
	}
}

func TestCalibrationDue(t *testing.T) {
	hours := []int{0, 12}
	if !CalibrationDue(at(12, 5, 0), hours, time.Time{}) {
		t.Error("should calibrate at 12:05 when never calibrated")
	}
	if CalibrationDue(at(12, 35, 0), hours, at(12, 5, 0)) {
		t.Error("already calibrated this hour")
	}
	if !CalibrationDue(at(12, 5, 0), hours, at(12, 5, 0).AddDate(0, 0, -1)) {
		t.Error("yesterday's calibration does not count")
	}
	if CalibrationDue(at(13, 0, 0), hours, time.Time{}) {
		t.Error("13 is not a calibration hour")
	}
}

func TestParseTimes(t *testing.T) {
	got, err := ParseTimes([]string{"12:00", "6:00", " 06:00 "})
	if err != nil || len(got) != 2 || got[0] != 6*60 || got[1] != 12*60 {
		t.Errorf("ParseTimes = %v, %v (want sorted, deduplicated [360 720])", got, err)
	}
	for _, bad := range []string{"24:00", "6", "06:60", "x"} {
		if _, err := ParseTimes([]string{bad}); err == nil {
			t.Errorf("%q accepted", bad)
		}
	}
}

func TestNextWakeAt(t *testing.T) {
	times := []int{6 * 60, 12 * 60}
	cases := []struct{ now, want time.Time }{
		{at(5, 0, 0), at(6, 0, 0)},
		{at(6, 0, 30), at(12, 0, 0)}, // woke at 06:00, now plans noon
		{at(5, 59, 0), at(12, 0, 0)}, // 06:00 is closer than minSleep
		{at(12, 1, 0), time.Date(2026, 10, 11, 6, 0, 0, 0, time.UTC)},
		{at(23, 59, 0), time.Date(2026, 10, 11, 6, 0, 0, 0, time.UTC)},
	}
	for _, c := range cases {
		if got := NextWakeAt(c.now, times, 90*time.Second); !got.Equal(c.want) {
			t.Errorf("NextWakeAt(%s) = %s, want %s", c.now.Format("15:04:05"), got, c.want)
		}
	}
}

func TestNextWakeAtAcrossDST(t *testing.T) {
	berlin, err := time.LoadLocation("Europe/Berlin")
	if err != nil {
		t.Skip("no tzdata")
	}
	now := time.Date(2026, 10, 24, 12, 5, 0, 0, berlin) // night of 25 Oct: 03:00 → 02:00
	got := NextWakeAt(now, []int{6 * 60}, time.Minute)
	if got.Day() != 25 || got.Hour() != 6 || got.Minute() != 0 {
		t.Errorf("got %s", got)
	}
}
