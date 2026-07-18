package main

import (
	"fmt"
	"strconv"
	"strings"
)

// parseMoney reads a human-typed amount into int64 cents. It accepts the forms
// in spec §6 — "12450,00", "12450.00", "12 450 €" — plus grouped variants.
//
// Disambiguation of '.' vs ',':
//   - both present: the rightmost is the decimal separator, the other groups
//     thousands ("12.450,00" and "12,450.00" both -> 1245000).
//   - one present: a separator followed by exactly 3 digits (or repeated) is
//     thousands grouping ("1.234" -> 123400); 1-2 digits is a decimal
//     ("1,50" -> 150, "1.5" -> 150).
func parseMoney(s string) (int64, error) {
	// Keep only digits and separators; drop spaces, €, letters.
	kept := make([]rune, 0, len(s))
	for _, r := range s {
		if (r >= '0' && r <= '9') || r == ',' || r == '.' {
			kept = append(kept, r)
		}
	}
	t := string(kept)
	if strings.Trim(t, ",.") == "" {
		return 0, fmt.Errorf("no numeric value in %q", s)
	}

	lastComma := strings.LastIndex(t, ",")
	lastDot := strings.LastIndex(t, ".")

	decPos := -1 // index of the decimal separator in t; -1 means none
	switch {
	case lastComma >= 0 && lastDot >= 0:
		decPos = max(lastComma, lastDot)
	case lastComma >= 0:
		decPos = lastComma
	case lastDot >= 0:
		decPos = lastDot
	}

	// With a single kind of separator, decide decimal vs. grouping.
	if decPos >= 0 && (lastComma < 0 || lastDot < 0) {
		sep := string(t[decPos])
		after := len(t) - decPos - 1
		switch {
		case strings.Count(t, sep) > 1 || after == 3:
			decPos = -1 // grouping, not a decimal point
		case after > 3:
			return 0, fmt.Errorf("cannot interpret %q", s)
		}
	}

	intPart, fracPart := t, ""
	if decPos >= 0 {
		intPart, fracPart = t[:decPos], t[decPos+1:]
	}
	intPart = strings.NewReplacer(",", "", ".", "").Replace(intPart) // strip grouping
	if intPart == "" {
		intPart = "0"
	}
	if len(fracPart) > 2 {
		return 0, fmt.Errorf("too many decimal places in %q", s)
	}

	euros, err := strconv.ParseInt(intPart, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("bad amount %q", s)
	}
	cents := int64(0)
	if fracPart != "" {
		f, err := strconv.ParseInt((fracPart + "00")[:2], 10, 64)
		if err != nil {
			return 0, fmt.Errorf("bad amount %q", s)
		}
		cents = f
	}
	return euros*100 + cents, nil
}

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
