package main

import (
	"testing"

	"github.com/lukingo187/re.balance/internal/rebal"
)

// TestCheckRemovable exercises the safety gate for `push confirm --remove`
// without ever calling the real (irreversible, no-preview) sc remove
// command — that's the whole point of pulling this logic out on its own.
func TestCheckRemovable(t *testing.T) {
	active := rebal.Position{ISIN: "AAA", WindDown: false}
	frozen := rebal.Position{ISIN: "BBB", WindDown: true}
	plansWithBBB := []savingsPlan{{ISIN: "BBB", Amount: 25, Frequency: "MONTHLY"}}

	t.Run("refuses a non-wind-down position outright", func(t *testing.T) {
		_, err := checkRemovable(active, "AAA", plansWithBBB)
		if err == nil {
			t.Fatal("want error for a position that isn't frozen in rebal")
		}
	})

	t.Run("frozen position with no active plan: no error, hasPlan false", func(t *testing.T) {
		hasPlan, err := checkRemovable(frozen, "BBB", nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if hasPlan {
			t.Error("hasPlan = true, want false — no plans were given")
		}
	})

	t.Run("frozen position with an active plan: removable", func(t *testing.T) {
		hasPlan, err := checkRemovable(frozen, "BBB", plansWithBBB)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !hasPlan {
			t.Error("hasPlan = false, want true — BBB has an active plan")
		}
	})

	t.Run("ISIN matching is case-insensitive", func(t *testing.T) {
		hasPlan, err := checkRemovable(frozen, "bbb", plansWithBBB)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !hasPlan {
			t.Error("hasPlan = false, want true for a lowercase ISIN match")
		}
	})
}

// TestCheckOverridable exercises the safety gate for --amount without ever
// calling sc.
func TestCheckOverridable(t *testing.T) {
	in := rebal.Input{Positions: []rebal.Position{
		{ISIN: "AAA", WindDown: false},
		{ISIN: "BBB", WindDown: true},
	}}

	t.Run("allows a tracked, active position", func(t *testing.T) {
		if err := checkOverridable(in, "AAA"); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("ISIN matching is case-insensitive", func(t *testing.T) {
		if err := checkOverridable(in, "aaa"); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("refuses a frozen position", func(t *testing.T) {
		if err := checkOverridable(in, "BBB"); err == nil {
			t.Error("want error for a frozen position — should point to --remove instead")
		}
	})

	t.Run("refuses an untracked ISIN", func(t *testing.T) {
		if err := checkOverridable(in, "ZZZ"); err == nil {
			t.Error("want error for an ISIN not in config.toml")
		}
	})
}
