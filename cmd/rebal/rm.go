package main

import (
	"fmt"
	"strings"

	"github.com/lukingo187/re.balance/internal/config"
	"github.com/lukingo187/re.balance/internal/rebal"
	"github.com/spf13/cobra"
)

func newRmCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "rm <ISIN> [--scale | --to ISIN]",
		Short: "Compute the target set after removing a position (prints, does not write)",
		Long: "Work out new targets after removing a position, giving its share back via\n" +
			"--scale or --to, and print the result to paste into config.toml.",
		Args: cobra.ExactArgs(1),
		RunE: runRm,
	}
	cmd.Flags().Bool("scale", false, "rescale remaining targets proportionally to fill 100")
	cmd.Flags().String("to", "", "give the freed target to this position (by ISIN)")
	return cmd
}

func runRm(cmd *cobra.Command, args []string) error {
	key := strings.ToUpper(strings.TrimSpace(args[0]))
	scale, _ := cmd.Flags().GetBool("scale")
	to, _ := cmd.Flags().GetString("to")
	to = strings.ToUpper(strings.TrimSpace(to))

	cfg, err := loadConfig(cmd)
	if err != nil {
		return err
	}
	st, _, err := loadState(cmd)
	if err != nil {
		return err
	}

	// Find the position to remove by ISIN.
	removeIdx := -1
	for i, p := range cfg.Positions {
		if strings.EqualFold(p.ISIN, key) {
			removeIdx = i
			break
		}
	}
	if removeIdx < 0 {
		return fmt.Errorf("no position matches ISIN %q", key)
	}
	removed := cfg.Positions[removeIdx]

	out := cmd.OutOrStdout()

	// Removing a wind-down position changes no targets — it has none.
	if removed.WindDown {
		fmt.Fprintf(out, "\nRemove %q (%s) — it is wind-down, so targets are unchanged.\n",
			removed.Name, removed.ISIN)
		return nil
	}

	// Remaining active positions and their targets, in order.
	var isins []string
	var targets []float64
	for i, p := range cfg.Positions {
		if i == removeIdx || p.WindDown {
			continue
		}
		isins = append(isins, p.ISIN)
		targets = append(targets, p.Target)
	}
	if len(targets) == 0 {
		return fmt.Errorf("cannot remove the last active position")
	}

	switch {
	case scale:
		targets = config.Scale(targets, 100)
	case to != "":
		idx := indexByISIN(isins, to)
		if idx < 0 {
			return fmt.Errorf("--to matches no active position (by ISIN): %q", to)
		}
		targets[idx] += removed.Target
	default:
		return fmt.Errorf("removing a position needs --scale or --to NAME")
	}
	targets = config.RoundTargets(targets)

	// Build the trial config (removed dropped, remaining targets updated) and
	// validate before presenting.
	trial := cfg
	trial.Positions = nil
	ai := 0
	for i, p := range cfg.Positions {
		if i == removeIdx {
			continue
		}
		if !p.WindDown {
			p.Target = targets[ai]
			ai++
		}
		trial.Positions = append(trial.Positions, p)
	}
	if issues := rebal.Validate(trial.ToInput(st.Values)); len(issues) > 0 {
		fmt.Fprintln(out, "refusing — the new target set is invalid:")
		for _, is := range issues {
			fmt.Fprintf(out, "  %s: %s\n", is.Severity, is.Message)
		}
		return fmt.Errorf("aborted")
	}

	fmt.Fprintf(out, "\nRemove %q (%s). New target set:\n", removed.Name, removed.ISIN)
	for _, p := range trial.Positions {
		if p.WindDown {
			continue
		}
		fmt.Fprintf(out, "  %-20s %s\n", p.Name, trimFloat(p.Target))
	}
	return nil
}
