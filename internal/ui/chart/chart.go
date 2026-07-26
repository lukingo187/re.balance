// Package chart renders the horizon chart: max portfolio drift (pp) against the
// number of months in the savings horizon. It is a pure function with no Bubble
// Tea import — `rebal sweep` prints it, and the TUI renders the same string and
// adds a cursor.
//
// One column is one month, so the x-axis is discrete and a click maps to a month
// exactly. The monotone decay means the point where the curve crosses the
// tolerance rule is the convergence horizon.
package chart

import (
	"fmt"
	"math"
	"strings"
)

// Series is the sweep output: X horizons (1..max) with parallel Y max-drift
// values in percentage points.
type Series struct {
	X []int
	Y []float64
}

// Opts controls rendering. Mark, when non-nil, wraps each column's cell so the
// TUI can hit-test it (bubblezone); it is nil in the CLI. Cursor is the X value
// (month) to mark, or 0 for none.
//
// The Style* hooks colour the output. Each defaults to identity when nil, so the
// package stays pure — the CLI injects ANSI, the TUI can inject its own. Colour
// is never the only signal: converged cells already use a distinct glyph.
type Opts struct {
	Height    int
	Tolerance float64
	Cursor    int
	Mark      func(month int, s string) string

	StyleDrift     func(string) string // above-tolerance points, and the "does not converge" line
	StyleConverged func(string) string // at/below-tolerance points, and the "converges" line
	StyleDim       func(string) string // chrome: header and the tolerance rule
}

// identity returns f, or a passthrough when f is nil.
func identity(f func(string) string) func(string) string {
	if f == nil {
		return func(s string) string { return s }
	}
	return f
}

const (
	glyphDrift     = "●" // above tolerance
	glyphConverged = "◆" // at or below tolerance
	glyphRule      = "╌" // the tolerance rule
	gutter         = 5   // width of the y-label column, before the axis border
)

// Render draws the chart and a one-line convergence summary. An empty series
// yields an empty string.
func Render(s Series, o Opts) string {
	n := len(s.X)
	if n == 0 {
		return ""
	}
	height := o.Height
	if height <= 0 {
		height = 10
	}
	mark := o.Mark
	if mark == nil {
		mark = func(_ int, str string) string { return str }
	}
	styleDrift := identity(o.StyleDrift)
	styleConverged := identity(o.StyleConverged)
	styleDim := identity(o.StyleDim)

	// y-range is 0 .. max(Y[0], 2×tolerance); Y[0] is month 1, the largest drift.
	ymax := math.Max(s.Y[0], 2*o.Tolerance)
	if ymax <= 0 {
		ymax = 1
	}
	rowOf := func(y float64) int {
		y = math.Max(0, math.Min(y, ymax))
		return int(math.Round(y / ymax * float64(height-1)))
	}
	tolRow := rowOf(o.Tolerance)
	pointRow := make([]int, n)
	for i, y := range s.Y {
		pointRow[i] = rowOf(y)
	}

	var b strings.Builder
	fmt.Fprintf(&b, "%s %s\n", strings.Repeat(" ", gutter), styleDim("max drift (pp)"))

	for r := height - 1; r >= 0; r-- {
		label := ""
		switch r {
		case height - 1:
			label = trim(ymax)
		case tolRow:
			label = trim(o.Tolerance)
		case 0:
			label = "0"
		}
		fmt.Fprintf(&b, "%*s ", gutter, label)
		if r == tolRow {
			b.WriteString("├")
		} else {
			b.WriteString("│")
		}
		for c := 0; c < n; c++ {
			cell := " "
			switch {
			case pointRow[c] == r && s.Y[c] <= o.Tolerance:
				cell = styleConverged(glyphConverged)
			case pointRow[c] == r:
				cell = styleDrift(glyphDrift)
			case r == tolRow:
				cell = styleDim(glyphRule)
			}
			b.WriteString(mark(s.X[c], cell))
		}
		if r == tolRow {
			fmt.Fprintf(&b, "  %s", styleDim(fmt.Sprintf("tolerance ±%spp", trim(o.Tolerance))))
		}
		b.WriteByte('\n')
	}

	// X-axis with ticks every three months.
	fmt.Fprintf(&b, "%s└", strings.Repeat(" ", gutter))
	for c := 0; c < n; c++ {
		if s.X[c]%3 == 0 {
			b.WriteString("┬")
		} else {
			b.WriteString("─")
		}
	}
	b.WriteByte('\n')

	// Month labels, right-aligned so each ends under its tick.
	b.WriteString(labelRow(s.X, n))

	// Optional cursor marker under the chart.
	if o.Cursor > 0 {
		if ci := indexOf(s.X, o.Cursor); ci >= 0 {
			row := make([]rune, n)
			for i := range row {
				row[i] = ' '
			}
			row[ci] = '▲'
			fmt.Fprintf(&b, "%s %s\n", strings.Repeat(" ", gutter), string(row))
		}
	}

	b.WriteString(summary(s, o, styleConverged, styleDrift))
	return b.String()
}

// labelRow builds the month-label line under the axis.
func labelRow(x []int, n int) string {
	row := make([]rune, n)
	for i := range row {
		row[i] = ' '
	}
	for c := 0; c < n; c++ {
		if x[c]%3 != 0 {
			continue
		}
		lbl := fmt.Sprintf("%d", x[c])
		start := c - len(lbl) + 1 // right-align: last char under the tick
		for k, ch := range lbl {
			if p := start + k; p >= 0 && p < n {
				row[p] = ch
			}
		}
	}
	return fmt.Sprintf("%s %s\n", strings.Repeat(" ", gutter), string(row))
}

// summary is the one-line verdict under the chart: the earliest horizon that
// lands within tolerance — the soonest you can be on target, distinct from the
// horizon you choose to aim for.
func summary(s Series, o Opts, styleConverged, styleDrift func(string) string) string {
	for c := range s.X {
		if s.Y[c] <= o.Tolerance {
			return styleConverged(fmt.Sprintf("earliest on target: month %d (±%spp) ✓", s.X[c], trim(s.Y[c]))) + "\n"
		}
	}
	return styleDrift(fmt.Sprintf("not on target within %d months", s.X[len(s.X)-1])) + "\n"
}

func indexOf(xs []int, v int) int {
	for i, x := range xs {
		if x == v {
			return i
		}
	}
	return -1
}

// trim renders a one-decimal number without a trailing ".0" (8.0 -> "8").
func trim(f float64) string {
	return strings.TrimSuffix(fmt.Sprintf("%.1f", f), ".0")
}
