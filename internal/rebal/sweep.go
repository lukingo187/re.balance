package rebal

// Sweep solves the same input for every horizon N = 1..maxMonths and returns
// the plans in order (index k is N = k+1). Each horizon is independent.
//
// The convergence horizon — first N whose MaxDriftPP is within tolerance — is
// left to the caller; Sweep has no notion of tolerance. A horizon that fails to
// solve yields a zero Plan in its slot rather than dropping the entry, so the
// index-to-N mapping stays intact.
func Sweep(in Input, maxMonths int) []Plan {
	plans := make([]Plan, 0, maxMonths)
	for months := 1; months <= maxMonths; months++ {
		in.Months = months // in is a value copy; this does not mutate the caller
		plan, err := Solve(in)
		if err != nil {
			plans = append(plans, Plan{})
			continue
		}
		plans = append(plans, plan)
	}
	return plans
}
