// Command rebal is the re.balance CLI: a savings-plan rebalancer that splits a
// fixed monthly amount across ETF positions so a portfolio converges back to
// its target weights without ever selling.
package main

import (
	"fmt"
	"os"

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
		// Subcommands land here as the milestones in rebalance-spec.md §10 are
		// completed. With no args this will launch the TUI (milestone 6+); until
		// then it prints help.
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}

	root.PersistentFlags().String("config", "", "path to config.toml")
	root.PersistentFlags().String("state", "", "path to state.json")
	root.PersistentFlags().Bool("no-color", false, "disable colour output")

	// Keep Cobra's auto-generated `completion` command working (so `rebal
	// completion zsh` still emits a script) but drop it from the help listing —
	// it's noise in a six-command tool.
	root.CompletionOptions.HiddenDefaultCmd = true

	root.AddCommand(newPlanCmd(), newSetCmd(), newCheckCmd(), newAddCmd(), newRmCmd(), newSweepCmd())

	return root
}

func main() {
	if err := newRootCmd().Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
