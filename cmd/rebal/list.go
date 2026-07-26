package main

import (
	"fmt"

	"github.com/lukingo187/re.balance/internal/money"
	"github.com/lukingo187/re.balance/internal/rebal"
	"github.com/spf13/cobra"
)

func newListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "Show positions, targets and current drift (no plan)",
		Long: "List every position with its target, current value, weight and drift. Unlike\n" +
			"'plan' it computes no contribution split and never fails on an invalid config —\n" +
			"it is a plain view of what you hold versus what you want.",
		Args: cobra.NoArgs,
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

			var total, activePool int64
			for i, v := range in.Values {
				total += v
				if !in.Positions[i].WindDown {
					activePool += v
				}
			}

			out := cmd.OutOrStdout()
			fmt.Fprintf(out, "\n%sPortfolio: %s total · %d €/month · target %d-month horizon\n\n",
				margin, money.Euro(total), in.Monthly, in.Months)

			rows := [][]Cell{{
				tinted("Position", bold), tinted("ISIN", bold), tinted("Target", bold),
				tinted("Value", bold), tinted("Now", bold), tinted("Drift", bold),
			}}
			for i := range in.Positions {
				p := in.Positions[i]
				val := plain(money.Euro(in.Values[i]))
				switch {
				case p.WindDown:
					rows = append(rows, []Cell{
						plain(p.Name), plain(p.ISIN), tinted("frozen", dim), val,
						tinted("—", dim), tinted("—", dim),
					})
				case activePool == 0:
					rows = append(rows, []Cell{
						plain(p.Name), plain(p.ISIN), plain(fmt.Sprintf("%.0f%%", p.TargetPct)),
						val, tinted("—", dim), tinted("—", dim),
					})
				default:
					nowPct := 100 * float64(in.Values[i]) / float64(activePool)
					drift := nowPct - p.TargetPct
					rows = append(rows, []Cell{
						plain(p.Name), plain(p.ISIN), plain(fmt.Sprintf("%.0f%%", p.TargetPct)),
						val, plain(fmt.Sprintf("%.1f%%", nowPct)),
						tinted(fmt.Sprintf("%+.1fpp", drift), driftColor(drift)),
					})
				}
			}
			table(out, rows, []bool{false, false, true, true, true, true})

			// Non-fatal: surface config problems as a hint without blocking the view.
			if issues := rebal.Validate(in); len(issues) > 0 {
				fmt.Fprintf(out, "\n%s%s\n", margin,
					dim(fmt.Sprintf("note: %d issue(s) — run 'rebal check'", len(issues))))
			}
			fmt.Fprintln(out)
			return nil
		},
	}
}
