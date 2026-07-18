package config

import (
	"path/filepath"
	"testing"
)

// TestLoadStarter parses the starter config WriteStarter produces and checks the
// values land where expected.
func TestLoadStarter(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	if err := WriteStarter(path); err != nil {
		t.Fatalf("WriteStarter: %v", err)
	}
	c, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if c.Settings.MonthlyTotal != 400 || c.Settings.DefaultMinMonthly != 25 {
		t.Fatalf("settings = %+v", c.Settings)
	}
	if len(c.Positions) != 3 {
		t.Fatalf("got %d positions, want 3", len(c.Positions))
	}
	// The EM position overrides min_monthly to 50; the others omit it.
	if c.Positions[1].MinMonthly == nil || *c.Positions[1].MinMonthly != 50 {
		t.Fatalf("EM min_monthly = %v, want 50", c.Positions[1].MinMonthly)
	}
	if c.Positions[0].MinMonthly != nil {
		t.Fatalf("World min_monthly = %v, want nil (use default)", c.Positions[0].MinMonthly)
	}
}

// TestToInput checks the config→state merge: value lookup by ISIN, default vs.
// override floors, missing ISIN defaulting to 0, and wind-down forcing floor 0.
func TestToInput(t *testing.T) {
	override := 50
	c := Config{
		Settings: Settings{MonthlyTotal: 400, DefaultMinMonthly: 25, HorizonMonths: 12},
		Positions: []Position{
			{Name: "A", ISIN: "AAA", Target: 60},                          // default floor 25
			{Name: "B", ISIN: "BBB", Target: 40, MinMonthly: &override},   // floor 50
			{Name: "C", ISIN: "CCC", WindDown: true, MinMonthly: &override}, // forced to 0
		},
	}
	in := c.ToInput(map[string]int64{"AAA": 1000, "BBB": 2000}) // CCC absent → 0

	if in.Positions[0].MinMonthly != 25 {
		t.Fatalf("A floor = %d, want 25 (default)", in.Positions[0].MinMonthly)
	}
	if in.Positions[1].MinMonthly != 50 {
		t.Fatalf("B floor = %d, want 50 (override)", in.Positions[1].MinMonthly)
	}
	if in.Positions[2].MinMonthly != 0 {
		t.Fatalf("C floor = %d, want 0 (wind-down)", in.Positions[2].MinMonthly)
	}
	if in.Values[0] != 1000 || in.Values[1] != 2000 || in.Values[2] != 0 {
		t.Fatalf("values = %v, want [1000 2000 0]", in.Values)
	}
	if in.Monthly != 400 || in.Months != 12 {
		t.Fatalf("monthly/months = %d/%d, want 400/12", in.Monthly, in.Months)
	}
}
