package main

import (
	"fmt"
	"io"
	"os"

	"github.com/lukingo187/re.balance/internal/money"
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
func printPlan(w io.Writer, in rebal.Input, plan rebal.Plan) {
	var total, activePool int64
	for i, v := range in.Values {
		total += v
		if !in.Positions[i].WindDown {
			activePool += v
		}
	}

	fmt.Fprintf(w, "\n%sPortfolio: %s total · %d €/month · %d-month horizon\n\n",
		margin, money.Euro(total), in.Monthly, in.Months)

	rows := [][]Cell{{
		tinted("Position", bold), tinted("ISIN", bold), tinted("Value", bold),
		tinted("Now", bold), tinted("Target", bold), tinted("Drift", bold), tinted("Plan", bold),
	}}
	for i := range in.Positions {
		p := in.Positions[i]
		val := plain(money.Euro(in.Values[i]))
		switch {
		case p.WindDown:
			rows = append(rows, []Cell{
				plain(p.Name), plain(p.ISIN), val,
				tinted("—", dim), tinted("—", dim), tinted("—", dim), tinted("frozen", dim),
			})
		case activePool == 0:
			// Fresh portfolio: current weight is undefined (0/0), so there is
			// nothing to drift from yet. The plan is still a valid target split.
			rows = append(rows, []Cell{
				plain(p.Name), plain(p.ISIN), val,
				tinted("—", dim), plain(fmt.Sprintf("%.0f%%", p.TargetPct)), tinted("—", dim),
				plain(fmt.Sprintf("%d €", plan.Monthly[i])),
			})
		default:
			nowPct := 100 * float64(in.Values[i]) / float64(activePool)
			drift := nowPct - p.TargetPct
			rows = append(rows, []Cell{
				plain(p.Name), plain(p.ISIN), val,
				plain(fmt.Sprintf("%.1f%%", nowPct)), plain(fmt.Sprintf("%.0f%%", p.TargetPct)),
				tinted(fmt.Sprintf("%+.1fpp", drift), driftColor(drift)),
				plain(fmt.Sprintf("%d €", plan.Monthly[i])),
			})
		}
	}
	table(w, rows, []bool{false, false, true, true, true, true, true})

	fmt.Fprintf(w, "\n%sProjected max drift at month %d: %s\n\n",
		margin, in.Months, formatPP(plan.MaxDriftPP))
}
