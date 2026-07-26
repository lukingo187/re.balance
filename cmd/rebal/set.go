package main

import (
	"fmt"
	"strings"
	"time"

	"github.com/lukingo187/re.balance/internal/config"
	"github.com/lukingo187/re.balance/internal/money"
	"github.com/lukingo187/re.balance/internal/state"
	"github.com/spf13/cobra"
)

func newSetCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "set <ISIN> <value>",
		Short: "Record a position's current market value",
		Long:  "Write one current market value into state.json so it pre-fills next time.\nAccepts amounts like 12450,00, 12450.00, or \"12 450 €\".",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			isin := strings.ToUpper(strings.TrimSpace(args[0]))
			cents, err := money.Parse(args[1])
			if err != nil {
				return err
			}
			if cents < 0 {
				return fmt.Errorf("value cannot be negative")
			}

			st, path, err := loadState(cmd)
			if err != nil {
				return err
			}
			st.SetValue(isin, cents, time.Now())
			if err := state.Save(path, st); err != nil {
				return fmt.Errorf("saving state: %w", err)
			}

			out := fmt.Sprintf("%sset %s = %s", margin, isin, money.Euro(cents))
			if !isinInConfig(cmd, isin) {
				out += "  (note: no position with this ISIN in config — stored anyway)"
			}
			fmt.Fprintln(cmd.OutOrStdout(), out)
			return nil
		},
	}
}

// isinInConfig reports whether the config lists a position with this ISIN. It is
// best-effort: a missing or unreadable config just means "unknown", and never
// triggers starter generation.
func isinInConfig(cmd *cobra.Command, isin string) bool {
	path, err := configPath(cmd)
	if err != nil {
		return false
	}
	c, err := config.Load(path)
	if err != nil {
		return false
	}
	for _, p := range c.Positions {
		if strings.EqualFold(p.ISIN, isin) {
			return true
		}
	}
	return false
}
