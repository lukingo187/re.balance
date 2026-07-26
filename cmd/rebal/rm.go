package main

import (
	"github.com/lukingo187/re.balance/internal/config"
	"github.com/lukingo187/re.balance/internal/rebal"
	"github.com/spf13/cobra"
)

func newRmCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "rm <ISIN> [--scale | --to ISIN[=pp]...]",
		Short: "Remove a position, giving its target back, and write config",
		Long: "Remove a position and rebalance targets to fill the gap. Give its share back\n" +
			"by rescaling the others (--scale) or by explicit amounts to named positions\n" +
			"(--to ISIN=pp, repeatable; the amounts must sum to the removed target). A bare\n" +
			"--to ISIN gives the whole share to that one position.\n\n" +
			"Writes config.toml by default; use --dry-run to preview without writing.",
		Args: cobra.ExactArgs(1),
		RunE: runRm,
	}
	cmd.Flags().Bool("scale", false, "rescale remaining targets proportionally to fill 100")
	cmd.Flags().StringArray("to", nil, "give target to this position: ISIN=pp (repeatable) or a bare ISIN")
	cmd.Flags().Bool("dry-run", false, "print the result but do not write config")
	return cmd
}

func runRm(cmd *cobra.Command, args []string) error {
	isin := normKey(args[0])
	scale, _ := cmd.Flags().GetBool("scale")
	toEntries, _ := cmd.Flags().GetStringArray("to")
	dry, _ := cmd.Flags().GetBool("dry-run")

	cfg, err := loadConfig(cmd)
	if err != nil {
		return err
	}
	st, _, err := loadState(cmd)
	if err != nil {
		return err
	}

	// A wind-down position has no target, so no --scale/--to is needed to remove
	// it. Otherwise its share must be redistributed. The removed target is what
	// bare --to should absorb.
	var removedTarget float64
	if idx := indexInConfig(cfg, isin); idx >= 0 {
		removedTarget = cfg.Positions[idx].Target
	}

	sources, err := parseSources(toEntries, removedTarget)
	if err != nil {
		return err
	}
	r, err := redistribution(scale, sources)
	if err != nil {
		return err
	}

	after, err := config.RemovePosition(cfg, isin, r)
	if err != nil {
		return err
	}
	if issues := rebal.Validate(after.ToInput(st.Values)); len(issues) > 0 {
		return refuseInvalid(cmd, issues)
	}

	return applyConfig(cmd, cfg, after, dry, nil)
}

// indexInConfig returns the index of a position by ISIN, or -1.
func indexInConfig(cfg config.Config, isin string) int {
	for i, p := range cfg.Positions {
		if normKey(p.ISIN) == normKey(isin) {
			return i
		}
	}
	return -1
}
