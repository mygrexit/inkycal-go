//go:build !linux

package sysinfo

import "errors"

func NTPSynced() bool { return false }

func PowerOff() error { return errors.New("power off is only supported on Linux") }
