package main

import (
	"fmt"
	"io"
	"math"
	"strconv"
	"strings"

	"github.com/lukingo187/re.balance/internal/config"
	"github.com/lukingo187/re.balance/internal/rebal"
	"github.com/spf13/cobra"
)

// margin is the common left indent every command's output uses, so plan, sweep,
// add, rm and the rest share one visual frame.
const margin = "  "

// formatPP renders a measured drift, always two decimals: "±0.01pp".
func formatPP(pp float64) string {
	if math.IsNaN(pp) {
		return "n/a"
	}
	return fmt.Sprintf("±%.2fpp", pp)
}

// formatTol renders a configured tolerance the way the user wrote it: "±0.5pp".
func formatTol(pp float64) string {
	return "±" + trimPct(pp) + "pp"
}

// trimPct renders a one-decimal percentage without a trailing ".0" (10.0 -> "10").
func trimPct(f float64) string {
	return strings.TrimSuffix(fmt.Sprintf("%.1f", f), ".0")
}

// printTargetSet renders the before -> after target table shared by every
// config-mutating command: one row per active position (unchanged rows marked
// "·"), plus a labelled row for any position that was added, removed, frozen or
// unfrozen.
func printTargetSet(w io.Writer, before, after config.Config) {
	beforeActive := map[string]float64{}
	beforeFrozen := map[string]bool{}
	for _, p := range before.Positions {
		if p.WindDown {
			beforeFrozen[normKey(p.ISIN)] = true
		} else {
			beforeActive[normKey(p.ISIN)] = p.Target
		}
	}
	afterFrozen := map[string]bool{}
	for _, p := range after.Positions {
		if p.WindDown {
			afterFrozen[normKey(p.ISIN)] = true
		}
	}

	rows := [][]Cell{{
		tinted("Position", bold), tinted("ISIN", bold), tinted("Target", bold), tinted("Δ", bold),
	}}

	afterActive := map[string]bool{}
	for _, p := range after.Positions {
		if p.WindDown {
			continue
		}
		key := normKey(p.ISIN)
		afterActive[key] = true
		switch old, wasActive := beforeActive[key]; {
		case wasActive:
			rows = append(rows, targetRow(p.Name, p.ISIN, old, p.Target, ""))
		case beforeFrozen[key]:
			rows = append(rows, targetRow(p.Name, p.ISIN, 0, p.Target, "unfrozen"))
		default:
			rows = append(rows, targetRow(p.Name, p.ISIN, 0, p.Target, "new"))
		}
	}
	// Positions active before but no longer in the active set: removed, or frozen
	// (still in the file, but out of the pool).
	for _, p := range before.Positions {
		key := normKey(p.ISIN)
		if p.WindDown || afterActive[key] {
			continue
		}
		label := "removed"
		if afterFrozen[key] {
			label = "frozen"
		}
		rows = append(rows, targetRow(p.Name, p.ISIN, p.Target, 0, label))
	}
	table(w, rows, []bool{false, false, false, true})
}

// targetRow builds one before→after row for the target diff table. The Δ is
// coloured git-style: green for an increase, red for a decrease, dim for none.
func targetRow(name, isin string, before, after float64, label string) []Cell {
	if label != "" {
		name += " (" + label + ")"
	}
	var target, delta string
	var dc func(string) string
	switch {
	case label == "removed" || label == "frozen":
		target = trimPct(before) + " → —"
		delta, dc = fmt.Sprintf("%+.1f", -before), red
	case before == after:
		target, delta, dc = trimPct(after), "·", dim
	default:
		target = trimPct(before) + " → " + trimPct(after)
		delta = fmt.Sprintf("%+.1f", after-before)
		if after > before {
			dc = green
		} else {
			dc = red
		}
	}
	return []Cell{plain(name), plain(isin), plain(target), tinted(delta, dc)}
}

func normKey(isin string) string { return strings.ToUpper(strings.TrimSpace(isin)) }

// parsePP reads a positive percentage-point amount, accepting a comma or dot
// decimal ("5", "6.67", "3,33").
func parsePP(s string) (float64, error) {
	f, err := strconv.ParseFloat(strings.ReplaceAll(strings.TrimSpace(s), ",", "."), 64)
	if err != nil || f <= 0 {
		return 0, fmt.Errorf("not a positive number: %q", s)
	}
	return f, nil
}

// applyConfig is the shared tail of every writing command: it prints the
// before -> after diff, runs any extra summary, then either writes the new config
// or, under --dry-run, reports that nothing was written.
func applyConfig(cmd *cobra.Command, before, after config.Config, dry bool, extra func(io.Writer)) error {
	out := cmd.OutOrStdout()

	fmt.Fprintln(out)
	printTargetSet(out, before, after)
	if extra != nil {
		fmt.Fprintln(out)
		extra(out)
	}
	fmt.Fprintln(out)

	if dry {
		fmt.Fprintf(out, "%sdry run — config not written.\n", margin)
		return nil
	}
	path, err := configPath(cmd)
	if err != nil {
		return err
	}
	if err := config.Write(path, after); err != nil {
		return fmt.Errorf("writing config: %w", err)
	}
	fmt.Fprintf(out, "%swrote %s\n", margin, path)
	return nil
}

// saveOrDry writes after to the resolved config path, or under --dry-run reports
// that nothing was written. Used by commands that print their own summary and
// don't need the target diff table.
func saveOrDry(cmd *cobra.Command, after config.Config, dry bool) error {
	out := cmd.OutOrStdout()
	if dry {
		fmt.Fprintf(out, "%sdry run — config not written.\n", margin)
		return nil
	}
	path, err := configPath(cmd)
	if err != nil {
		return err
	}
	if err := config.Write(path, after); err != nil {
		return fmt.Errorf("writing config: %w", err)
	}
	fmt.Fprintf(out, "%swrote %s\n", margin, path)
	return nil
}

// refuseInvalid prints validation issues and returns the sentinel error that
// stops a command before it writes anything.
func refuseInvalid(cmd *cobra.Command, issues []rebal.Issue) error {
	out := cmd.OutOrStdout()
	fmt.Fprintf(out, "\n%srefusing — the result would be invalid:\n", margin)
	for _, is := range issues {
		fmt.Fprintf(out, "%s  %s: %s\n", margin, red(is.Severity.String()), is.Message)
	}
	return fmt.Errorf("aborted — nothing written")
}

// parseSources turns repeated --from/--to entries into explicit Sources. Each
// entry is either "ISIN" (bare: take the whole amount from that one position) or
// "ISIN=pp". The two forms may not be mixed, and a bare entry must be alone.
func parseSources(entries []string, whole float64) ([]config.Source, error) {
	if len(entries) == 0 {
		return nil, nil
	}
	var bare, explicit []string
	for _, e := range entries {
		if strings.Contains(e, "=") {
			explicit = append(explicit, e)
		} else {
			bare = append(bare, e)
		}
	}
	if len(bare) > 0 && len(explicit) > 0 {
		return nil, fmt.Errorf("don't mix bare ISIN and ISIN=pp forms in one command")
	}
	if len(bare) > 0 {
		if len(bare) > 1 {
			return nil, fmt.Errorf("give amounts (ISIN=pp) when taking from more than one position")
		}
		return []config.Source{{ISIN: normKey(bare[0]), PP: whole}}, nil
	}

	var srcs []config.Source
	for _, e := range explicit {
		name, amount, _ := strings.Cut(e, "=")
		pp, err := parsePP(amount)
		if err != nil {
			return nil, fmt.Errorf("bad amount in %q (want ISIN=pp with pp > 0)", e)
		}
		srcs = append(srcs, config.Source{ISIN: normKey(name), PP: pp})
	}
	return srcs, nil
}

// redistribution builds a Redistribution from the --scale flag and parsed
// sources, rejecting the ambiguous case where both are given.
func redistribution(scale bool, sources []config.Source) (config.Redistribution, error) {
	if scale && len(sources) > 0 {
		return config.Redistribution{}, fmt.Errorf("use either --scale or explicit sources, not both")
	}
	if scale {
		return config.Redistribution{Scale: true}, nil
	}
	return config.Redistribution{Sources: sources}, nil
}
