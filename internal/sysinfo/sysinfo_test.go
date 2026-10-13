package sysinfo

import (
	"context"
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

// Windows hides extensions: a "new text file" named inkycal-maintenance is really .txt.
func TestMaintenanceAcceptsTxtFromWindows(t *testing.T) {
	for _, want := range []string{"/boot/firmware/inkycal-maintenance.txt", "/boot/inkycal-maintenance.txt"} {
		found := false
		for _, f := range MaintenanceFiles {
			found = found || f == want
		}
		if !found {
			t.Errorf("%s is not checked", want)
		}
	}
}

func TestHasDefaultRoute(t *testing.T) {
	old := routePath
	defer func() { routePath = old }()
	routePath = filepath.Join(t.TempDir(), "route")
	const header = "Iface\tDestination\tGateway\tFlags\tRefCnt\tUse\tMetric\tMask\tMTU\tWindow\tIRTT\n"
	cases := []struct {
		name, table string
		want        bool
	}{
		{"missing file", "", false},
		{"only the LAN", header + "wlan0\t0014A8C0\t00000000\t0001\t0\t0\t600\t00FFFFFF\t0\t0\t0\n", false},
		{"default route", header + "wlan0\t00000000\t0114A8C0\t0003\t0\t0\t600\t00000000\t0\t0\t0\n", true},
		{"default route down", header + "wlan0\t00000000\t0114A8C0\t0002\t0\t0\t600\t00000000\t0\t0\t0\n", false},
	}
	for _, c := range cases {
		os.Remove(routePath)
		if c.table != "" {
			os.WriteFile(routePath, []byte(c.table), 0o644)
		}
		if got := HasDefaultRoute(); got != c.want {
			t.Errorf("%s: got %v", c.name, got)
		}
	}
	// WaitNetwork gives up after its timeout without a route …
	os.WriteFile(routePath, []byte(cases[1].table), 0o644)
	start := time.Now()
	if WaitNetwork(context.Background(), 120*time.Millisecond) || time.Since(start) > time.Second {
		t.Error("WaitNetwork without a route")
	}
	// … returns as soon as one appears …
	go func() {
		time.Sleep(100 * time.Millisecond)
		os.WriteFile(routePath, []byte(cases[2].table), 0o644)
	}()
	if !WaitNetwork(context.Background(), 5*time.Second) {
		t.Error("route appeared but WaitNetwork timed out")
	}
	// … and does not block where there is no route table at all.
	os.Remove(routePath)
	if !WaitNetwork(context.Background(), time.Hour) {
		t.Error("no route table must not block")
	}
}

func TestWirelessInterfaces(t *testing.T) {
	old := netClassPath
	defer func() { netClassPath = old }()
	netClassPath = t.TempDir()
	os.MkdirAll(filepath.Join(netClassPath, "wlan0", "wireless"), 0o755)
	os.MkdirAll(filepath.Join(netClassPath, "eth0"), 0o755)
	os.MkdirAll(filepath.Join(netClassPath, "lo"), 0o755)
	if got := WirelessInterfaces(); len(got) != 1 || got[0] != "wlan0" {
		t.Errorf("got %v", got)
	}
}
