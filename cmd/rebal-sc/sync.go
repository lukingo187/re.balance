package main

import (
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/lukingo187/re.balance/internal/config"
	"github.com/lukingo187/re.balance/internal/money"
	"github.com/lukingo187/re.balance/internal/state"
	"github.com/spf13/cobra"
)

func newSyncCmd() *cobra.Command {
	var all, dryRun, createMissing bool
	cmd := &cobra.Command{
		Use:   "sync [ISIN...]",
		Short: "Pull current Scalable valuations into rebal's state.json",
		Long: "Fetches live valuations from `sc broker holdings` and writes them into\n" +
			"state.json — the same effect as running `rebal set <ISIN> <value>` for each\n" +
			"one. By default it only touches ISINs that already exist in config.toml and\n" +
			"never creates positions.\n\n" +
			"Give explicit ISINs to update just those, or --all for every position that\n" +
			"exists in both rebal and Scalable.\n\n" +
			"--create-missing (used with --all) also adds any Scalable holding that isn't\n" +
			"in config.toml yet, at a target proportional to its current value — handy the\n" +
			"first time you point rebal-sc at an account rebal doesn't know about yet. It\n" +
			"never removes or renames anything; review the printed target diff and adjust\n" +
			"with `rebal edit` afterwards if the starting point isn't what you want.",
		RunE: func(cmd *cobra.Command, args []string) error {
			if createMissing && !all {
				return fmt.Errorf("--create-missing requires --all")
			}
			if !all && len(args) == 0 {
				return fmt.Errorf("give one or more ISINs to sync, or --all")
			}
			if all && len(args) > 0 {
				return fmt.Errorf("use either explicit ISINs or --all, not both")
			}

			cfg, cfgPath, cfgExisted, err := loadConfigIfExists(cmd)
			if err != nil {
				return err
			}
			if !cfgExisted && !createMissing {
				return fmt.Errorf("no config at %s — run `rebal check` to create one, or pass --create-missing --all to build it from Scalable", cfgPath)
			}

			st, statePath, err := loadState(cmd)
			if err != nil {
				return err
			}

			holdings, err := fetchHoldings()
			if err != nil {
				return err
			}

			want := map[string]bool{}
			for _, a := range args {
				want[strings.ToUpper(strings.TrimSpace(a))] = true
			}

			out := cmd.OutOrStdout()
			now := time.Now()

			tracked := map[string]bool{}
			for _, p := range cfg.Positions {
				tracked[strings.ToUpper(p.ISIN)] = true
			}

			// --- value sync: every holding already tracked by rebal ---
			rows := [][]Cell{header("ISIN", "Name", "Old", "New")}
			valueChanges := 0
			for _, h := range holdings {
				isin := strings.ToUpper(h.ISIN)
				if !tracked[isin] {
					continue
				}
				if !all && !want[isin] {
					continue
				}
				newCents := int64(math.Round(h.Valuation * 100))
				oldCents := st.Values[isin]
				if newCents == oldCents {
					continue
				}
				dc := green
				if newCents < oldCents {
					dc = red
				}
				rows = append(rows, []Cell{
					plain(h.ISIN), plain(h.Name), plain(money.Euro(oldCents)), tinted(money.Euro(newCents), dc),
				})
				valueChanges++
				st.SetValue(isin, newCents, now)
			}
			if valueChanges > 0 {
				fmt.Fprintf(out, "\n%sValues\n\n", margin)
				table(out, rows, []bool{false, false, true, true})
			}

			// Explicit ISINs that don't match a Scalable holding at all: say so.
			for isin := range want {
				found := false
				for _, h := range holdings {
					if strings.ToUpper(h.ISIN) == isin {
						found = true
						break
					}
				}
				if !found {
					fmt.Fprintf(out, "%s%s\n", margin, dim(fmt.Sprintf("note: %s not found in Scalable holdings, skipped", isin)))
				}
			}

			// --- create-missing: holdings rebal doesn't know about yet ---
			createdChanges := 0
			if createMissing {
				var missing []holding
				for _, h := range holdings {
					if !tracked[strings.ToUpper(h.ISIN)] {
						missing = append(missing, h)
					}
				}
				if len(missing) > 0 {
					newCfg, err := addMissingPositions(cfg, missing, st)
					if err != nil {
						return fmt.Errorf("computing new positions: %w", err)
					}
					if !cfgExisted {
						newCfg.Settings = starterSettings(newCfg.Settings)
						if _, total, err := fetchSavingsPlans(); err == nil && total > 0 {
							newCfg.Settings.MonthlyTotal = int(math.Round(total))
						}
					}

					fmt.Fprintf(out, "\n%sNew positions %s\n\n", margin, dim("(target ≈ current value share)"))
					newRows := [][]Cell{header("ISIN", "Name", "Target")}
					for _, p := range newCfg.Positions {
						if !tracked[strings.ToUpper(p.ISIN)] {
							newRows = append(newRows, []Cell{plain(p.ISIN), plain(p.Name), plain(trimPct(p.Target) + "%")})
						}
					}
					table(out, newRows, []bool{false, false, true})
					createdChanges = len(missing)

					if !dryRun {
						if err := config.Write(cfgPath, newCfg); err != nil {
							return fmt.Errorf("writing config: %w", err)
						}
						fmt.Fprintf(out, "%swrote %s (%d position(s) added)\n", margin, cfgPath, createdChanges)
					}
					cfg = newCfg
					for _, h := range missing {
						isin := strings.ToUpper(h.ISIN)
						st.SetValue(isin, int64(math.Round(h.Valuation*100)), now)
						valueChanges++
					}
				}
			}

			if valueChanges == 0 && createdChanges == 0 {
				fmt.Fprintf(out, "\n%snothing to update — already in sync\n", margin)
				return nil
			}
			if dryRun {
				fmt.Fprintf(out, "\n%sdry run — nothing written\n", margin)
				return nil
			}
			if err := state.Save(statePath, st); err != nil {
				return fmt.Errorf("saving state: %w", err)
			}
			fmt.Fprintf(out, "%swrote %s\n\n", margin, statePath)
			return nil
		},
	}
	cmd.Flags().BoolVar(&all, "all", false, "sync every position that exists in both rebal and Scalable")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "preview changes without writing config.toml or state.json")
	cmd.Flags().BoolVar(&createMissing, "create-missing", false, "also add Scalable holdings not yet in config.toml (requires --all)")
	return cmd
}

// addMissingPositions appends one position per missing holding, each targeted
// at its share of the full portfolio value (already-tracked positions' synced
// state values, plus the missing holdings themselves) — so day-one drift is
// close to zero. Existing active targets are scaled down as one group to make
// room, preserving their relative proportions to each other.
//
// This computes every new target directly from value shares rather than
// looping config.AddPosition (whose --scale redistributes relative to
// whatever is *currently* active, so each subsequent addition would skew the
// ones added before it — e.g. three holdings with true 60/30/10 value shares
// come out 63/27/10 if added one at a time instead of all at once).
func addMissingPositions(cfg config.Config, missing []holding, st state.State) (config.Config, error) {
	var existingTotal float64
	for _, p := range cfg.Positions {
		if !p.WindDown {
			existingTotal += float64(st.Values[strings.ToUpper(p.ISIN)]) / 100
		}
	}
	var missingTotal float64
	for _, h := range missing {
		missingTotal += h.Valuation
	}
	grandTotal := existingTotal + missingTotal
	if grandTotal <= 0 {
		return config.Config{}, fmt.Errorf("total portfolio value is zero — sync values first")
	}

	out := cfg
	out.Positions = append([]config.Position{}, cfg.Positions...)

	var activeIdx []int
	var activeTargets []float64
	for i, p := range out.Positions {
		if !p.WindDown {
			activeIdx = append(activeIdx, i)
			activeTargets = append(activeTargets, p.Target)
		}
	}
	scaled := config.Scale(activeTargets, 100*existingTotal/grandTotal)
	for k, i := range activeIdx {
		out.Positions[i].Target = scaled[k]
	}

	for _, h := range missing {
		out.Positions = append(out.Positions, config.Position{
			Name:   h.Name,
			ISIN:   strings.ToUpper(strings.TrimSpace(h.ISIN)),
			Target: h.Valuation / grandTotal * 100,
		})
	}

	var idx []int
	var targets []float64
	for i, p := range out.Positions {
		if !p.WindDown {
			idx = append(idx, i)
			targets = append(targets, p.Target)
		}
	}
	rounded := config.RoundTargets(targets)
	for k, i := range idx {
		out.Positions[i].Target = rounded[k]
	}
	return out, nil
}

// starterSettings fills in defaults for a brand-new config. Unlike rebal's own
// starter (internal/config's WriteStarter, default_min_monthly = 25) the floor
// defaults to 0: we have no real per-position broker minimum from Scalable
// here, and a made-up floor can easily exceed a small target's long-run share
// of monthly_total, making the config fail to converge. Leaving monthly_total
// for the caller to override from Scalable's savings-plan total when
// available.
func starterSettings(s config.Settings) config.Settings {
	s.DefaultMinMonthly = 0
	s.HorizonMonths = 12
	s.TolerancePP = 0.5
	s.MaxHorizon = 36
	return s
}

// trimPct renders a one-decimal percentage without a trailing ".0".
func trimPct(f float64) string {
	return strings.TrimSuffix(fmt.Sprintf("%.1f", f), ".0")
}
