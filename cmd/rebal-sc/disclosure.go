package main

import (
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"time"
)

// The types below mirror the "presentation" object sc's own
// `savings-plans add` phase-1 response returns — sc's capabilities spec
// (rule_id "savings_plan_ex_ante_full_disclosure_v1") requires every one of
// these leaf values to be shown to the user, in this order, before any
// phase-2 confirmation, with nulls shown literally rather than omitted. This
// package doesn't reformat or drop anything from that list; costPair's
// String() prints "null" for a missing amount/percentage rather than hiding
// the row.

type costPair struct {
	Amount     *float64 `json:"amount"`
	Percentage *float64 `json:"percentage"`
}

func (c *costPair) amountStr() string {
	if c == nil || c.Amount == nil {
		return "null"
	}
	return formatMoney(*c.Amount, "EUR")
}

func (c *costPair) pctStr() string {
	if c == nil || c.Percentage == nil {
		return "null"
	}
	return strconv.FormatFloat(*c.Percentage*100, 'f', 3, 64) + "%"
}

type costBreakdown struct {
	ProductCosts *costPair `json:"productCosts"`
	ServiceCosts *costPair `json:"serviceCosts"`
	Total        *costPair `json:"total"`
}

type effectOnReturn struct {
	InitialYearCosts    *costPair `json:"initialYearCosts"`
	FollowingYearsCosts *costPair `json:"followingYearsCosts"`
	FinalYearCosts      *costPair `json:"finalYearCosts"`
}

type exAnteCosts struct {
	ID              string          `json:"id"`
	EntryCosts      *costBreakdown  `json:"entryCosts"`
	OngoingCosts    *costBreakdown  `json:"ongoingCosts"`
	ExitCosts       *costBreakdown  `json:"exitCosts"`
	EffectOnReturn  *effectOnReturn `json:"effectOnReturn"`
	FiveYearsCosts  *costPair       `json:"fiveYearsCosts"`
	IncidentalCosts *costPair       `json:"incidentalCosts"`
}

type effectiveConfig struct {
	DayOfMonth       int    `json:"day_of_month"`
	DynamizationRate string `json:"dynamization_rate"`
	Frequency        string `json:"frequency"`
	PaymentMethod    string `json:"payment_method"`
	YearMonth        string `json:"year_month"`
}

type planSecurity struct {
	ISIN         string `json:"isin"`
	Name         string `json:"name"`
	SecurityType string `json:"security_type"`
}

type savingsPlanPreview struct {
	Amount          string          `json:"amount"`
	CostVenue       string          `json:"cost_venue"`
	EffectiveConfig effectiveConfig `json:"effective_configuration"`
	Security        planSecurity    `json:"security"`
}

type confirmation struct {
	CommandTemplate string `json:"command_template"`
	ExpiresAtEpoch  int64  `json:"expires_at_epoch"`
	ID              string `json:"id"`
	Instruction     string `json:"instruction"`
}

type presentation struct {
	SavingsPlan  savingsPlanPreview `json:"savings_plan"`
	ExAnteCosts  exAnteCosts        `json:"ex_ante_costs"`
	Confirmation confirmation       `json:"confirmation"`
}

// previewSavingsPlanAdd runs sc's phase-1 preview for one ISIN — side-effect
// free by sc's own design ("this does not create or update a plan") — and
// returns the disclosure sc requires be shown before any confirmation.
func previewSavingsPlanAdd(isin string, amountEUR int) (presentation, error) {
	data, err := scRun("broker", "savings-plans", "add", "--isin", isin, "--amount", strconv.Itoa(amountEUR))
	if err != nil {
		return presentation{}, err
	}
	return parsePresentation(data)
}

// parsePresentation pulls the "presentation" object out of a
// `savings-plans add` phase-1 data payload. Split out from
// previewSavingsPlanAdd so it can be unit-tested against a captured real
// response without shelling out to sc.
func parsePresentation(data []byte) (presentation, error) {
	var r struct {
		Result struct {
			Presentation presentation `json:"presentation"`
		} `json:"result"`
	}
	if err := json.Unmarshal(data, &r); err != nil {
		return presentation{}, fmt.Errorf("parsing savings-plans add preview: %w", err)
	}
	return r.Result.Presentation, nil
}

// printDisclosure prints every required leaf value from a phase-1 preview, in
// the section order sc specifies (savings_plan, ex_ante_costs, confirmation).
func printDisclosure(w io.Writer, p presentation) {
	fmt.Fprintf(w, "\n%s%s (%s)\n\n", margin, p.SavingsPlan.Security.Name, p.SavingsPlan.Security.ISIN)

	amt, err := strconv.ParseFloat(p.SavingsPlan.Amount, 64)
	amtStr := p.SavingsPlan.Amount
	if err == nil {
		amtStr = formatMoney(amt, "EUR")
	}
	table(w, [][]Cell{
		{plain("Amount"), plain(amtStr)},
		{plain("Frequency"), plain(p.SavingsPlan.EffectiveConfig.Frequency)},
		{plain("Day of month"), plain(fmt.Sprintf("%d", p.SavingsPlan.EffectiveConfig.DayOfMonth))},
		{plain("Starts"), plain(p.SavingsPlan.EffectiveConfig.YearMonth)},
		{plain("Dynamization rate"), plain(p.SavingsPlan.EffectiveConfig.DynamizationRate)},
		{plain("Payment method"), plain(p.SavingsPlan.EffectiveConfig.PaymentMethod)},
		{plain("Cost venue"), plain(p.SavingsPlan.CostVenue)},
	}, []bool{false, false})

	fmt.Fprintf(w, "\n%sCosts %s\n\n", margin, dim("(ex-ante disclosure "+p.ExAnteCosts.ID+")"))
	rows := [][]Cell{header("", "Amount", "% of investment")}
	row := func(label string, cp *costPair) {
		rows = append(rows, []Cell{plain(label), plain(cp.amountStr()), plain(cp.pctStr())})
	}
	var entry, ongoing, exit *costBreakdown = p.ExAnteCosts.EntryCosts, p.ExAnteCosts.OngoingCosts, p.ExAnteCosts.ExitCosts
	breakdown := func(label string, b *costBreakdown) {
		if b == nil {
			row(label+" — product", nil)
			row(label+" — service", nil)
			row(label+" — total", nil)
			return
		}
		row(label+" — product", b.ProductCosts)
		row(label+" — service", b.ServiceCosts)
		row(label+" — total", b.Total)
	}
	breakdown("Entry", entry)
	breakdown("Ongoing", ongoing)
	breakdown("Exit", exit)
	if eor := p.ExAnteCosts.EffectOnReturn; eor != nil {
		row("Effect on return — initial year", eor.InitialYearCosts)
		row("Effect on return — following years", eor.FollowingYearsCosts)
		row("Effect on return — final year", eor.FinalYearCosts)
	} else {
		row("Effect on return — initial year", nil)
		row("Effect on return — following years", nil)
		row("Effect on return — final year", nil)
	}
	row("Five-year costs", p.ExAnteCosts.FiveYearsCosts)
	row("Incidental costs", p.ExAnteCosts.IncidentalCosts)
	table(w, rows, []bool{false, true, true})

	expires := time.Unix(p.Confirmation.ExpiresAtEpoch, 0).Local().Format("15:04:05")
	fmt.Fprintf(w, "\n%sconfirmation id: %s  %s\n", margin, p.Confirmation.ID, dim("(expires "+expires+")"))
	fmt.Fprintf(w, "%sthis previews only — nothing was submitted. Review the above, then either\n", margin)
	fmt.Fprintf(w, "%srun yourself:\n\n%s  %s\n\n", margin, margin, p.Confirmation.CommandTemplate)
	fmt.Fprintf(w, "%sor, as a separate deliberate step:\n\n%s  rebal-sc push confirm %s --id %s\n\n",
		margin, margin, p.SavingsPlan.Security.ISIN, p.Confirmation.ID)
}
