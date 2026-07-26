package state

import (
	"path/filepath"
	"testing"
	"time"
)

// TestSaveLoadRoundTrip writes state and reads it back unchanged.
func TestSaveLoadRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.json")
	want := State{
		Updated: "2026-07-18T10:00:00Z",
		Values:  map[string]int64{"IE00B4L5Y983": 1245000, "IE00BKM4GZ66": 731050},
	}
	if err := Save(path, want); err != nil {
		t.Fatalf("Save: %v", err)
	}
	got, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got.Values["IE00B4L5Y983"] != 1245000 || got.Values["IE00BKM4GZ66"] != 731050 {
		t.Fatalf("round-trip mismatch: %+v", got)
	}
}

// TestLoadMissingIsEmpty: a missing file is not an error; values default to 0.
func TestLoadMissingIsEmpty(t *testing.T) {
	got, err := Load(filepath.Join(t.TempDir(), "does-not-exist.json"))
	if err != nil {
		t.Fatalf("Load of missing file returned error: %v", err)
	}
	if got.Values == nil || len(got.Values) != 0 {
		t.Fatalf("expected empty values map, got %v", got.Values)
	}
}

// TestSetValue: the latest write for an ISIN wins and Updated is stamped.
func TestSetValue(t *testing.T) {
	var s State
	now := time.Date(2026, 7, 18, 9, 0, 0, 0, time.UTC)

	s.SetValue("AAA", 100, now)
	s.SetValue("AAA", 200, now.Add(2*time.Hour)) // overwrites
	s.SetValue("BBB", 300, now.Add(3*time.Hour))

	if s.Values["AAA"] != 200 || s.Values["BBB"] != 300 {
		t.Fatalf("values = %v", s.Values)
	}
	if s.Updated == "" {
		t.Fatal("Updated was not stamped")
	}
}
