package main

import (
	"github.com/lukingo187/re.balance/internal/config"
	"github.com/lukingo187/re.balance/internal/rebal"
	"github.com/spf13/cobra"
)

func newFreezeCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "freeze <ISIN> [--scale | --to ISIN[=pp]...]",
		Short: "Stop buying a position but keep holding it, and write config",
		Long: "Mark a position wind-down: you keep the holding but never buy it again. Its\n" +
			"target is redistributed to the remaining active positions by rescaling them\n" +
			"(--scale) or by explicit amounts (--to ISIN=pp, must sum to its target). A\n" +
			"bare --to ISIN gives the whole share to that one position.\n\n" +
			"Writes config.toml by default; use --dry-run to preview without writing.",
		Args: cobra.ExactArgs(1),
		RunE: runFreeze,
	}
	cmd.Flags().Bool("scale", false, "rescale remaining targets proportionally to fill 100")
	cmd.Flags().StringArray("to", nil, "give target to this position: ISIN=pp (repeatable) or a bare ISIN")
	cmd.Flags().Bool("dry-run", false, "print the result but do not write config")
	return cmd
}

func runFreeze(cmd *cobra.Command, args []string) error {
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

	var freed float64
	if idx := indexInConfig(cfg, isin); idx >= 0 {
		freed = cfg.Positions[idx].Target
	}
	sources, err := parseSources(toEntries, freed)
	if err != nil {
		return err
	}
	r, err := redistribution(scale, sources)
	if err != nil {
		return err
	}

	after, err := config.Freeze(cfg, isin, r)
	if err != nil {
		return err
	}
	if issues := rebal.Validate(after.ToInput(st.Values)); len(issues) > 0 {
		return refuseInvalid(cmd, issues)
	}
	return applyConfig(cmd, cfg, after, dry, nil)
}
