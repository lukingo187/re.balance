package main

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

func newListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "Show Scalable holdings, savings plans and watchlist in one view",
		Long: "Reads straight from `sc` (holdings, savings plans, watchlist) and prints all\n" +
			"three together — read-only, nothing is written. The watchlist section is\n" +
			"copy-paste ready ISINs for `rebal add --isin ...`.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			out := cmd.OutOrStdout()

			cfg, _, _, _ := loadConfigIfExists(cmd) // best-effort, only used to mark tracked ISINs
			tracked := map[string]bool{}
			for _, p := range cfg.Positions {
				tracked[strings.ToUpper(p.ISIN)] = true
			}

			holdings, err := fetchHoldings()
			if err != nil {
				return err
			}
			plans, _, err := fetchSavingsPlans()
			if err != nil {
				return err
			}
			watch, err := fetchWatchlist()
			if err != nil {
				return err
			}

			fmt.Fprintf(out, "\n%sHoldings\n\n", margin)
			rows := [][]Cell{header("Position", "ISIN", "Value", "In rebal?")}
			for _, h := range holdings {
				inRebal := tinted("—", dim)
				if tracked[strings.ToUpper(h.ISIN)] {
					inRebal = plain("yes")
				}
				rows = append(rows, []Cell{
					plain(h.Name), plain(h.ISIN), plain(formatMoney(h.Valuation, h.ValuationCurrency)), inRebal,
				})
			}
			table(out, rows, []bool{false, false, true, false})

			fmt.Fprintf(out, "\n%sSavings plans\n\n", margin)
			rows = [][]Cell{header("Position", "ISIN", "Amount", "Frequency", "Day", "Next execution")}
			for _, p := range plans {
				freq := plain(p.Frequency)
				if p.Frequency != "MONTHLY" {
					freq = tinted(p.Frequency, dim)
				}
				rows = append(rows, []Cell{
					plain(p.Name), plain(p.ISIN), plain(formatMoney(p.Amount, "EUR")), freq,
					plain(fmt.Sprintf("%d", p.DayOfMonth)), plain(p.NextExecutionDate),
				})
			}
			table(out, rows, []bool{false, false, true, false, true, false})

			fmt.Fprintf(out, "\n%sWatchlist\n\n", margin)
			rows = [][]Cell{header("Position", "ISIN", "Price")}
			for _, item := range watch {
				rows = append(rows, []Cell{
					plain(item.Name), plain(item.ISIN), plain(formatMoney(item.QuoteMidPrice, item.QuoteCurrency)),
				})
			}
			table(out, rows, []bool{false, false, true})
			fmt.Fprintln(out)

			return nil
		},
	}
}
