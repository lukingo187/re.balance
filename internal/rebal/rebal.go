// Package rebal is the re.balance engine: given current ETF values, target
// weights, and a fixed monthly contribution, it computes how to split that
// contribution so the portfolio converges back to target over a horizon —
// without ever selling.
//
// This package is pure: it imports nothing outside the standard library and
// holds no I/O, config, or UI. The CLI and TUI are skins over it.
package rebal

import "errors"

// Position is one holding. TargetPct is written as in config (50.0, not 0.5)
// and is a share of the ACTIVE pool — a wind-down position is outside that pool
// and carries no target.
type Position struct {
	Name       string
	ISIN       string
	TargetPct  float64 // share of the active pool; 0 and unused when WindDown
	MinMonthly int     // euros/month floor for this plan; must be 0 when WindDown
	WindDown   bool    // hold forever, never buy
}

// Input is a complete problem. Values is in cents and runs parallel to
// Positions. Monthly is whole euros (brokers require it); Months is the horizon.
type Input struct {
	Positions []Position
	Values    []int64 // cents, current market value, parallel to Positions
	Monthly   int     // euros/month, total across all plans
	Months    int     // horizon N
}

// Plan is a solution. All slices run parallel to Input.Positions; a wind-down
// position is present with Monthly 0, EndValue unchanged, and EndPct NaN.
type Plan struct {
	Monthly    []int     // euros/month per position, sums exactly to Input.Monthly
	EndValue   []int64   // projected cents at month N
	EndPct     []float64 // projected weight at month N; NaN for wind-down
	MaxDriftPP float64   // max |EndPct - TargetPct| across ACTIVE positions
}

// Severity classifies a validation Issue. Only Fatal exists today; the type
// leaves room for a Warning tier without reshaping callers.
type Severity int

const (
	Fatal Severity = iota
)

func (s Severity) String() string {
	if s == Fatal {
		return "fatal"
	}
	return "unknown"
}

// Issue is one validation problem. Validate returns these; it never panics.
type Issue struct {
	Severity Severity
	Message  string
}

// Errors Solve can return. These are the two conditions it cannot compute past;
// everything else is caught by Validate before solving.
var (
	ErrFloorsExceedBudget = errors.New("floors exceed monthly budget")
	ErrNoActivePositions  = errors.New("at least one position must be active")
)
