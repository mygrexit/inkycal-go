package state

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestLoadMissingIsZero(t *testing.T) {
	s, err := Load(t.TempDir())
	if err != nil || s.LastHash != "" || s.Counter != 0 {
		t.Fatalf("%+v %v", s, err)
	}
}

func TestSaveLoadRoundTrip(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "sub")
	want := State{LastHash: "abc", Counter: 4, LastCalibration: time.Date(2026, 10, 10, 12, 0, 0, 0, time.UTC)}
	if err := Save(dir, want); err != nil {
		t.Fatal(err)
	}
	got, err := Load(dir)
	if err != nil || got.LastHash != want.LastHash || got.Counter != 4 || !got.LastCalibration.Equal(want.LastCalibration) {
		t.Fatalf("%+v %v", got, err)
	}
	entries, _ := os.ReadDir(dir)
	if len(entries) != 1 {
		t.Errorf("temp files left behind: %v", entries)
	}
}

func TestLoadCorruptFileErrors(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "state.json"), []byte("{"), 0o644)
	if _, err := Load(dir); err == nil {
		t.Fatal("expected error")
	}
}

func TestAppendTiming(t *testing.T) {
	dir := t.TempDir()
	AppendTiming(dir, "a")
	AppendTiming(dir, "b")
	data, _ := os.ReadFile(filepath.Join(dir, "timings.csv"))
	if string(data) != "a\nb\n" {
		t.Errorf("%q", data)
	}
}
