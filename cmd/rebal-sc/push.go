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
	cmd := &cobra.Command{
		Use:   "push [ISIN...]",
		Short: "Preview rebal's plan as Scalable savings-plan changes (never submits)",
		Long: "Computes rebal's contribution plan (like `rebal plan`) and, for each\n" +
			"position whose computed monthly amount differs from its current Scalable\n" +
			"savings plan, runs `sc broker savings-plans add` phase 1 — a preview with\n" +
			"no side effects, per sc's own docs — and prints the full ex-ante cost\n" +
			"disclosure sc requires before any confirmation.\n\n" +
			"This command never submits anything itself. Review the disclosure, then\n" +
			"either run the printed sc command yourself, or come back later and run\n" +
			"`rebal-sc push confirm <ISIN> --id <CONFIRMATION_ID>` as a separate,\n" +
			"deliberate step — never chained automatically from this preview.\n\n" +
			"Only touches monthly plans that already exist on Scalable: it never\n" +
			"creates a new plan, changes a non-monthly one, or reduces one to zero.",
		RunE: func(cmd *cobra.Command, args []string) error {
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
			var candidates []int
			for i, pos := range in.Positions {
				isin := strings.ToUpper(pos.ISIN)
				if !all && !want[isin] {
					continue
				}
				if pos.WindDown {
					if sp, ok := byISIN[isin]; ok {
						fmt.Fprintf(out, "%s%s\n", margin, dim(fmt.Sprintf(
							"note: %s is frozen in rebal but still has an active %s Scalable plan (%s) — not touched",
							pos.ISIN, sp.Frequency, formatMoney(sp.Amount, "EUR"))))
					}
					continue
				}
				target := plan.Monthly[i]
				sp, ok := byISIN[isin]
				switch {
				case !ok:
					fmt.Fprintf(out, "%s%s\n", margin, dim(fmt.Sprintf(
						"note: %s has no existing Scalable savings plan — creating a new one isn't supported yet, add it manually", pos.ISIN)))
				case sp.Frequency != "MONTHLY":
					fmt.Fprintf(out, "%s%s\n", margin, dim(fmt.Sprintf(
						"note: %s's Scalable plan is %s, not monthly — skipped", pos.ISIN, sp.Frequency)))
				case target <= 0:
					fmt.Fprintf(out, "%s%s\n", margin, dim(fmt.Sprintf(
						"note: %s's computed plan is %d € — reducing or removing a plan isn't supported yet", pos.ISIN, target)))
				case int(math.Round(sp.Amount)) == target:
					// already in sync
				default:
					candidates = append(candidates, i)
				}
			}

			if len(candidates) == 0 {
				fmt.Fprintf(out, "\n%snothing to push — Scalable already matches rebal's plan\n\n", margin)
				return nil
			}

			rows := [][]Cell{header("ISIN", "Name", "Scalable now", "rebal wants")}
			for _, i := range candidates {
				pos := in.Positions[i]
				sp := byISIN[strings.ToUpper(pos.ISIN)]
				dc := green
				if plan.Monthly[i] < int(math.Round(sp.Amount)) {
					dc = red
				}
				rows = append(rows, []Cell{
					plain(pos.ISIN), plain(pos.Name), plain(formatMoney(sp.Amount, "EUR")),
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
			return nil
		},
	}
	cmd.Flags().BoolVar(&all, "all", false, "preview every position whose plan differs from Scalable")
	cmd.AddCommand(newPushConfirmCmd())
	return cmd
}

func newPushConfirmCmd() *cobra.Command {
	var id string
	cmd := &cobra.Command{
		Use:   "confirm <ISIN>",
		Short: "Submit a previously previewed savings-plan change (phase 2)",
		Long: "Runs `sc broker savings-plans add --confirm` for one ISIN, using rebal's\n" +
			"currently computed monthly amount and the confirmation ID printed by a\n" +
			"prior `rebal-sc push` preview. This is the only rebal-sc command that\n" +
			"actually changes anything on Scalable — run it only after reviewing that\n" +
			"disclosure yourself, as a separate, deliberate step; rebal-sc never runs\n" +
			"this on its own.\n\n" +
			"If the plan has changed since the preview (e.g. after another `rebal-sc\n" +
			"sync`) or the confirmation has expired, sc rejects it rather than submit\n" +
			"something you didn't see.",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if id == "" {
				return fmt.Errorf("--id is required — copy it from the preview's disclosure")
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
			if in.Positions[idx].WindDown {
				return fmt.Errorf("%s is frozen in rebal — refusing to submit a plan for it", isin)
			}
			amount := plan.Monthly[idx]
			if amount <= 0 {
				return fmt.Errorf("%s's computed plan is %d € — nothing to confirm", isin, amount)
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
			fmt.Fprintf(cmd.OutOrStdout(), "\n%s%s: %s (%d €/month)\n\n", margin, isin, orDefault(r.Result.Action, "submitted"), amount)
			return nil
		},
	}
	cmd.Flags().StringVar(&id, "id", "", "confirmation ID printed by rebal-sc push")
	return cmd
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
