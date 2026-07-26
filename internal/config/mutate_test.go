package config

import (
	"math"
	"path/filepath"
	"testing"
)

// base is a 50/30/20 three-position config used across the mutation tests.
func base() Config {
	return Config{
		Settings: Settings{MonthlyTotal: 400, DefaultMinMonthly: 25, HorizonMonths: 12, TolerancePP: 0.5, MaxHorizon: 36},
		Positions: []Position{
			{Name: "World", ISIN: "AAA", Target: 50},
			{Name: "EM", ISIN: "BBB", Target: 30},
			{Name: "Europe", ISIN: "CCC", Target: 20},
		},
	}
}

// targetsSumTo100 asserts the active targets sum to exactly 100.0.
func targetsSumTo100(t *testing.T, c Config) {
	t.Helper()
	var sum float64
	for _, p := range c.Positions {
		if !p.WindDown {
			sum += p.Target
		}
	}
	if math.Abs(sum-100) > 1e-9 {
		t.Fatalf("active targets sum to %v, want 100", sum)
	}
}

func targetOf(c Config, isin string) float64 {
	for _, p := range c.Positions {
		if p.ISIN == isin {
			return p.Target
		}
	}
	return math.NaN()
}

func TestAddPositionScale(t *testing.T) {
	got, err := AddPosition(base(), "Small", "DDD", 10, nil, Redistribution{Scale: true})
	if err != nil {
		t.Fatalf("AddPosition: %v", err)
	}
	if len(got.Positions) != 4 {
		t.Fatalf("got %d positions, want 4", len(got.Positions))
	}
	targetsSumTo100(t, got)
	// 50/30/20 scaled into 90 → 45/27/18, plus the new 10.
	if targetOf(got, "AAA") != 45 || targetOf(got, "BBB") != 27 || targetOf(got, "CCC") != 18 || targetOf(got, "DDD") != 10 {
		t.Fatalf("targets = %+v", got.Positions)
	}
	// Input config is not mutated.
	if targetOf(base(), "AAA") != 50 {
		t.Fatal("AddPosition mutated its input")
	}
}

func TestAddPositionExplicitSources(t *testing.T) {
	r := Redistribution{Sources: []Source{{ISIN: "AAA", PP: 6}, {ISIN: "BBB", PP: 4}}}
	got, err := AddPosition(base(), "Small", "DDD", 10, nil, r)
	if err != nil {
		t.Fatalf("AddPosition: %v", err)
	}
	targetsSumTo100(t, got)
	if targetOf(got, "AAA") != 44 || targetOf(got, "BBB") != 26 || targetOf(got, "CCC") != 20 || targetOf(got, "DDD") != 10 {
		t.Fatalf("targets = %+v", got.Positions)
	}
}

func TestAddPositionErrors(t *testing.T) {
	cases := []struct {
		name string
		r    Redistribution
		isin string
		tgt  float64
	}{
		{"sources don't sum to target", Redistribution{Sources: []Source{{"AAA", 3}, {"BBB", 3}}}, "DDD", 10},
		{"source driven below zero", Redistribution{Sources: []Source{{"CCC", 30}}}, "DDD", 30},
		{"unknown source ISIN", Redistribution{Sources: []Source{{"ZZZ", 10}}}, "DDD", 10},
		{"duplicate ISIN", Redistribution{Scale: true}, "AAA", 10},
		{"no redistribution given", Redistribution{}, "DDD", 10},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if _, err := AddPosition(base(), "X", c.isin, c.tgt, nil, c.r); err == nil {
				t.Fatal("expected error, got nil")
			}
		})
	}
}

func TestAddFirstPosition(t *testing.T) {
	empty := Config{Settings: base().Settings}
	got, err := AddPosition(empty, "Only", "AAA", 100, nil, Redistribution{})
	if err != nil {
		t.Fatalf("AddPosition: %v", err)
	}
	if len(got.Positions) != 1 || targetOf(got, "AAA") != 100 {
		t.Fatalf("targets = %+v", got.Positions)
	}
}

func TestRemovePositionScale(t *testing.T) {
	got, err := RemovePosition(base(), "CCC", Redistribution{Scale: true})
	if err != nil {
		t.Fatalf("RemovePosition: %v", err)
	}
	if len(got.Positions) != 2 {
		t.Fatalf("got %d positions, want 2", len(got.Positions))
	}
	targetsSumTo100(t, got)
	// 50/30 scaled to 100 → 62.5/37.5.
	if targetOf(got, "AAA") != 62.5 || targetOf(got, "BBB") != 37.5 {
		t.Fatalf("targets = %+v", got.Positions)
	}
}

func TestRemovePositionExplicit(t *testing.T) {
	r := Redistribution{Sources: []Source{{ISIN: "AAA", PP: 8}, {ISIN: "BBB", PP: 12}}}
	got, err := RemovePosition(base(), "CCC", r)
	if err != nil {
		t.Fatalf("RemovePosition: %v", err)
	}
	targetsSumTo100(t, got)
	if targetOf(got, "AAA") != 58 || targetOf(got, "BBB") != 42 {
		t.Fatalf("targets = %+v", got.Positions)
	}
}

func TestRemoveLastActiveFails(t *testing.T) {
	c := Config{Settings: base().Settings, Positions: []Position{{Name: "Only", ISIN: "AAA", Target: 100}}}
	if _, err := RemovePosition(c, "AAA", Redistribution{Scale: true}); err == nil {
		t.Fatal("expected error removing last active position")
	}
}

func TestRetargetUpAndDown(t *testing.T) {
	// Raise AAA 50→60, scale the others down; they must fill 40.
	up, err := Retarget(base(), "AAA", 60, Redistribution{Scale: true})
	if err != nil {
		t.Fatalf("Retarget up: %v", err)
	}
	targetsSumTo100(t, up)
	if targetOf(up, "AAA") != 60 {
		t.Fatalf("AAA = %v, want 60", targetOf(up, "AAA"))
	}

	// Lower AAA 50→40 taking the 10 explicitly onto BBB.
	down, err := Retarget(base(), "AAA", 40, Redistribution{Sources: []Source{{ISIN: "BBB", PP: 10}}})
	if err != nil {
		t.Fatalf("Retarget down: %v", err)
	}
	targetsSumTo100(t, down)
	if targetOf(down, "AAA") != 40 || targetOf(down, "BBB") != 40 || targetOf(down, "CCC") != 20 {
		t.Fatalf("targets = %+v", down.Positions)
	}
}

func TestFreezeAndUnfreeze(t *testing.T) {
	// Freeze Europe (20), scaling the others to fill 100.
	frozen, err := Freeze(base(), "CCC", Redistribution{Scale: true})
	if err != nil {
		t.Fatalf("Freeze: %v", err)
	}
	var cc Position
	for _, p := range frozen.Positions {
		if p.ISIN == "CCC" {
			cc = p
		}
	}
	if !cc.WindDown || cc.Target != 0 || cc.MinMonthly != nil {
		t.Fatalf("frozen CCC = %+v, want wind-down with 0 target and no floor", cc)
	}
	targetsSumTo100(t, frozen) // active set (AAA/BBB) sums to 100
	if targetOf(frozen, "AAA") != 62.5 || targetOf(frozen, "BBB") != 37.5 {
		t.Fatalf("active targets = %+v", frozen.Positions)
	}

	// Unfreeze it back to 20, taking room explicitly from AAA.
	back, err := Unfreeze(frozen, "CCC", 20, Redistribution{Sources: []Source{{ISIN: "AAA", PP: 20}}})
	if err != nil {
		t.Fatalf("Unfreeze: %v", err)
	}
	for _, p := range back.Positions {
		if p.ISIN == "CCC" && (p.WindDown || p.Target != 20) {
			t.Fatalf("unfrozen CCC = %+v, want active at 20", p)
		}
	}
	targetsSumTo100(t, back)
}

func TestFreezeErrors(t *testing.T) {
	// Freezing an already-frozen position fails.
	frozen, _ := Freeze(base(), "CCC", Redistribution{Scale: true})
	if _, err := Freeze(frozen, "CCC", Redistribution{Scale: true}); err == nil {
		t.Fatal("expected error freezing an already-frozen position")
	}
	// Unfreezing an active position fails.
	if _, err := Unfreeze(base(), "AAA", 10, Redistribution{Scale: true}); err == nil {
		t.Fatal("expected error unfreezing an active position")
	}
	// Freezing the only active position fails.
	solo := Config{Settings: base().Settings, Positions: []Position{{Name: "Only", ISIN: "AAA", Target: 100}}}
	if _, err := Freeze(solo, "AAA", Redistribution{Scale: true}); err == nil {
		t.Fatal("expected error freezing the only active position")
	}
}

func TestEditPosition(t *testing.T) {
	name := "World ex-US"
	min := 75
	got, err := EditPosition(base(), "AAA", &name, &min, false)
	if err != nil {
		t.Fatalf("EditPosition: %v", err)
	}
	if got.Positions[0].Name != "World ex-US" || got.Positions[0].MinMonthly == nil || *got.Positions[0].MinMonthly != 75 {
		t.Fatalf("position = %+v", got.Positions[0])
	}
	// clearMin drops the override.
	cleared, err := EditPosition(got, "AAA", nil, nil, true)
	if err != nil {
		t.Fatalf("EditPosition clear: %v", err)
	}
	if cleared.Positions[0].MinMonthly != nil {
		t.Fatalf("min_monthly = %v, want nil", cleared.Positions[0].MinMonthly)
	}
}

func TestSetSettings(t *testing.T) {
	mon, tol := 500, 1.5
	got := SetSettings(base(), SettingsPatch{Monthly: &mon, Tolerance: &tol})
	if got.Settings.MonthlyTotal != 500 || got.Settings.TolerancePP != 1.5 {
		t.Fatalf("settings = %+v", got.Settings)
	}
	// Unspecified fields are untouched.
	if got.Settings.HorizonMonths != base().Settings.HorizonMonths {
		t.Fatal("SetSettings changed an unspecified field")
	}
}

// TestWriteRoundTrip checks a written config re-loads to an equal value, and that
// an omitted min_monthly stays omitted.
func TestWriteRoundTrip(t *testing.T) {
	min := 50
	c := base()
	c.Positions[1].MinMonthly = &min

	path := filepath.Join(t.TempDir(), "config.toml")
	if err := Write(path, c); err != nil {
		t.Fatalf("Write: %v", err)
	}
	got, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(got.Positions) != 3 || got.Settings != c.Settings {
		t.Fatalf("round-trip settings/positions mismatch: %+v", got)
	}
	if got.Positions[0].MinMonthly != nil {
		t.Fatalf("World min_monthly = %v, want nil", got.Positions[0].MinMonthly)
	}
	if got.Positions[1].MinMonthly == nil || *got.Positions[1].MinMonthly != 50 {
		t.Fatalf("EM min_monthly = %v, want 50", got.Positions[1].MinMonthly)
	}
}
