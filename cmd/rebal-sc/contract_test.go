package main

import (
	"encoding/json"
	"os"
	"os/exec"
	"slices"
	"strings"
	"testing"
)

// TestSCContract pins the parts of the real `sc` CLI that rebal-sc relies
// on: the commands it shells out to, their flags, and the two-phase
// savings-plan workflow. It runs only with SC_CONTRACT=1 (the CI sc-contract
// job, against the SC_VERSION Renovate keeps current), so a failing sc bump
// PR means the new release changed something this addon depends on.
//
// Everything here works logged out. Response field shapes (result.items,
// valuation, …) need a real account and stay covered by manual verification.
func TestSCContract(t *testing.T) {
	if os.Getenv("SC_CONTRACT") == "" {
		t.Skip("set SC_CONTRACT=1 to run against the installed sc CLI")
	}

	data, err := scRun("capabilities")
	if err != nil {
		t.Fatalf("sc capabilities: %v", err)
	}
	var caps struct {
		Version   string   `json:"version"`
		Output    string   `json:"output"`
		Commands  []string `json:"commands"`
		Workflows map[string]struct {
			Mode string `json:"mode"`
		} `json:"workflows"`
	}
	if err := json.Unmarshal(data, &caps); err != nil {
		t.Fatalf("parsing capabilities: %v", err)
	}
	t.Logf("sc %s", caps.Version)

	t.Run("json envelope output", func(t *testing.T) {
		if caps.Output != "json_envelope" {
			t.Errorf("output = %q, want json_envelope — scRun's envelope parsing assumes it", caps.Output)
		}
	})

	t.Run("commands rebal-sc calls still exist", func(t *testing.T) {
		for _, c := range []string{
			"broker.holdings",
			"broker.savings-plans",
			"broker.savings-plans.add",
			"broker.savings-plans.remove",
			"broker.watchlist",
		} {
			if !slices.Contains(caps.Commands, c) {
				t.Errorf("command %q is gone", c)
			}
		}
	})

	// push / push confirm exist to honor this split; see docs/rebal-sc.md.
	t.Run("savings-plans add is still two-phase", func(t *testing.T) {
		if got := caps.Workflows["broker.savings-plans.add"].Mode; got != "two_phase_confirmation" {
			t.Errorf("savings-plans.add mode = %q, want two_phase_confirmation", got)
		}
	})

	// push confirm --remove calls remove directly because it has no preview
	// step. If sc adds one, that flow needs redesigning, not just a pass.
	t.Run("savings-plans remove still has no workflow", func(t *testing.T) {
		if w, ok := caps.Workflows["broker.savings-plans.remove"]; ok {
			t.Errorf("savings-plans.remove now has a %q workflow — revisit push confirm --remove", w.Mode)
		}
	})

	t.Run("flags rebal-sc passes still exist", func(t *testing.T) {
		for cmd, flags := range map[string][]string{
			"savings-plans add":    {"--isin", "--amount", "--confirm", "--json"},
			"savings-plans remove": {"--isin", "--json"},
			"holdings":             {"--json"},
			"watchlist":            {"--json"},
		} {
			args := append([]string{"broker"}, strings.Fields(cmd)...)
			out, err := exec.Command("sc", append(args, "--help")...).Output()
			if err != nil {
				t.Errorf("sc broker %s --help: %v", cmd, err)
				continue
			}
			for _, f := range flags {
				if !strings.Contains(string(out), f+" ") {
					t.Errorf("sc broker %s: flag %s is gone", cmd, f)
				}
			}
		}
	})
}
