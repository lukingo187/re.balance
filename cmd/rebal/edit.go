package main

import (
	"fmt"

	"github.com/lukingo187/re.balance/internal/config"
	"github.com/lukingo187/re.balance/internal/rebal"
	"github.com/spf13/cobra"
)

func newEditCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "edit <ISIN> [--name N] [--min M | --clear-min] [--target T [--scale | --from ISIN[=pp]...]]",
		Short: "Change an existing position's name, floor or target, and write config",
		Long: "Edit an existing position. --name and --min change its display name and\n" +
			"monthly floor (--clear-min drops the override, back to default_min_monthly).\n" +
			"--target changes its target percentage, absorbing the difference into the\n" +
			"others by rescaling them (--scale) or by explicit amounts (--from ISIN=pp,\n" +
			"which must sum to the size of the change).\n\n" +
			"Writes config.toml by default; use --dry-run to preview without writing.",
		Args: cobra.ExactArgs(1),
		RunE: runEdit,
	}
	cmd.Flags().String("name", "", "new display name")
	cmd.Flags().Int("min", -1, "new monthly floor for this position")
	cmd.Flags().Bool("clear-min", false, "remove the per-position floor (use the default)")
	cmd.Flags().Float64("target", 0, "new target percentage")
	cmd.Flags().Bool("scale", false, "with --target: rescale the other targets to absorb the change")
	cmd.Flags().StringArray("from", nil, "with --target: move target against this position: ISIN=pp (repeatable) or a bare ISIN")
	cmd.Flags().Bool("dry-run", false, "print the result but do not write config")
	return cmd
}

func runEdit(cmd *cobra.Command, args []string) error {
	isin := normKey(args[0])
	dry, _ := cmd.Flags().GetBool("dry-run")
	clearMin, _ := cmd.Flags().GetBool("clear-min")
	scale, _ := cmd.Flags().GetBool("scale")
	fromEntries, _ := cmd.Flags().GetStringArray("from")
	changeTarget := cmd.Flags().Changed("target")

	var namePtr *string
	if cmd.Flags().Changed("name") {
		n, _ := cmd.Flags().GetString("name")
		namePtr = &n
	}
	var minPtr *int
	if cmd.Flags().Changed("min") {
		m, _ := cmd.Flags().GetInt("min")
		minPtr = &m
	}

	if clearMin && minPtr != nil {
		return fmt.Errorf("use either --min or --clear-min, not both")
	}
	if !changeTarget && namePtr == nil && minPtr == nil && !clearMin {
		return fmt.Errorf("nothing to change — pass --name, --min, --clear-min or --target")
	}
	if (scale || len(fromEntries) > 0) && !changeTarget {
		return fmt.Errorf("--scale and --from only apply together with --target")
	}

	cfg, err := loadConfig(cmd)
	if err != nil {
		return err
	}
	st, _, err := loadState(cmd)
	if err != nil {
		return err
	}

	after := cfg

	// Target change first (it rebalances the whole active set), then the plain
	// attribute edits on top.
	if changeTarget {
		target, _ := cmd.Flags().GetFloat64("target")
		var delta float64
		if idx := indexInConfig(cfg, isin); idx >= 0 {
			if delta = target - cfg.Positions[idx].Target; delta < 0 {
				delta = -delta
			}
		}
		sources, err := parseSources(fromEntries, delta)
		if err != nil {
			return err
		}
		r, err := redistribution(scale, sources)
		if err != nil {
			return err
		}
		if after, err = config.Retarget(after, isin, target, r); err != nil {
			return err
		}
	}
	if namePtr != nil || minPtr != nil || clearMin {
		if after, err = config.EditPosition(after, isin, namePtr, minPtr, clearMin); err != nil {
			return err
		}
	}

	if issues := rebal.Validate(after.ToInput(st.Values)); len(issues) > 0 {
		return refuseInvalid(cmd, issues)
	}

	// A target change reshapes every target, so show the diff table. A name/floor
	// edit alone prints a one-line summary instead.
	if changeTarget {
		return applyConfig(cmd, cfg, after, dry, nil)
	}
	idx := indexInConfig(after, isin)
	p := after.Positions[idx]
	minLabel := "default"
	if p.MinMonthly != nil {
		minLabel = fmt.Sprintf("%d €", *p.MinMonthly)
	}
	fmt.Fprintf(cmd.OutOrStdout(), "\n%s%s (%s) — floor %s\n\n", margin, p.Name, p.ISIN, minLabel)
	return saveOrDry(cmd, after, dry)
}
