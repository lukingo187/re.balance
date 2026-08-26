package main

import (
	"fmt"
	"io"
	"strings"
	"text/tabwriter"

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

			fmt.Fprintln(out, "Holdings")
			w := newTable(out)
			fmt.Fprintln(w, "ISIN\tName\tValue\tIn rebal?")
			for _, h := range holdings {
				inRebal := ""
				if tracked[strings.ToUpper(h.ISIN)] {
					inRebal = "yes"
				}
				fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", h.ISIN, h.Name, formatMoney(h.Valuation, h.ValuationCurrency), inRebal)
			}
			w.Flush()

			fmt.Fprintln(out, "\nSavings plans")
			w = newTable(out)
			fmt.Fprintln(w, "ISIN\tName\tAmount\tFrequency\tDay\tNext execution")
			for _, p := range plans {
				fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%d\t%s\n",
					p.ISIN, p.Name, formatMoney(p.Amount, "EUR"), p.Frequency, p.DayOfMonth, p.NextExecutionDate)
			}
			w.Flush()

			fmt.Fprintln(out, "\nWatchlist")
			w = newTable(out)
			fmt.Fprintln(w, "ISIN\tName\tPrice")
			for _, item := range watch {
				fmt.Fprintf(w, "%s\t%s\t%s\n", item.ISIN, item.Name, formatMoney(item.QuoteMidPrice, item.QuoteCurrency))
			}
			w.Flush()

			return nil
		},
	}
}

func newTable(out io.Writer) *tabwriter.Writer {
	return tabwriter.NewWriter(out, 0, 2, 2, ' ', 0)
}
