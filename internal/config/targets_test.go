package config

import (
	"math"
	"testing"
)

func sumF(xs []float64) float64 {
	var s float64
	for _, x := range xs {
		s += x
	}
	return s
}

// Adding a 10% position to 50/30/20 by scaling gives 45/27/18.
func TestScale_MakeRoom(t *testing.T) {
	got := Scale([]float64{50, 30, 20}, 90)
	want := []float64{45, 27, 18}
	for i := range want {
		if math.Abs(got[i]-want[i]) > 1e-9 {
			t.Fatalf("Scale = %v, want %v", got, want)
		}
	}
}

// Removing a 20% position from 50/30/20 by scaling gives 62.5/37.5.
func TestScale_Redistribute(t *testing.T) {
	got := RoundTargets(Scale([]float64{50, 30}, 100))
	want := []float64{62.5, 37.5}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got %v, want %v", got, want)
		}
	}
}

// Largest-remainder always lands on exactly 100.0, even for inputs that round
// awkwardly (thirds).
func TestRoundTargets_SumsTo100(t *testing.T) {
	cases := [][]float64{
		{100.0 / 3, 100.0 / 3, 100.0 / 3}, // 33.33... each
		{45, 27, 18, 10},                  // sums to 100 already
		{16.666, 33.333, 50.001},
		{62.5, 37.5},
	}
	for _, in := range cases {
		got := RoundTargets(in)
		if s := sumF(got); math.Abs(s-100) > 1e-9 {
			t.Fatalf("RoundTargets(%v) = %v sums to %v, want 100", in, got, s)
		}
		// One decimal place only.
		for _, g := range got {
			if math.Abs(g*10-math.Round(g*10)) > 1e-9 {
				t.Fatalf("%v is not rounded to one decimal", g)
			}
		}
	}
}
