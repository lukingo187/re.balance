package main

import (
	"bytes"
	"strings"
	"testing"
)

// realSavingsPlanAddData is the "data" payload from a real
// `sc broker savings-plans add --isin IE00B4L5Y983 --amount 60 --json`
// phase-1 preview, captured live against a real Scalable account
// (2026-08-26, before the machine locked and further live sc calls became
// impossible for this session — see the sc-cli-signing memory). Trimmed to
// the "presentation" object, which is all parsePresentation reads; the
// surrounding fields (compliance text, duplicate required_leaf_paths list)
// are omitted since they add nothing.
const realSavingsPlanAddData = `{
  "account_id": "acct00000000000000000",
  "portfolio_id": "pf000000000000000000",
  "result": {
    "action": "preview",
    "next_step": "confirm_with_id",
    "presentation": {
      "savings_plan": {
        "amount": "60",
        "cost_venue": "SEIX",
        "effective_configuration": {
          "day_of_month": 4,
          "dynamization_rate": "0",
          "frequency": "MONTHLY",
          "payment_method": "REFERENCE_ACCOUNT",
          "year_month": "2026-09"
        },
        "security": {
          "isin": "IE00B4L5Y983",
          "name": "iShares Core MSCI World (Acc)",
          "security_type": "ETF"
        }
      },
      "ex_ante_costs": {
        "id": "pf000000000000000000-IE00B4L5Y983",
        "entryCosts": {
          "productCosts": {"amount": 0, "percentage": 0},
          "serviceCosts": {"amount": 0, "percentage": 0},
          "total": {"amount": 0, "percentage": 0}
        },
        "ongoingCosts": {
          "productCosts": {"amount": 0.19, "percentage": 0.00027},
          "serviceCosts": {"amount": 0, "percentage": 0},
          "total": {"amount": 0.19, "percentage": 0.00027}
        },
        "exitCosts": {
          "productCosts": {"amount": 0, "percentage": 0},
          "serviceCosts": {"amount": 0.99, "percentage": 0.00138},
          "total": {"amount": 0.99, "percentage": 0.00138}
        },
        "effectOnReturn": {
          "initialYearCosts": {"amount": 0.19, "percentage": 0.00027},
          "followingYearsCosts": {"amount": 0.19, "percentage": 0.00027},
          "finalYearCosts": {"amount": 1.18, "percentage": 0.00165}
        },
        "fiveYearsCosts": null,
        "incidentalCosts": {"amount": 0, "percentage": 0}
      },
      "confirmation": {
        "command_template": "sc broker savings-plans add --isin IE00B4L5Y983 --amount 60 --confirm scsp1_00000000000000000000000000000000 --json",
        "expires_at_epoch": 1787763575,
        "id": "scsp1_00000000000000000000000000000000",
        "instruction": "Present every value in this disclosure to the client in a human-readable summary. Obtain an explicit affirmative confirmation in a separate interaction before running phase 2."
      },
      "section_order": ["savings_plan", "ex_ante_costs", "confirmation"]
    },
    "security": {
      "isin": "IE00B4L5Y983",
      "name": "iShares Core MSCI World (Acc)",
      "security_type": "ETF"
    }
  }
}`

func TestParsePresentationRealResponse(t *testing.T) {
	p, err := parsePresentation([]byte(realSavingsPlanAddData))
	if err != nil {
		t.Fatalf("parsePresentation: %v", err)
	}
	if p.SavingsPlan.Security.ISIN != "IE00B4L5Y983" {
		t.Errorf("ISIN = %q, want IE00B4L5Y983", p.SavingsPlan.Security.ISIN)
	}
	if p.SavingsPlan.Amount != "60" {
		t.Errorf("Amount = %q, want \"60\"", p.SavingsPlan.Amount)
	}
	if p.SavingsPlan.EffectiveConfig.Frequency != "MONTHLY" {
		t.Errorf("Frequency = %q, want MONTHLY", p.SavingsPlan.EffectiveConfig.Frequency)
	}
	if p.ExAnteCosts.FiveYearsCosts != nil {
		t.Errorf("FiveYearsCosts = %+v, want nil (sc returned JSON null)", p.ExAnteCosts.FiveYearsCosts)
	}
	if p.ExAnteCosts.ExitCosts == nil || p.ExAnteCosts.ExitCosts.ServiceCosts == nil || *p.ExAnteCosts.ExitCosts.ServiceCosts.Amount != 0.99 {
		t.Errorf("ExitCosts.ServiceCosts.Amount = %+v, want 0.99", p.ExAnteCosts.ExitCosts)
	}
	if p.Confirmation.ID != "scsp1_00000000000000000000000000000000" {
		t.Errorf("Confirmation.ID = %q", p.Confirmation.ID)
	}
	if p.Confirmation.ExpiresAtEpoch != 1787763575 {
		t.Errorf("ExpiresAtEpoch = %d", p.Confirmation.ExpiresAtEpoch)
	}
}

// TestPrintDisclosureRealResponse checks the renderer doesn't panic on a real
// response and that every value sc requires disclosed actually appears in the
// output — including the null five-year cost, shown literally rather than
// dropped (sc's own "display_null_as_literal" requirement).
func TestPrintDisclosureRealResponse(t *testing.T) {
	p, err := parsePresentation([]byte(realSavingsPlanAddData))
	if err != nil {
		t.Fatalf("parsePresentation: %v", err)
	}
	var buf bytes.Buffer
	printDisclosure(&buf, p)
	out := buf.String()

	for _, want := range []string{
		"iShares Core MSCI World (Acc)", "IE00B4L5Y983",
		"MONTHLY", "SEIX",
		"scsp1_00000000000000000000000000000000",
		"--confirm scsp1_00000000000000000000000000000000",
		"rebal-sc push confirm IE00B4L5Y983 --id scsp1_00000000000000000000000000000000",
		"null", // fiveYearsCosts, shown literally per sc's presentation spec
	} {
		if !strings.Contains(out, want) {
			t.Errorf("printDisclosure output missing %q\n--- got ---\n%s", want, out)
		}
	}
}
