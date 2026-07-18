package main

import (
	"fmt"
	"strings"
)

// formatEuro renders int64 cents as EU-style money: 1245000 -> "12 450,00 €".
// Thousands are space-grouped and the decimal separator is a comma, matching
// the display in rebalance-spec.md §8.
func formatEuro(cents int64) string {
	neg := cents < 0
	if neg {
		cents = -cents
	}
	whole := groupThousands(cents / 100)
	out := fmt.Sprintf("%s,%02d €", whole, cents%100)
	if neg {
		out = "-" + out
	}
	return out
}

// groupThousands inserts a space every three digits from the right.
func groupThousands(n int64) string {
	s := fmt.Sprintf("%d", n)
	if len(s) <= 3 {
		return s
	}
	var b strings.Builder
	lead := len(s) % 3
	if lead > 0 {
		b.WriteString(s[:lead])
	}
	for i := lead; i < len(s); i += 3 {
		if b.Len() > 0 {
			b.WriteByte(' ')
		}
		b.WriteString(s[i : i+3])
	}
	return b.String()
}
