package main

import (
	"fmt"
	"math"
	"os"
	"text/tabwriter"

	"github.com/lukingo187/re.balance/internal/rebal"
	"github.com/spf13/cobra"
)

func newPlanCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "plan",
		Short: "Show the contribution plan for one horizon",
		Long:  "Compute how to split the monthly contribution across positions so the\nportfolio converges toward its targets over the chosen horizon.",
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg, err := loadConfig(cmd)
			if err != nil {
				return err
			}
			st, _, err := loadState(cmd)
			if err != nil {
				return err
			}
			in := cfg.ToInput(st.Values)
			if m, _ := cmd.Flags().GetInt("months"); m > 0 {
				in.Months = m
			}
			if c, _ := cmd.Flags().GetInt("monthly"); c > 0 {
				in.Monthly = c
			}

			if issues := rebal.Validate(in); len(issues) > 0 {
				fmt.Fprintln(os.Stderr, "config has fatal issues:")
				for _, is := range issues {
					fmt.Fprintf(os.Stderr, "  %s: %s\n", is.Severity, is.Message)
				}
				return fmt.Errorf("cannot plan until issues are fixed")
			}

			plan, err := rebal.Solve(in)
			if err != nil {
				return err
			}
			printPlan(cmd.OutOrStdout(), in, plan)
			return nil
		},
	}
	cmd.Flags().Int("months", 0, "horizon in months (default from config)")
	cmd.Flags().Int("monthly", 0, "total euros/month to invest (default from config)")
	return cmd
}

// printPlan renders the plan table from spec §8. "Now" and "Target" are shares
// of the active pool; frozen positions show em-dashes and a "frozen" plan.
func printPlan(w interface{ Write([]byte) (int, error) }, in rebal.Input, plan rebal.Plan) {
	var total, activePool int64
	for i, v := range in.Values {
		total += v
		if !in.Positions[i].WindDown {
			activePool += v
		}
	}

	fmt.Fprintf(w, "\n  Portfolio: %s total · %d €/month · %d-month horizon\n\n",
		formatEuro(total), in.Monthly, in.Months)

	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', tabwriter.AlignRight)
	fmt.Fprintln(tw, "  Position\tISIN\tValue\tNow\tTarget\tDrift\tPlan\t")
	for i := range in.Positions {
		p := in.Positions[i]
		if p.WindDown {
			fmt.Fprintf(tw, "  %s\t%s\t%s\t—\t—\t—\tfrozen\t\n", p.Name, p.ISIN, formatEuro(in.Values[i]))
			continue
		}
		if activePool == 0 {
			// Fresh portfolio: current weight is undefined (0/0), so there is
			// nothing to drift from yet. The plan is still a valid target split.
			fmt.Fprintf(tw, "  %s\t%s\t%s\t—\t%.0f%%\t—\t%d €\t\n",
				p.Name, p.ISIN, formatEuro(in.Values[i]), p.TargetPct, plan.Monthly[i])
			continue
		}
		nowPct := 100 * float64(in.Values[i]) / float64(activePool)
		drift := nowPct - p.TargetPct
		fmt.Fprintf(tw, "  %s\t%s\t%s\t%.1f%%\t%.0f%%\t%+.1fpp\t%d €\t\n",
			p.Name, p.ISIN, formatEuro(in.Values[i]), nowPct, p.TargetPct, drift, plan.Monthly[i])
	}
	tw.Flush()

	fmt.Fprintf(w, "\n  Projected max drift at month %d: %s\n\n",
		in.Months, driftLabel(plan.MaxDriftPP))
}

func driftLabel(pp float64) string {
	if math.IsNaN(pp) {
		return "n/a"
	}
	return fmt.Sprintf("±%.2fpp", pp)
}
