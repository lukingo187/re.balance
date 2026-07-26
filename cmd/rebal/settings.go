package main

import (
	"fmt"
	"text/tabwriter"

	"github.com/lukingo187/re.balance/internal/config"
	"github.com/lukingo187/re.balance/internal/rebal"
	"github.com/spf13/cobra"
)

func newSettingsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "settings [--monthly N] [--tolerance X] [--horizon N] [--max-horizon N] [--default-min N]",
		Short: "Show or change [settings] and write config",
		Long: "With no flags, print the current settings. With one or more flags, change\n" +
			"only those values and write config.toml (use --dry-run to preview).",
		Args: cobra.NoArgs,
		RunE: runSettings,
	}
	cmd.Flags().Int("monthly", 0, "total euros/month across all plans")
	cmd.Flags().Float64("tolerance", 0, "convergence tolerance in percentage points")
	cmd.Flags().Int("horizon", 0, "default horizon in months")
	cmd.Flags().Int("max-horizon", 0, "sweep upper bound in months")
	cmd.Flags().Int("default-min", 0, "default monthly floor per plan")
	cmd.Flags().Bool("dry-run", false, "print the result but do not write config")
	return cmd
}

func runSettings(cmd *cobra.Command, _ []string) error {
	dry, _ := cmd.Flags().GetBool("dry-run")
	cfg, err := loadConfig(cmd)
	if err != nil {
		return err
	}
	st, _, err := loadState(cmd)
	if err != nil {
		return err
	}

	patch, changed := settingsPatch(cmd)
	out := cmd.OutOrStdout()

	// No flags: just show what's there.
	if !changed {
		printSettings(out, cfg.Settings)
		return nil
	}

	after := config.SetSettings(cfg, patch)
	if issues := rebal.Validate(after.ToInput(st.Values)); len(issues) > 0 {
		return refuseInvalid(cmd, issues)
	}

	printSettings(out, after.Settings)
	fmt.Fprintln(out)
	return saveOrDry(cmd, after, dry)
}

// settingsPatch reads only the flags the user actually set, so unspecified
// settings are left untouched.
func settingsPatch(cmd *cobra.Command) (config.SettingsPatch, bool) {
	var p config.SettingsPatch
	changed := false
	if cmd.Flags().Changed("monthly") {
		v, _ := cmd.Flags().GetInt("monthly")
		p.Monthly, changed = &v, true
	}
	if cmd.Flags().Changed("tolerance") {
		v, _ := cmd.Flags().GetFloat64("tolerance")
		p.Tolerance, changed = &v, true
	}
	if cmd.Flags().Changed("horizon") {
		v, _ := cmd.Flags().GetInt("horizon")
		p.Horizon, changed = &v, true
	}
	if cmd.Flags().Changed("max-horizon") {
		v, _ := cmd.Flags().GetInt("max-horizon")
		p.MaxHorizon, changed = &v, true
	}
	if cmd.Flags().Changed("default-min") {
		v, _ := cmd.Flags().GetInt("default-min")
		p.DefaultMin, changed = &v, true
	}
	return p, changed
}

func printSettings(w interface{ Write([]byte) (int, error) }, s config.Settings) {
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	fmt.Fprintf(tw, "\n%smonthly_total\t%d €/month\t\n", margin, s.MonthlyTotal)
	fmt.Fprintf(tw, "%sdefault_min_monthly\t%d €\t\n", margin, s.DefaultMinMonthly)
	fmt.Fprintf(tw, "%shorizon_months\t%d\t\n", margin, s.HorizonMonths)
	fmt.Fprintf(tw, "%stolerance_pp\t%s\t\n", margin, formatTol(s.TolerancePP))
	fmt.Fprintf(tw, "%smax_horizon\t%d\t\n", margin, s.MaxHorizon)
	tw.Flush()
}
