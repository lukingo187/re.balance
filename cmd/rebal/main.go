// Command rebal is the re.balance CLI: a savings-plan rebalancer that splits a
// fixed monthly amount across ETF positions so a portfolio converges back to
// its target weights without ever selling.
package main

import (
	"fmt"
	"os"

	"github.com/lukingo187/re.balance/internal/ui"
	"github.com/spf13/cobra"
)

// version is overridden at build time via -ldflags "-X main.version=...".
var version = "dev"

func newRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:     "rebal",
		Short:   "re.balance — savings-plan rebalancer",
		Long:    "re.balance splits a fixed monthly amount across your ETF positions so the\nportfolio converges back to its target weights over a chosen horizon — without\never selling.",
		Version: version,
		// Usage is help, not an error report: don't dump it when a command
		// returns a runtime error (bad value, validation failure, etc.).
		SilenceUsage: true,
		// Decide colour once per run, before any command produces output.
		PersistentPreRun: func(cmd *cobra.Command, _ []string) { setColor(cmd) },
		// With no subcommand, launch the TUI (spec §8). It reuses the same config
		// and state the CLI does; a missing config still writes a starter first.
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := loadConfig(cmd)
			if err != nil {
				return err
			}
			st, statePath, err := loadState(cmd)
			if err != nil {
				return err
			}
			cfgPath, err := configPath(cmd)
			if err != nil {
				return err
			}
			final, err := ui.Run(cfg, st, cfgPath, statePath)
			if err != nil {
				return err
			}
			// Print the plan the user settled on, so the values are ready to
			// enter at the broker once the TUI's alt-screen is gone.
			if in, plan, ok := final.Result(); ok {
				printPlan(cmd.OutOrStdout(), in, plan)
			}
			return nil
		},
	}

	root.PersistentFlags().String("config", "", "path to config.toml")
	root.PersistentFlags().String("state", "", "path to state.json")
	root.PersistentFlags().Bool("no-color", false, "disable colour output (also honours NO_COLOR)")

	// Keep Cobra's auto-generated `completion` command working (so `rebal
	// completion zsh` still emits a script) but drop it from the help listing —
	// it's noise in a six-command tool.
	root.CompletionOptions.HiddenDefaultCmd = true

	root.AddCommand(
		newListCmd(), newPlanCmd(), newSetCmd(), newCheckCmd(), newSweepCmd(),
		newAddCmd(), newRmCmd(), newEditCmd(), newSettingsCmd(),
		newFreezeCmd(), newUnfreezeCmd(),
	)

	return root
}

func main() {
	if err := newRootCmd().Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
