package rebal

import (
	"math"
	"math/rand"
	"testing"
)

func sumInts(xs []int) int {
	total := 0
	for _, x := range xs {
		total += x
	}
	return total
}

// mustSolve fails the test if Solve returns an error.
func mustSolve(t *testing.T, in Input) Plan {
	t.Helper()
	p, err := Solve(in)
	if err != nil {
		t.Fatalf("Solve returned error: %v", err)
	}
	return p
}

// Tests 1 and 2: over a fuzz of inputs, the allocation sums to Monthly exactly
// and every position gets at least its floor.
func TestSolve_SumAndFloors(t *testing.T) {
	rng := rand.New(rand.NewSource(1))
	for iter := 0; iter < 2000; iter++ {
		floor := rng.Intn(40)               // shared floor 0..39
		monthly := 3*floor + rng.Intn(2000) // guaranteed >= sum of floors
		in := Input{
			Positions: []Position{
				{Name: "A", ISIN: "A", TargetPct: 50, MinMonthly: floor},
				{Name: "B", ISIN: "B", TargetPct: 30, MinMonthly: floor},
				{Name: "C", ISIN: "C", TargetPct: 20, MinMonthly: floor},
			},
			Values:  []int64{rng.Int63n(5_000_000), rng.Int63n(5_000_000), rng.Int63n(5_000_000)},
			Monthly: monthly,
			Months:  1 + rng.Intn(36),
		}
		p := mustSolve(t, in)

		if got := sumInts(p.Monthly); got != in.Monthly {
			t.Fatalf("iter %d: sum(Monthly)=%d, want %d", iter, got, in.Monthly)
		}
		for i, m := range p.Monthly {
			if m < in.Positions[i].MinMonthly {
				t.Fatalf("iter %d: Monthly[%d]=%d below floor %d", iter, i, m, in.Positions[i].MinMonthly)
			}
		}
	}
}

// Test 3: an already-balanced portfolio splits contributions along the targets.
func TestSolve_BalancedTracksTargets(t *testing.T) {
	// Values are exactly at 50/30/20, so the plan should mirror that split.
	in := Input{
		Positions: []Position{
			{Name: "A", ISIN: "A", TargetPct: 50},
			{Name: "B", ISIN: "B", TargetPct: 30},
			{Name: "C", ISIN: "C", TargetPct: 20},
		},
		Values:  []int64{5_000_000, 3_000_000, 2_000_000},
		Monthly: 400,
		Months:  12,
	}
	p := mustSolve(t, in)
	want := []int{200, 120, 80}
	for i, w := range want {
		if diff := p.Monthly[i] - w; diff < -1 || diff > 1 {
			t.Fatalf("Monthly[%d]=%d, want ~%d", i, p.Monthly[i], w)
		}
	}
}

// Test 4: a wildly overweight position stays parked at its floor.
func TestSolve_OverweightParksAtFloor(t *testing.T) {
	in := Input{
		Positions: []Position{
			{Name: "A", ISIN: "A", TargetPct: 50, MinMonthly: 25}, // hugely overweight
			{Name: "B", ISIN: "B", TargetPct: 30, MinMonthly: 25},
			{Name: "C", ISIN: "C", TargetPct: 20, MinMonthly: 25},
		},
		Values:  []int64{50_000_000, 100_000, 100_000},
		Monthly: 400,
		Months:  12,
	}
	p := mustSolve(t, in)
	if p.Monthly[0] != 25 {
		t.Fatalf("overweight position got %d, want floor 25", p.Monthly[0])
	}
}

// Test 5: a fresh position at 0 absorbs everything above the other floors.
func TestSolve_FreshPositionAbsorbs(t *testing.T) {
	in := Input{
		Positions: []Position{
			{Name: "A", ISIN: "A", TargetPct: 50, MinMonthly: 25},
			{Name: "B", ISIN: "B", TargetPct: 30, MinMonthly: 25},
			{Name: "C", ISIN: "C", TargetPct: 20, MinMonthly: 0}, // brand new
		},
		Values:  []int64{6_000_000, 3_600_000, 0},
		Monthly: 400,
		Months:  12,
	}
	p := mustSolve(t, in)
	if p.Monthly[2] != 350 { // 400 - 25 - 25
		t.Fatalf("fresh position got %d, want 350", p.Monthly[2])
	}
}

// Test 6: symmetric input is deterministic and ties break to the lower index.
func TestSolve_SymmetricDeterministic(t *testing.T) {
	in := Input{
		Positions: []Position{
			{Name: "A", ISIN: "A", TargetPct: 50},
			{Name: "B", ISIN: "B", TargetPct: 50},
		},
		Values:  []int64{1_000_000, 1_000_000},
		Monthly: 401, // odd, so the tie-break is observable
		Months:  12,
	}
	first := mustSolve(t, in)
	second := mustSolve(t, in)

	for i := range first.Monthly {
		if first.Monthly[i] != second.Monthly[i] {
			t.Fatalf("non-deterministic at %d: %d vs %d", i, first.Monthly[i], second.Monthly[i])
		}
	}
	if first.Monthly[0] != 201 || first.Monthly[1] != 200 {
		t.Fatalf("tie-break wrong: got %v, want [201 200]", first.Monthly)
	}
}

// Test 8: MaxDriftPP is non-increasing in N across the sweep, within an epsilon
// that absorbs whole-euro rounding wobble.
func TestSweep_DriftNonIncreasing(t *testing.T) {
	plans := Sweep(baseInput(), 36)
	const eps = 0.5 // pp; generous vs. the ~0.04pp a 1 €/mo/position shift costs
	for n := 1; n < len(plans); n++ {
		if plans[n].MaxDriftPP > plans[n-1].MaxDriftPP+eps {
			t.Fatalf("drift rose at N=%d: %.4f > %.4f", n+1, plans[n].MaxDriftPP, plans[n-1].MaxDriftPP)
		}
	}
}

// --- Wind-down tests (design doc §6) ---

// windDownInput freezes Europe SC; the remaining active targets sum to 100.
func windDownInput() Input {
	return Input{
		Positions: []Position{
			{Name: "MSCI World", ISIN: "A", TargetPct: 62.5},
			{Name: "MSCI EM IMI", ISIN: "B", TargetPct: 37.5},
			{Name: "Europe SC", ISIN: "C", WindDown: true},
		},
		Values:  []int64{1245000, 731050, 437000},
		Monthly: 400,
		Months:  12,
	}
}

// Test 9: a frozen position gets 0, and the total still equals Monthly.
func TestSolve_FrozenGetsZero(t *testing.T) {
	p := mustSolve(t, windDownInput())
	if p.Monthly[2] != 0 {
		t.Fatalf("frozen position got %d, want 0", p.Monthly[2])
	}
	if got := sumInts(p.Monthly); got != 400 {
		t.Fatalf("sum(Monthly)=%d, want 400", got)
	}
}

// Test 10: a frozen position's value passes through and its EndPct is NaN.
func TestSolve_FrozenValueUntouched(t *testing.T) {
	in := windDownInput()
	p := mustSolve(t, in)
	if p.EndValue[2] != in.Values[2] {
		t.Fatalf("frozen EndValue=%d, want %d", p.EndValue[2], in.Values[2])
	}
	if !math.IsNaN(p.EndPct[2]) {
		t.Fatalf("frozen EndPct=%v, want NaN", p.EndPct[2])
	}
}

// Test 11: drift ignores a hugely overweight frozen position.
func TestSolve_DriftIgnoresFrozen(t *testing.T) {
	in := windDownInput()
	in.Values[2] = 500_000_000 // frozen position dominates the real total
	p := mustSolve(t, in)
	if math.IsNaN(p.MaxDriftPP) || p.MaxDriftPP > 1.0 {
		t.Fatalf("MaxDriftPP=%v; frozen position leaked into drift", p.MaxDriftPP)
	}
}

// Test 12: active positions converge to their targets against the active pool.
func TestSolve_ActiveConvergesAgainstPool(t *testing.T) {
	in := windDownInput()
	in.Months = 36
	p := mustSolve(t, in)
	if diff := p.EndPct[0] - 62.5; math.Abs(diff) > 0.5 {
		t.Fatalf("EndPct[0]=%.2f, want ~62.5", p.EndPct[0])
	}
	if diff := p.EndPct[1] - 37.5; math.Abs(diff) > 0.5 {
		t.Fatalf("EndPct[1]=%.2f, want ~37.5", p.EndPct[1])
	}
}

// Test 14: freezing a position yields the same active plan as deleting it.
func TestSolve_FreezeEqualsDelete(t *testing.T) {
	frozen := mustSolve(t, windDownInput())

	deleted := mustSolve(t, Input{
		Positions: []Position{
			{Name: "MSCI World", ISIN: "A", TargetPct: 62.5},
			{Name: "MSCI EM IMI", ISIN: "B", TargetPct: 37.5},
		},
		Values:  []int64{1245000, 731050},
		Monthly: 400,
		Months:  12,
	})

	if frozen.Monthly[0] != deleted.Monthly[0] || frozen.Monthly[1] != deleted.Monthly[1] {
		t.Fatalf("freeze != delete: frozen=%v deleted=%v", frozen.Monthly[:2], deleted.Monthly)
	}
	if math.Abs(frozen.MaxDriftPP-deleted.MaxDriftPP) > 1e-9 {
		t.Fatalf("drift differs: frozen=%.6f deleted=%.6f", frozen.MaxDriftPP, deleted.MaxDriftPP)
	}
}
