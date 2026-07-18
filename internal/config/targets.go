package config

import (
	"math"
	"sort"
)

// Scale rescales targets proportionally so they sum to `to`. A zero-sum input
// yields all zeros.
func Scale(targets []float64, to float64) []float64 {
	var sum float64
	for _, t := range targets {
		sum += t
	}
	out := make([]float64, len(targets))
	if sum == 0 {
		return out
	}
	for i, t := range targets {
		out[i] = t * to / sum
	}
	return out
}

// RoundTargets rounds each target to one decimal place using the largest-
// remainder method, so the result sums to exactly 100.0 (never 99.9 or 100.1).
// It assumes the input already sums to ~100.
func RoundTargets(targets []float64) []float64 {
	n := len(targets)
	tenths := make([]int, n) // each target in tenths of a percent
	frac := make([]float64, n)
	total := 0
	for i, t := range targets {
		x := t * 10
		f := math.Floor(x)
		tenths[i] = int(f)
		frac[i] = x - f
		total += int(f)
	}

	// Distribute the residual (in tenths) to reach exactly 1000 (= 100.0%).
	// Positive residual: to the largest discarded fractions. Negative: reclaim
	// from the smallest.
	residual := 1000 - total
	order := make([]int, n)
	for i := range order {
		order[i] = i
	}
	if residual > 0 {
		sort.SliceStable(order, func(a, b int) bool { return frac[order[a]] > frac[order[b]] })
		for k := 0; k < residual && k < n; k++ {
			tenths[order[k]]++
		}
	} else if residual < 0 {
		sort.SliceStable(order, func(a, b int) bool { return frac[order[a]] < frac[order[b]] })
		for k := 0; k < -residual && k < n; k++ {
			tenths[order[k]]--
		}
	}

	out := make([]float64, n)
	for i := range out {
		out[i] = float64(tenths[i]) / 10
	}
	return out
}
