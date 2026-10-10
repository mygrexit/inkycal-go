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
