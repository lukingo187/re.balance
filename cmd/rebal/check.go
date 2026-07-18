package main

import (
	"fmt"

	"github.com/lukingo187/re.balance/internal/rebal"
	"github.com/spf13/cobra"
)

func newCheckCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "check",
		Short: "Validate config and state; non-zero exit on fatal issues",
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg, err := loadConfig(cmd)
			if err != nil {
				return err
			}
			st, _, err := loadState(cmd)
			if err != nil {
				return err
			}

			issues := rebal.Validate(cfg.ToInput(st.Values))
			out := cmd.OutOrStdout()
			if len(issues) == 0 {
				fmt.Fprintln(out, "✓ config and state are valid")
				return nil
			}
			for _, is := range issues {
				fmt.Fprintf(out, "  %s: %s\n", is.Severity, is.Message)
			}
			return fmt.Errorf("%d issue(s) found", len(issues))
		},
	}
}
