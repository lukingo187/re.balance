package main

import (
	"fmt"
	"io"

	"github.com/lukingo187/re.balance/internal/config"
	"github.com/lukingo187/re.balance/internal/rebal"
	"github.com/spf13/cobra"
)

func newAddCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "add --name NAME --isin ISIN --target T [--scale | --from ISIN[=pp]...] [--min M]",
		Short: "Add a position, making room for its target, and write config",
		Long: "Add a position and rebalance targets to fit it. Make room by rescaling the\n" +
			"others (--scale) or by taking explicit amounts from named positions\n" +
			"(--from ISIN=pp, repeatable; the amounts must sum to the new target). A bare\n" +
			"--from ISIN takes the whole target from that one position.\n\n" +
			"Writes config.toml by default; use --dry-run to preview without writing.",
		RunE: runAdd,
	}
	cmd.Flags().String("name", "", "position name (required)")
	cmd.Flags().String("isin", "", "position ISIN (required)")
	cmd.Flags().Float64("target", 0, "target percentage for the new position (required)")
	cmd.Flags().Bool("scale", false, "rescale existing targets proportionally to make room")
	cmd.Flags().StringArray("from", nil, "take target from this position: ISIN=pp (repeatable) or a bare ISIN")
	cmd.Flags().Int("min", -1, "monthly floor for the new position (default: config default)")
	cmd.Flags().Bool("dry-run", false, "print the result but do not write config")
	return cmd
}

func runAdd(cmd *cobra.Command, _ []string) error {
	name, _ := cmd.Flags().GetString("name")
	isin, _ := cmd.Flags().GetString("isin")
	target, _ := cmd.Flags().GetFloat64("target")
	scale, _ := cmd.Flags().GetBool("scale")
	fromEntries, _ := cmd.Flags().GetStringArray("from")
	minFlag, _ := cmd.Flags().GetInt("min")
	dry, _ := cmd.Flags().GetBool("dry-run")

	if name == "" || isin == "" || target <= 0 {
		return fmt.Errorf("--name, --isin and a positive --target are required")
	}

	cfg, err := loadConfig(cmd)
	if err != nil {
		return err
	}
	st, _, err := loadState(cmd)
	if err != nil {
		return err
	}

	sources, err := parseSources(fromEntries, target)
	if err != nil {
		return err
	}
	r, err := redistribution(scale, sources)
	if err != nil {
		return err
	}

	var min *int
	if minFlag >= 0 {
		min = &minFlag
	}

	after, err := config.AddPosition(cfg, name, isin, target, min, r)
	if err != nil {
		return err
	}
	if issues := rebal.Validate(after.ToInput(st.Values)); len(issues) > 0 {
		return refuseInvalid(cmd, issues)
	}

	return applyConfig(cmd, cfg, after, dry, func(w io.Writer) {
		printConvergence(w, after, normKey(isin), name, st.Values)
	})
}

// printConvergence reports when the (usually fresh, 0 €) position catches up to
// target across the whole portfolio.
func printConvergence(w io.Writer, cfg config.Config, isin, name string, values map[string]int64) {
	in := cfg.ToInput(values)
	if in.Months < 1 {
		in.Months = 1
	}
	tol := cfg.Settings.TolerancePP
	max := cfg.Settings.MaxHorizon
	if max < 1 {
		max = 36
	}
	if n, ok := convergenceHorizon(in, tol, max); ok {
		fmt.Fprintf(w, "%swith %s added, the portfolio is on target (within %s) from month %d.\n",
			margin, name, formatTol(tol), n)
	} else {
		fmt.Fprintf(w, "%swith %s added, the portfolio is not on target within %d months at %d €/month.\n",
			margin, name, max, in.Monthly)
	}
}

// convergenceHorizon returns the first N in 1..max whose max drift is within tol.
func convergenceHorizon(in rebal.Input, tol float64, max int) (int, bool) {
	for n, p := range rebal.Sweep(in, max) {
		if p.MaxDriftPP <= tol {
			return n + 1, true
		}
	}
	return 0, false
}
