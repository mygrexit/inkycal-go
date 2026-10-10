// Package sysinfo wraps the few OS facts the app needs (Linux; stubs elsewhere).
package sysinfo

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

var (
	uptimePath = "/proc/uptime"
	// MaintenanceFiles: if one exists, the device never powers off (so you can SSH in).
	MaintenanceFiles = []string{"/boot/firmware/inkycal-maintenance", "/boot/inkycal-maintenance"}
)

// Uptime is the time since the kernel booted (= how long the OS took before the app started).
func Uptime() (time.Duration, error) {
	data, err := os.ReadFile(uptimePath)
	if err != nil {
		return 0, err
	}
	fields := strings.Fields(string(data))
	if len(fields) == 0 {
		return 0, fmt.Errorf("empty %s", uptimePath)
	}
	secs, err := strconv.ParseFloat(fields[0], 64)
	if err != nil {
		return 0, err
	}
	return time.Duration(secs * float64(time.Second)), nil
}

// Maintenance reports whether a maintenance flag file exists.
func Maintenance() bool {
	for _, p := range MaintenanceFiles {
		if _, err := os.Stat(p); err == nil {
			return true
		}
	}
	return false
}
