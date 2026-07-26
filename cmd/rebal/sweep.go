package main

import (
	"fmt"

	"github.com/lukingo187/re.balance/internal/rebal"
	"github.com/lukingo187/re.balance/internal/ui/chart"
	"github.com/spf13/cobra"
)

func newSweepCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "sweep [--max N]",
		Short: "Show the horizon chart: max drift vs. months",
		Long:  "Solve for every horizon up to --max and chart the resulting max drift, so\nthe point where the curve meets the tolerance is the convergence horizon.",
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
			if issues := rebal.Validate(in); len(issues) > 0 {
				for _, is := range issues {
					fmt.Fprintf(cmd.ErrOrStderr(), "  %s: %s\n", is.Severity, is.Message)
				}
				return fmt.Errorf("cannot sweep until issues are fixed")
			}

			max := cfg.Settings.MaxHorizon
			if m, _ := cmd.Flags().GetInt("max"); m > 0 {
				max = m
			}
			if max < 1 {
				max = 36
			}

			plans := rebal.Sweep(in, max)
			s := chart.Series{X: make([]int, max), Y: make([]float64, max)}
			for i := range plans {
				s.X[i] = i + 1
				s.Y[i] = plans[i].MaxDriftPP
			}

			out := cmd.OutOrStdout()
			fmt.Fprintf(out, "\n%s%d €/month · tolerance %s\n\n", margin, in.Monthly, formatTol(cfg.Settings.TolerancePP))
			fmt.Fprintln(out, chart.Render(s, chart.Opts{Tolerance: cfg.Settings.TolerancePP}))
			return nil
		},
	}
	cmd.Flags().Int("max", 0, "highest horizon to sweep (default from config)")
	return cmd
}
