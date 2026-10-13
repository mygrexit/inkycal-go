// Package sysinfo wraps the few OS facts the app needs (Linux; stubs elsewhere).
package sysinfo

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

var (
	uptimePath   = "/proc/uptime"
	routePath    = "/proc/net/route"
	netClassPath = "/sys/class/net"
	// MaintenanceFiles: if one exists, the device never powers off (so you can SSH in).
	MaintenanceFiles = []string{
		"/boot/firmware/inkycal-maintenance", "/boot/firmware/inkycal-maintenance.txt",
		"/boot/inkycal-maintenance", "/boot/inkycal-maintenance.txt",
	}
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

// HasDefaultRoute reports whether the kernel has an IPv4 default route, which is
// what DHCP leaves behind once the network is usable.
func HasDefaultRoute() bool {
	data, err := os.ReadFile(routePath)
	if err != nil {
		return false
	}
	lines := strings.Split(string(data), "\n")
	for _, l := range lines[min(1, len(lines)):] { // first line is the header
		f := strings.Fields(l)
		// Iface Destination Gateway Flags RefCnt Use Metric Mask …; flag 0x1 = up
		if len(f) >= 8 && f[1] == "00000000" && f[7] == "00000000" {
			if flags, err := strconv.ParseUint(f[3], 16, 32); err == nil && flags&1 != 0 {
				return true
			}
		}
	}
	return false
}

// WaitNetwork polls for a default route every 50 ms until timeout or ctx ends and
// reports whether one appeared. Without a route table (not Linux) it returns at once.
func WaitNetwork(ctx context.Context, timeout time.Duration) bool {
	if _, err := os.Stat(routePath); err != nil {
		return true
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	tick := time.NewTicker(50 * time.Millisecond)
	defer tick.Stop()
	for !HasDefaultRoute() {
		select {
		case <-ctx.Done():
			return false
		case <-tick.C:
		}
	}
	return true
}

// WirelessInterfaces lists the network interfaces that are radios (wlan0 on a Pi).
func WirelessInterfaces() []string {
	dirs, _ := filepath.Glob(filepath.Join(netClassPath, "*", "wireless"))
	names := make([]string, 0, len(dirs))
	for _, d := range dirs {
		names = append(names, filepath.Base(filepath.Dir(d)))
	}
	return names
}
