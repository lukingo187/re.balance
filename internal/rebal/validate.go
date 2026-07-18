package rebal

import (
	"fmt"
	"math"
)

// Validate checks an Input against every rule in the spec (§4) plus the
// wind-down rules, scoping the portfolio rules to active positions. It returns
// all issues found and never panics. An empty result means the input is safe to
// Solve.
func Validate(in Input) []Issue {
	var issues []Issue
	add := func(format string, a ...any) {
		issues = append(issues, Issue{Fatal, fmt.Sprintf(format, a...)})
	}

	if in.Months < 1 {
		add("horizon months must be at least 1")
	}

	seen := make(map[string]bool)
	activeCount := 0
	var targetSum float64
	for i := range in.Positions {
		p := in.Positions[i]
		if p.Name == "" {
			add("position %d has an empty name", i)
		}
		if p.ISIN != "" {
			if seen[p.ISIN] {
				add("duplicate ISIN: %s", p.ISIN)
			}
			seen[p.ISIN] = true
		}

		if p.WindDown {
			// Wind-down positions carry neither a target nor a floor; a nonzero
			// value here means the config half-says "freeze" and half-says "buy".
			if p.MinMonthly != 0 {
				add("wind-down position cannot have a monthly floor: %s", p.Name)
			}
			if p.TargetPct != 0 {
				add("wind-down position cannot have a target: %s", p.Name)
			}
			continue
		}

		activeCount++
		targetSum += p.TargetPct
		if p.TargetPct <= 0 {
			add("target must be positive: %s", p.Name)
		}
	}

	if activeCount == 0 {
		add("at least one position must be active")
		return issues // remaining checks are meaningless with no active pool
	}

	if math.Abs(targetSum-100) > 1e-6 {
		add("active targets must sum to 100 (got %.4g)", targetSum)
	}

	floorSum := 0
	for i := range in.Positions {
		if !in.Positions[i].WindDown {
			floorSum += in.Positions[i].MinMonthly
		}
	}
	if floorSum > in.Monthly {
		add("floors exceed monthly budget (%d > %d)", floorSum, in.Monthly)
	}

	// Never-converges: a floor above the position's long-run share drifts it
	// permanently overweight. Strict comparison — an equal floor still converges.
	for i := range in.Positions {
		p := in.Positions[i]
		if p.WindDown {
			continue
		}
		share := p.TargetPct / 100 * float64(in.Monthly)
		if float64(p.MinMonthly) > share+1e-9 {
			add("position never converges — floor %d exceeds long-run share %.1f: %s",
				p.MinMonthly, share, p.Name)
		}
	}

	return issues
}
