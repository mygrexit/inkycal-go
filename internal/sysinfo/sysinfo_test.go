package sysinfo

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestUptime(t *testing.T) {
	p := filepath.Join(t.TempDir(), "uptime")
	os.WriteFile(p, []byte("12.34 40.00\n"), 0o644)
	old := uptimePath
	uptimePath = p
	defer func() { uptimePath = old }()
	d, err := Uptime()
	if err != nil || d != 12340*time.Millisecond {
		t.Fatalf("%v %v", d, err)
	}
}

func TestMaintenance(t *testing.T) {
	dir := t.TempDir()
	old := MaintenanceFiles
	defer func() { MaintenanceFiles = old }()
	MaintenanceFiles = []string{filepath.Join(dir, "a"), filepath.Join(dir, "b")}
	if Maintenance() {
		t.Fatal("no file yet")
	}
	os.WriteFile(MaintenanceFiles[1], nil, 0o644)
	if !Maintenance() {
		t.Fatal("flag file not detected")
	}
}
