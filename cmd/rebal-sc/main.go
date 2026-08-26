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

func newRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:          "rebal-sc",
		Short:        "re.balance addon: bridge to the Scalable Capital broker CLI (sc)",
		SilenceUsage: true,
	}
	root.PersistentFlags().String("config", "", "path to rebal's config.toml")
	root.PersistentFlags().String("state", "", "path to rebal's state.json")

	root.AddCommand(newListCmd(), newSyncCmd())
	return root
}

func main() {
	if err := newRootCmd().Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
