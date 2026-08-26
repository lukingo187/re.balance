package main

import (
	"encoding/json"
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/lukingo187/re.balance/internal/rebal"
	"github.com/spf13/cobra"
)

func newPushCmd() *cobra.Command {
	var all bool
	var amount int
	cmd := &cobra.Command{
		Use:   "push [ISIN...]",
		Short: "Preview rebal's plan as Scalable savings-plan changes (never submits)",
		Long: "Computes rebal's contribution plan (like `rebal plan`) and, for each\n" +
			"active position whose computed monthly amount differs from Scalable (or\n" +
			"has no Scalable plan at all yet), runs `sc broker savings-plans add`\n" +
			"phase 1 — a preview with no side effects, per sc's own docs — and prints\n" +
			"the full ex-ante cost disclosure sc requires before any confirmation.\n" +
			"Separately, any position that's frozen (wind-down) in rebal but still has\n" +
			"an active Scalable plan is listed for removal — sc has no preview step\n" +
			"for removing a plan, so that section just shows the current plan and the\n" +
			"exact commands to remove it later.\n\n" +
			"This command never submits or removes anything itself. Review the\n" +
			"disclosure, then either run the printed sc command yourself, or come back\n" +
			"later and run `rebal-sc push confirm <ISIN> --id <CONFIRMATION_ID>` (or\n" +
			"`--remove` for a frozen position's plan) as a separate, deliberate step —\n" +
			"never chained automatically from this preview.\n\n" +
			"Skipped, always with a note: a non-monthly Scalable plan, and a computed\n" +
			"amount of 0 or less on a position that isn't frozen (an edge case, not\n" +
			"the normal wind-down path).\n\n" +
			"--amount overrides rebal's computed plan entirely for one ISIN (e.g. to\n" +
			"force a plan back to a specific value regardless of what rebal currently\n" +
			"recommends). Requires exactly one ISIN and cannot be combined with --all;\n" +
			"the same amount must then be repeated on `push confirm ... --amount`.",
		RunE: func(cmd *cobra.Command, args []string) error {
			if cmd.Flags().Changed("amount") {
				if all || len(args) != 1 {
					return fmt.Errorf("--amount needs exactly one ISIN and cannot be combined with --all")
				}
				if amount <= 0 {
					return fmt.Errorf("--amount must be a positive number of euros")
				}
				isin := strings.ToUpper(strings.TrimSpace(args[0]))
				in, _, err := computePlan(cmd)
				if err != nil {
					return err
				}
				if err := checkOverridable(in, isin); err != nil {
					return err
				}
				out := cmd.OutOrStdout()
				fmt.Fprintf(out, "\n%s%s\n", margin, dim(fmt.Sprintf(
					"forcing %s to %d €/month — overriding rebal's own computed plan", isin, amount)))
				pres, err := previewSavingsPlanAdd(isin, amount)
				if err != nil {
					return err
				}
				printDisclosure(out, pres)
				return nil
			}
			if !all && len(args) == 0 {
				return fmt.Errorf("give one or more ISINs to preview, or --all")
			}
			if all && len(args) > 0 {
				return fmt.Errorf("use either explicit ISINs or --all, not both")
			}
			want := map[string]bool{}
			for _, a := range args {
				want[strings.ToUpper(strings.TrimSpace(a))] = true
			}

			in, plan, err := computePlan(cmd)
			if err != nil {
				return err
			}
			plans, _, err := fetchSavingsPlans()
			if err != nil {
				return err
			}
			byISIN := map[string]savingsPlan{}
			for _, p := range plans {
				byISIN[strings.ToUpper(p.ISIN)] = p
			}

			out := cmd.OutOrStdout()
			var candidates, removeCandidates []int
			for i, pos := range in.Positions {
				isin := strings.ToUpper(pos.ISIN)
				if !all && !want[isin] {
					continue
				}
				sp, ok := byISIN[isin]
				if pos.WindDown {
					if ok {
						removeCandidates = append(removeCandidates, i)
					}
					continue
				}
				target := plan.Monthly[i]
				switch {
				case target <= 0:
					fmt.Fprintf(out, "%s%s\n", margin, dim(fmt.Sprintf(
						"note: %s's computed plan is %d € — not frozen, but nothing to push (unusual — check `rebal check`)", pos.ISIN, target)))
				case ok && sp.Frequency != "MONTHLY":
					fmt.Fprintf(out, "%s%s\n", margin, dim(fmt.Sprintf(
						"note: %s's Scalable plan is %s, not monthly — skipped", pos.ISIN, sp.Frequency)))
				case ok && int(math.Round(sp.Amount)) == target:
					// already in sync
				default:
					candidates = append(candidates, i)
				}
			}

			if len(candidates) == 0 && len(removeCandidates) == 0 {
				fmt.Fprintf(out, "\n%snothing to push — Scalable already matches rebal's plan\n\n", margin)
				return nil
			}

			if len(candidates) > 0 {
				rows := [][]Cell{header("ISIN", "Name", "Scalable now", "rebal wants")}
				for _, i := range candidates {
					pos := in.Positions[i]
					sp, ok := byISIN[strings.ToUpper(pos.ISIN)]
					now := tinted("— (new plan)", dim)
					dc := green
					if ok {
						now = plain(formatMoney(sp.Amount, "EUR"))
						if plan.Monthly[i] < int(math.Round(sp.Amount)) {
							dc = red
						}
					}
					rows = append(rows, []Cell{
						plain(pos.ISIN), plain(pos.Name), now,
						tinted(fmt.Sprintf("%d €", plan.Monthly[i]), dc),
					})
				}
				fmt.Fprintf(out, "\n%sProposed changes\n\n", margin)
				table(out, rows, []bool{false, false, true, true})

				for _, i := range candidates {
					pos := in.Positions[i]
					pres, err := previewSavingsPlanAdd(pos.ISIN, plan.Monthly[i])
					if err != nil {
						fmt.Fprintf(out, "\n%s%s\n", margin, red(fmt.Sprintf("%s: %v", pos.ISIN, err)))
						continue
					}
					printDisclosure(out, pres)
				}
			}

			if len(removeCandidates) > 0 {
				fmt.Fprintf(out, "\n%sFrozen in rebal, still has an active Scalable plan\n\n", margin)
				rows := [][]Cell{header("ISIN", "Name", "Current plan")}
				for _, i := range removeCandidates {
					pos := in.Positions[i]
					sp := byISIN[strings.ToUpper(pos.ISIN)]
					rows = append(rows, []Cell{
						plain(pos.ISIN), plain(pos.Name),
						plain(fmt.Sprintf("%s (%s)", formatMoney(sp.Amount, "EUR"), sp.Frequency)),
					})
				}
				table(out, rows, []bool{false, false, true})
				fmt.Fprintf(out, "\n%ssc has no preview step for removing a plan. Review the amount above,\n", margin)
				fmt.Fprintf(out, "%sthen either run yourself:\n\n", margin)
				for _, i := range removeCandidates {
					fmt.Fprintf(out, "%s  sc broker savings-plans remove --isin %s --json\n", margin, in.Positions[i].ISIN)
				}
				fmt.Fprintf(out, "\n%sor, as a separate deliberate step:\n\n", margin)
				for _, i := range removeCandidates {
					fmt.Fprintf(out, "%s  rebal-sc push confirm %s --remove\n", margin, in.Positions[i].ISIN)
				}
				fmt.Fprintln(out)
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&all, "all", false, "preview every position whose plan differs from Scalable")
	cmd.Flags().IntVar(&amount, "amount", 0, "force one ISIN to this amount instead of rebal's computed plan")
	cmd.AddCommand(newPushConfirmCmd())
	return cmd
}

func newPushConfirmCmd() *cobra.Command {
	var id string
	var remove bool
	var amountOverride int
	cmd := &cobra.Command{
		Use:   "confirm <ISIN>",
		Short: "Submit or remove a previously previewed savings-plan change",
		Long: "With --id: runs `sc broker savings-plans add --confirm` for one ISIN,\n" +
			"using rebal's currently computed monthly amount — or, with --amount, a\n" +
			"value you choose instead — and the confirmation ID printed by a prior\n" +
			"`rebal-sc push` (or `push --amount`) preview. The amount given here must\n" +
			"match the one used for that preview; sc rejects a mismatch.\n\n" +
			"With --remove: runs `sc broker savings-plans remove` for a position\n" +
			"that's frozen (wind-down) in rebal but still has an active Scalable\n" +
			"plan — refuses if the position isn't actually frozen in rebal, or has\n" +
			"no active plan to remove. sc has no phase-1/phase-2 split for removal,\n" +
			"so --remove itself is the only confirmation step; that's exactly why\n" +
			"it's its own separate, deliberately-named flag rather than the default.\n\n" +
			"Either way, this is the only rebal-sc command that actually changes\n" +
			"anything on Scalable — run it only after reviewing `rebal-sc push`'s\n" +
			"output yourself, as a separate, deliberate step; rebal-sc never runs\n" +
			"this on its own.\n\n" +
			"For --id: if the plan has changed since the preview (e.g. after another\n" +
			"`rebal-sc sync`) or the confirmation has expired, sc rejects it rather\n" +
			"than submit something you didn't see.",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if remove && id != "" {
				return fmt.Errorf("use either --remove or --id, not both")
			}
			if !remove && id == "" {
				return fmt.Errorf("--id is required (copy it from the preview's disclosure), or pass --remove for a frozen position's plan")
			}
			if remove && cmd.Flags().Changed("amount") {
				return fmt.Errorf("--amount has no effect with --remove")
			}
			isin := strings.ToUpper(strings.TrimSpace(args[0]))

			in, plan, err := computePlan(cmd)
			if err != nil {
				return err
			}
			idx := -1
			for i, pos := range in.Positions {
				if strings.ToUpper(pos.ISIN) == isin {
					idx = i
					break
				}
			}
			if idx < 0 {
				return fmt.Errorf("no position matches ISIN %s", isin)
			}

			out := cmd.OutOrStdout()
			if remove {
				plans, _, err := fetchSavingsPlans()
				if err != nil {
					return err
				}
				hasPlan, err := checkRemovable(in.Positions[idx], isin, plans)
				if err != nil {
					return err
				}
				if !hasPlan {
					fmt.Fprintf(out, "\n%s%s: no active Scalable plan to remove\n\n", margin, isin)
					return nil
				}
				if _, err := scRun("broker", "savings-plans", "remove", "--isin", isin); err != nil {
					return err
				}
				fmt.Fprintf(out, "\n%s%s: removed\n\n", margin, isin)
				return nil
			}

			if err := checkOverridable(in, isin); err != nil {
				return err
			}
			amount := plan.Monthly[idx]
			if cmd.Flags().Changed("amount") {
				amount = amountOverride
			}
			if amount <= 0 {
				return fmt.Errorf("%s's amount is %d € — nothing to confirm", isin, amount)
			}

			data, err := scRun("broker", "savings-plans", "add", "--isin", isin, "--amount", strconv.Itoa(amount), "--confirm", id)
			if err != nil {
				return err
			}
			var r struct {
				Result struct {
					Action string `json:"action"`
				} `json:"result"`
			}
			_ = json.Unmarshal(data, &r)
			fmt.Fprintf(out, "\n%s%s: %s (%d €/month)\n\n", margin, isin, orDefault(r.Result.Action, "submitted"), amount)
			return nil
		},
	}
	cmd.Flags().StringVar(&id, "id", "", "confirmation ID printed by rebal-sc push")
	cmd.Flags().BoolVar(&remove, "remove", false, "remove a frozen position's active Scalable plan instead of updating an amount")
	cmd.Flags().IntVar(&amountOverride, "amount", 0, "use this amount instead of rebal's computed plan (must match the preview)")
	return cmd
}

// checkRemovable is the safety gate for `push confirm --remove`: it never
// touches sc itself, just decides whether removal is allowed. Refuses
// outright if the position isn't wind-down in rebal (removal is exclusively
// the frozen-position path); reports "no plan" (not an error — nothing to
// do) if isin has no active Scalable plan.
func checkRemovable(pos rebal.Position, isin string, plans []savingsPlan) (hasPlan bool, err error) {
	if !pos.WindDown {
		return false, fmt.Errorf("%s is not frozen in rebal — refusing to remove its Scalable plan (use --id to update the amount instead)", isin)
	}
	for _, p := range plans {
		if strings.EqualFold(p.ISIN, isin) {
			return true, nil
		}
	}
	return false, nil
}

// checkOverridable is the safety gate for --amount: it only allows forcing
// an amount for an ISIN rebal actually tracks and isn't frozen. Frozen goes
// through --remove instead; an untracked ISIN never gets touched by any
// rebal-sc command, override or not.
func checkOverridable(in rebal.Input, isin string) error {
	for _, pos := range in.Positions {
		if strings.EqualFold(pos.ISIN, isin) {
			if pos.WindDown {
				return fmt.Errorf("%s is frozen in rebal — use --remove instead to remove its Scalable plan", isin)
			}
			return nil
		}
	}
	return fmt.Errorf("no position matches ISIN %s — rebal-sc only touches ISINs already in config.toml", isin)
}

// computePlan loads config and state, validates, and solves — the same
// sequence `rebal plan` itself uses.
func computePlan(cmd *cobra.Command) (rebal.Input, rebal.Plan, error) {
	cfg, err := loadConfig(cmd)
	if err != nil {
		return rebal.Input{}, rebal.Plan{}, err
	}
	st, _, err := loadState(cmd)
	if err != nil {
		return rebal.Input{}, rebal.Plan{}, err
	}
	in := cfg.ToInput(st.Values)
	if issues := rebal.Validate(in); len(issues) > 0 {
		msg := "config has fatal issues — fix with `rebal check` first:\n"
		for _, is := range issues {
			msg += fmt.Sprintf("  %s: %s\n", is.Severity, is.Message)
		}
		return rebal.Input{}, rebal.Plan{}, fmt.Errorf("%s", msg)
	}
	plan, err := rebal.Solve(in)
	if err != nil {
		return rebal.Input{}, rebal.Plan{}, err
	}
	return in, plan, nil
}

func orDefault(s, fallback string) string {
	if s == "" {
		return fallback
	}
	return s
}
