package main

import (
	"fmt"
	"io"
	"os"
	"strings"
	"unicode/utf8"

	"github.com/spf13/cobra"
)

// colorEnabled gates every colour helper. It defaults to false — off unless a
// command run explicitly turns it on — so tests and piped output stay plain.
var colorEnabled bool

const (
	ansiReset = "\033[0m"
	ansiRed   = "\033[31m"
	ansiGreen = "\033[32m"
	ansiDim   = "\033[2m"
	ansiBold  = "\033[1m"
)

func colorize(s, code string) string {
	if !colorEnabled || s == "" {
		return s
	}
	return code + s + ansiReset
}

func green(s string) string { return colorize(s, ansiGreen) }
func red(s string) string   { return colorize(s, ansiRed) }
func dim(s string) string   { return colorize(s, ansiDim) }
func bold(s string) string  { return colorize(s, ansiBold) }

// setColor decides whether output should be coloured: off if --no-color is set,
// off if NO_COLOR is present (the standard convention), off if stdout is not a
// terminal (piped or redirected), on otherwise.
func setColor(cmd *cobra.Command) {
	if noColor, _ := cmd.Flags().GetBool("no-color"); noColor {
		colorEnabled = false
		return
	}
	if _, ok := os.LookupEnv("NO_COLOR"); ok {
		colorEnabled = false
		return
	}
	fi, err := os.Stdout.Stat()
	colorEnabled = err == nil && fi.Mode()&os.ModeCharDevice != 0
}

// signColor returns green for a value at or below zero (under target) and red
// for a positive drift (over target); near-zero drift is dimmed. The +/- sign
// already carries the meaning, so colour is reinforcement, never the only cue.
func driftColor(pp float64) func(string) string {
	switch {
	case pp > 0.05:
		return red
	case pp < -0.05:
		return green
	default:
		return dim
	}
}

// Cell is one table cell: visible text plus an optional colourizer. Width is
// always measured from the visible text, so colour never breaks alignment.
type Cell struct {
	s     string
	color func(string) string
}

func plain(s string) Cell                         { return Cell{s: s} }
func tinted(s string, c func(string) string) Cell { return Cell{s: s, color: c} }

func (c Cell) pad(width int, right bool) string {
	gap := width - utf8.RuneCountInString(c.s)
	if gap < 0 {
		gap = 0
	}
	body := c.s
	if c.color != nil {
		body = c.color(c.s)
	}
	if right {
		return strings.Repeat(" ", gap) + body
	}
	return body + strings.Repeat(" ", gap)
}

// table renders rows as an aligned grid indented by margin, with a two-space
// gutter between columns. align[j] right-aligns column j. Each line is trimmed of
// trailing padding.
func table(w io.Writer, rows [][]Cell, align []bool) {
	var widths []int
	for _, r := range rows {
		for j, c := range r {
			for len(widths) <= j {
				widths = append(widths, 0)
			}
			if n := utf8.RuneCountInString(c.s); n > widths[j] {
				widths[j] = n
			}
		}
	}
	for _, r := range rows {
		var b strings.Builder
		b.WriteString(margin)
		for j, c := range r {
			if j > 0 {
				b.WriteString("  ")
			}
			right := j < len(align) && align[j]
			b.WriteString(c.pad(widths[j], right))
		}
		fmt.Fprintln(w, strings.TrimRight(b.String(), " "))
	}
}
