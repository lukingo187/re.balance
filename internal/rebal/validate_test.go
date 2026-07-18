package rebal

import (
	"strings"
	"testing"
)

// baseInput is a known-valid portfolio (the spec's 50/30/20 example) that each
// test mutates to isolate one violation.
func baseInput() Input {
	return Input{
		Positions: []Position{
			{Name: "MSCI World", ISIN: "IE00B4L5Y983", TargetPct: 50, MinMonthly: 25},
			{Name: "MSCI EM IMI", ISIN: "IE00BKM4GZ66", TargetPct: 30, MinMonthly: 25},
			{Name: "Europe SC", ISIN: "IE00B3VWMM18", TargetPct: 20, MinMonthly: 25},
		},
		Values:  []int64{1245000, 731050, 437000},
		Monthly: 400,
		Months:  12,
	}
}

func hasIssue(issues []Issue, substr string) bool {
	for _, is := range issues {
		if strings.Contains(is.Message, substr) {
			return true
		}
	}
	return false
}

func TestValidate_BaseIsClean(t *testing.T) {
	if issues := Validate(baseInput()); len(issues) != 0 {
		t.Fatalf("expected no issues, got %v", issues)
	}
}

// Tests 7 and 13: every fatal condition triggers its message.
func TestValidate_FatalConditions(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*Input)
		want   string
	}{
		{"targets not 100", func(in *Input) { in.Positions[0].TargetPct = 55 }, "sum to 100"},
		{"floors exceed budget", func(in *Input) { in.Monthly = 50 }, "floors exceed"},
		{"never converges", func(in *Input) { in.Positions[2].MinMonthly = 100 }, "never converges"},
		{"duplicate ISIN", func(in *Input) { in.Positions[1].ISIN = in.Positions[0].ISIN }, "duplicate ISIN"},
		{"empty name", func(in *Input) { in.Positions[0].Name = "" }, "empty name"},
		{"months below one", func(in *Input) { in.Months = 0 }, "at least 1"},
		{
			// 0/80/20 still sums to 100, isolating the "must be positive" rule.
			"non-positive target",
			func(in *Input) { in.Positions[0].TargetPct = 0; in.Positions[1].TargetPct = 80 },
			"must be positive",
		},
		{
			"wind-down with floor",
			func(in *Input) {
				in.Positions[0].TargetPct, in.Positions[1].TargetPct = 50, 50
				in.Positions[2].WindDown, in.Positions[2].TargetPct = true, 0 // keeps MinMonthly 25
			},
			"cannot have a monthly floor",
		},
		{
			"wind-down with target",
			func(in *Input) {
				in.Positions[0].TargetPct, in.Positions[1].TargetPct = 50, 50
				in.Positions[2].WindDown, in.Positions[2].MinMonthly = true, 0 // keeps TargetPct 20
			},
			"cannot have a target",
		},
		{
			"all frozen",
			func(in *Input) {
				for i := range in.Positions {
					in.Positions[i].WindDown = true
					in.Positions[i].TargetPct, in.Positions[i].MinMonthly = 0, 0
				}
			},
			"at least one position must be active",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			in := baseInput()
			tt.mutate(&in)
			issues := Validate(in)
			if !hasIssue(issues, tt.want) {
				t.Fatalf("expected an issue containing %q, got %v", tt.want, issues)
			}
		})
	}
}
