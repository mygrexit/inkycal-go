//go:build !linux

package sysinfo

import "errors"

func NTPSynced() bool { return false }

func PowerOff() error { return errors.New("power off is only supported on Linux") }

func SetLink(string, bool) error { return errors.New("links can only be switched on Linux") }
