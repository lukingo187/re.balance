package main

import (
	"fmt"

	"github.com/lukingo187/re.balance/internal/config"
	"github.com/lukingo187/re.balance/internal/rebal"
	"github.com/spf13/cobra"
)

func newUnfreezeCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "unfreeze <ISIN> <target> [--scale | --from ISIN[=pp]...]",
		Short: "Resume buying a wind-down position, and write config",
		Long: "Bring a wind-down position back into the active pool with the given target,\n" +
			"taking room from the others by rescaling them (--scale) or by explicit\n" +
			"amounts (--from ISIN=pp, must sum to the new target).\n\n" +
			"Writes config.toml by default; use --dry-run to preview without writing.",
		Args: cobra.ExactArgs(2),
		RunE: runUnfreeze,
	}
	cmd.Flags().Bool("scale", false, "rescale the other targets proportionally to make room")
	cmd.Flags().StringArray("from", nil, "take target from this position: ISIN=pp (repeatable) or a bare ISIN")
	cmd.Flags().Bool("dry-run", false, "print the result but do not write config")
	return cmd
}

func runUnfreeze(cmd *cobra.Command, args []string) error {
	isin := normKey(args[0])
	target, err := parsePP(args[1])
	if err != nil {
		return fmt.Errorf("target must be a positive number, got %q", args[1])
	}
	scale, _ := cmd.Flags().GetBool("scale")
	fromEntries, _ := cmd.Flags().GetStringArray("from")
	dry, _ := cmd.Flags().GetBool("dry-run")

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

	after, err := config.Unfreeze(cfg, isin, target, r)
	if err != nil {
		return err
	}
	if issues := rebal.Validate(after.ToInput(st.Values)); len(issues) > 0 {
		return refuseInvalid(cmd, issues)
	}
	return applyConfig(cmd, cfg, after, dry, nil)
}
