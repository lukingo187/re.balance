package main

import (
	"fmt"
	"strings"

	"github.com/lukingo187/re.balance/internal/config"
	"github.com/lukingo187/re.balance/internal/rebal"
	"github.com/spf13/cobra"
)

func newAddCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "add --name NAME --isin ISIN --target T [--scale | --from NAME] [--min M]",
		Short: "Compute the target set after adding a position (prints, does not write)",
		Long: "Work out new targets after adding a position, make room via --scale or\n" +
			"--from, and print the [[positions]] block to paste into config.toml.",
		RunE: runAdd,
	}
	cmd.Flags().String("name", "", "position name (required)")
	cmd.Flags().String("isin", "", "position ISIN (required)")
	cmd.Flags().Float64("target", 0, "target percentage for the new position (required)")
	cmd.Flags().Bool("scale", false, "rescale existing targets proportionally to make room")
	cmd.Flags().String("from", "", "take the whole target from this one position")
	cmd.Flags().Int("min", -1, "monthly floor for the new position (default: config default)")
	return cmd
}

func runAdd(cmd *cobra.Command, _ []string) error {
	name, _ := cmd.Flags().GetString("name")
	isin, _ := cmd.Flags().GetString("isin")
	target, _ := cmd.Flags().GetFloat64("target")
	scale, _ := cmd.Flags().GetBool("scale")
	from, _ := cmd.Flags().GetString("from")
	minFlag, _ := cmd.Flags().GetInt("min")

	name, isin, from = strings.TrimSpace(name), strings.ToUpper(strings.TrimSpace(isin)), strings.TrimSpace(from)
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

	// Existing active positions, in order.
	var activeNames []string
	var activeTargets []float64
	for _, p := range cfg.Positions {
		if !p.WindDown {
			activeNames = append(activeNames, p.Name)
			activeTargets = append(activeTargets, p.Target)
		}
	}

	// Make room, producing the full new active target list (existing + new).
	var finalTargets []float64
	switch {
	case len(activeTargets) == 0:
		finalTargets = []float64{target}
	case scale:
		finalTargets = append(config.Scale(activeTargets, 100-target), target)
	case from != "":
		idx := indexByName(activeNames, from)
		if idx < 0 {
			return fmt.Errorf("--from names no active position: %q", from)
		}
		adjusted := append([]float64(nil), activeTargets...)
		adjusted[idx] -= target
		if adjusted[idx] <= 0 {
			return fmt.Errorf("taking %.1f from %q would drive it to %.1f (must stay > 0)",
				target, from, adjusted[idx])
		}
		finalTargets = append(adjusted, target)
	default:
		return fmt.Errorf("adding to an existing portfolio needs --scale or --from NAME")
	}
	finalTargets = config.RoundTargets(finalTargets)
	newTarget := finalTargets[len(finalTargets)-1]

	// Build the trial config (existing targets updated, new position appended)
	// and refuse if it would break validation — chiefly the never-converges rule.
	trial := trialConfig(cfg, finalTargets, name, isin, minFlag)
	if issues := rebal.Validate(trial.ToInput(st.Values)); len(issues) > 0 {
		out := cmd.OutOrStdout()
		fmt.Fprintln(out, "refusing — the new target set is invalid:")
		for _, is := range issues {
			fmt.Fprintf(out, "  %s: %s\n", is.Severity, is.Message)
		}
		return fmt.Errorf("aborted")
	}

	printAddResult(cmd, trial, name, isin, newTarget, minFlag, st.Values)
	return nil
}

// trialConfig returns cfg with active targets replaced by finalTargets[:n-1] and
// a new position (finalTargets[n-1]) appended.
func trialConfig(cfg config.Config, finalTargets []float64, name, isin string, minFlag int) config.Config {
	trial := cfg
	trial.Positions = make([]config.Position, 0, len(cfg.Positions)+1)
	ai := 0
	for _, p := range cfg.Positions {
		if !p.WindDown {
			p.Target = finalTargets[ai]
			ai++
		}
		trial.Positions = append(trial.Positions, p)
	}
	newPos := config.Position{Name: name, ISIN: isin, Target: finalTargets[len(finalTargets)-1]}
	if minFlag >= 0 {
		newPos.MinMonthly = &minFlag
	}
	trial.Positions = append(trial.Positions, newPos)
	return trial
}

func printAddResult(cmd *cobra.Command, trial config.Config, name, isin string, newTarget float64, minFlag int, values map[string]int64) {
	out := cmd.OutOrStdout()

	fmt.Fprintf(out, "\nAdd this block to your config.toml:\n\n")
	fmt.Fprintln(out, "[[positions]]")
	fmt.Fprintf(out, "name   = %q\n", name)
	fmt.Fprintf(out, "isin   = %q\n", isin)
	fmt.Fprintf(out, "target = %s\n", trimFloat(newTarget))
	if minFlag >= 0 {
		fmt.Fprintf(out, "min_monthly = %d\n", minFlag)
	}

	fmt.Fprintln(out, "\nNew target set:")
	for _, p := range trial.Positions {
		if p.WindDown {
			continue
		}
		fmt.Fprintf(out, "  %-20s %s\n", p.Name, trimFloat(p.Target))
	}

	// Convergence horizon for the fresh position, which starts at 0 €.
	in := trial.ToInput(values)
	if in.Months < 1 {
		in.Months = 1
	}
	tol := trial.Settings.TolerancePP
	max := trial.Settings.MaxHorizon
	if max < 1 {
		max = 36
	}
	if n, ok := convergenceHorizon(in, tol, max); ok {
		fmt.Fprintf(out, "\n%s starts at 0 € and converges to within ±%.1fpp at month %d.\n", name, tol, n)
	} else {
		fmt.Fprintf(out, "\n%s does not converge within %d months at %d €/month.\n", name, max, in.Monthly)
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

func indexByName(names []string, name string) int {
	for i, n := range names {
		if strings.EqualFold(n, name) {
			return i
		}
	}
	return -1
}

// trimFloat renders a one-decimal target without a trailing ".0" (10.0 -> "10").
func trimFloat(f float64) string {
	s := fmt.Sprintf("%.1f", f)
	return strings.TrimSuffix(s, ".0")
}
