package main

import (
	"math"
	"testing"

	"github.com/lukingo187/re.balance/internal/config"
	"github.com/lukingo187/re.balance/internal/state"
)

func targetsSumTo100(t *testing.T, c config.Config) {
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

func targetOf(t *testing.T, c config.Config, isin string) float64 {
	t.Helper()
	for _, p := range c.Positions {
		if p.ISIN == isin {
			return p.Target
		}
	}
	t.Fatalf("no position with ISIN %s", isin)
	return 0
}

func TestAddMissingPositionsBootstrap(t *testing.T) {
	missing := []holding{
		{ISIN: "AAA", Name: "World", Valuation: 6000},
		{ISIN: "BBB", Name: "EM", Valuation: 3000},
		{ISIN: "CCC", Name: "Europe", Valuation: 1000},
	}
	out, err := addMissingPositions(config.Config{}, missing, state.State{Values: map[string]int64{}})
	if err != nil {
		t.Fatalf("addMissingPositions: %v", err)
	}
	targetsSumTo100(t, out)
	if got := targetOf(t, out, "AAA"); math.Abs(got-60) > 1e-9 {
		t.Errorf("AAA target = %v, want 60", got)
	}
	if got := targetOf(t, out, "BBB"); math.Abs(got-30) > 1e-9 {
		t.Errorf("BBB target = %v, want 30", got)
	}
	if got := targetOf(t, out, "CCC"); math.Abs(got-10) > 1e-9 {
		t.Errorf("CCC target = %v, want 10", got)
	}
}

func TestAddMissingPositionsIntoExisting(t *testing.T) {
	cfg := config.Config{
		Positions: []config.Position{
			{Name: "World", ISIN: "AAA", Target: 60},
			{Name: "EM", ISIN: "BBB", Target: 30},
		},
	}
	st := state.State{Values: map[string]int64{
		"AAA": 600000, // 6 000 €
		"BBB": 300000, // 3 000 €
	}}
	missing := []holding{{ISIN: "CCC", Name: "Europe", Valuation: 1000}} // 1 000 €, 10% of 10 000 total

	out, err := addMissingPositions(cfg, missing, st)
	if err != nil {
		t.Fatalf("addMissingPositions: %v", err)
	}
	targetsSumTo100(t, out)
	if got := targetOf(t, out, "CCC"); math.Abs(got-10) > 1e-9 {
		t.Errorf("CCC target = %v, want 10", got)
	}
	// AAA:BBB were 60:30 (2:1) before; scaling to fill the remaining 90 keeps
	// that ratio.
	if got := targetOf(t, out, "AAA"); math.Abs(got-60) > 1e-9 {
		t.Errorf("AAA target = %v, want 60", got)
	}
	if got := targetOf(t, out, "BBB"); math.Abs(got-30) > 1e-9 {
		t.Errorf("BBB target = %v, want 30", got)
	}
}

func TestAddMissingPositionsZeroTotal(t *testing.T) {
	_, err := addMissingPositions(config.Config{}, nil, state.State{Values: map[string]int64{}})
	if err == nil {
		t.Fatal("want error for zero total portfolio value")
	}
}

func TestExtractErrorMessage(t *testing.T) {
	cases := []struct {
		name string
		raw  string
		want string
	}{
		{"message field", `{"ok":false,"message":"not logged in"}`, "not logged in"},
		{"error string", `{"ok":false,"error":"session expired"}`, "session expired"},
		{"error object", `{"ok":false,"error":{"message":"rate limited"}}`, "rate limited"},
		{"no known field", `{"ok":false}`, ""},
		{"invalid json", `not json`, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := extractErrorMessage([]byte(tc.raw)); got != tc.want {
				t.Errorf("extractErrorMessage(%q) = %q, want %q", tc.raw, got, tc.want)
			}
		})
	}
}
