//go:build linux

package sysinfo

import (
	"os/exec"

	"golang.org/x/sys/unix"
)

// NTPSynced reports the kernel's clock-synchronised flag (what timedatectl shows as "System clock synchronized").
func NTPSynced() bool {
	var tx unix.Timex
	if _, err := unix.Adjtimex(&tx); err != nil {
		return false
	}
	return tx.Status&unix.STA_UNSYNC == 0
}

// PowerOff asks systemd to power off (the PiSugar alarm must be set before).
func PowerOff() error { return exec.Command("systemctl", "poweroff").Run() }
