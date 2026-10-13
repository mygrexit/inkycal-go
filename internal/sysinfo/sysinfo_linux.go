//go:build linux

package sysinfo

import (
	"fmt"
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

// SetLink brings an interface up or down. Unlike rfkill this is not restored at
// the next boot, so a radio switched off before power-off comes back by itself.
func SetLink(name string, up bool) error {
	state := "down"
	if up {
		state = "up"
	}
	if out, err := exec.Command("ip", "link", "set", "dev", name, state).CombinedOutput(); err != nil {
		return fmt.Errorf("ip link set %s %s: %w: %s", name, state, err, out)
	}
	return nil
}
