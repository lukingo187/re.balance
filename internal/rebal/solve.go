package rebal

import "math"

// Solve allocates Input.Monthly euros across the active positions so their
// projected end values are as close as possible to their targets, subject to
// per-position floors. It is the classic separable-convex resource-allocation
// result: each marginal euro goes to whoever is furthest below target.
//
// Wind-down positions are filtered out entirely — they never enter the math and
// come back with Monthly 0, unchanged EndValue, and NaN EndPct.
//
// Solve does not run full validation; call Validate for that. It guards only
// the two conditions it cannot proceed past.
func Solve(in Input) (Plan, error) {
	n := len(in.Positions)
	plan := Plan{
		Monthly:  make([]int, n),
		EndValue: make([]int64, n),
		EndPct:   make([]float64, n),
	}

	// Active = the positions still being bought.
	active := make([]int, 0, n)
	for i := range in.Positions {
		if !in.Positions[i].WindDown {
			active = append(active, i)
		}
	}
	if len(active) == 0 {
		return Plan{}, ErrNoActivePositions
	}

	// S = current active value; T = active pool value at month N (cents).
	// Since the c[i] below sum to Monthly, the active EndValues sum to exactly T.
	var s int64
	for _, i := range active {
		s += in.Values[i]
	}
	t := s + int64(in.Months*in.Monthly)*100

	// Start every active position at its floor; distribute the remainder.
	c := make([]int, n) // euros/month; wind-down positions stay 0
	floorSum := 0
	for _, i := range active {
		c[i] = in.Positions[i].MinMonthly
		floorSum += c[i]
	}
	rem := in.Monthly - floorSum
	if rem < 0 {
		return Plan{}, ErrFloorsExceedBudget
	}

	// Incremental greedy: give each euro to the active position with the
	// smallest (projected end value − target end value). Ties break to the
	// lower index, which falls out of iterating active in ascending order with
	// a strict-less comparison.
	for k := 0; k < rem; k++ {
		best := -1
		var bestCost float64
		for _, i := range active {
			w := in.Positions[i].TargetPct / 100
			cost := float64(in.Values[i]+int64(in.Months*c[i])*100) - w*float64(t)
			if best == -1 || cost < bestCost {
				best, bestCost = i, cost
			}
		}
		c[best]++
	}

	// Project results back onto the full, position-parallel arrays.
	for i := range in.Positions {
		plan.Monthly[i] = c[i]
		plan.EndValue[i] = in.Values[i] + int64(in.Months*c[i])*100
	}
	for i := range in.Positions {
		if in.Positions[i].WindDown {
			plan.EndPct[i] = math.NaN() // undefined: outside the active pool
			continue
		}
		plan.EndPct[i] = 100 * float64(plan.EndValue[i]) / float64(t)
	}

	// Max drift is measured over the active pool only.
	for _, i := range active {
		if d := math.Abs(plan.EndPct[i] - in.Positions[i].TargetPct); d > plan.MaxDriftPP {
			plan.MaxDriftPP = d
		}
	}

	return plan, nil
}
