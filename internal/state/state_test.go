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
		Monthly: 400,
		Months:  12,
	}
	if err := Save(path, want); err != nil {
		t.Fatalf("Save: %v", err)
	}
	got, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got.Values["IE00B4L5Y983"] != 1245000 || got.Monthly != 400 || got.Months != 12 {
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

// TestSetValueSameDayReplaces: two writes on the same day fold into one snapshot.
func TestSetValueSameDayReplaces(t *testing.T) {
	var s State
	day := time.Date(2026, 7, 18, 9, 0, 0, 0, time.UTC)

	s.SetValue("AAA", 100, day)
	s.SetValue("AAA", 200, day.Add(2*time.Hour)) // same calendar day
	s.SetValue("BBB", 300, day.Add(3*time.Hour))

	if s.Values["AAA"] != 200 {
		t.Fatalf("AAA = %d, want 200 (latest wins)", s.Values["AAA"])
	}
	if len(s.History) != 1 {
		t.Fatalf("history has %d snapshots, want 1 (same day)", len(s.History))
	}
	if s.History[0].Values["AAA"] != 200 || s.History[0].Values["BBB"] != 300 {
		t.Fatalf("snapshot = %v", s.History[0].Values)
	}

	// A later day starts a new snapshot.
	s.SetValue("AAA", 400, day.AddDate(0, 0, 1))
	if len(s.History) != 2 {
		t.Fatalf("history has %d snapshots, want 2 after new day", len(s.History))
	}
}
