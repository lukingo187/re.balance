package main

import (
	"fmt"
	"math"

	"github.com/lukingo187/re.balance/internal/money"
)

// formatMoney renders an amount the way rebal itself does (EU-style, comma
// decimal, space-grouped thousands) when it's EUR — which is all this
// account deals in — and falls back to a plain "amount CCY" otherwise.
func formatMoney(amount float64, currency string) string {
	if currency == "EUR" {
		return money.Euro(int64(math.Round(amount * 100)))
	}
	return fmt.Sprintf("%.2f %s", amount, currency)
}
