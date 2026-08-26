// Command rebal-sc is an optional addon that bridges re.balance and the
// official Scalable Capital broker CLI (`sc`, see
// https://github.com/ScalableCapital/scalable-cli). It shells out to `sc
// --json` and reads/writes rebal's own config.toml and state.json directly —
// rebal itself never depends on this binary or on `sc`.
package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

// version is overridden at build time via -ldflags "-X main.version=...",
// same as cmd/rebal.
var version = "dev"

func newRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:          "rebal-sc",
		Short:        "re.balance addon: bridge to the Scalable Capital broker CLI (sc)",
		Version:      version,
		SilenceUsage: true,
		// Decide colour once per run, before any command produces output.
		PersistentPreRun: func(cmd *cobra.Command, _ []string) { setColor(cmd) },
	}
	root.PersistentFlags().String("config", "", "path to rebal's config.toml")
	root.PersistentFlags().String("state", "", "path to rebal's state.json")
	root.PersistentFlags().Bool("no-color", false, "disable colour output (also honours NO_COLOR)")

	// Keep Cobra's auto-generated `completion` command working but drop it
	// from the help listing — same call as cmd/rebal.
	root.CompletionOptions.HiddenDefaultCmd = true

	root.AddCommand(newListCmd(), newSyncCmd(), newPushCmd())
	return root
}

func main() {
	if err := newRootCmd().Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
