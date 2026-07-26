package ui

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/lukingo187/re.balance/internal/config"
	"github.com/lukingo187/re.balance/internal/state"
)

func sampleModel(t *testing.T) Model {
	t.Helper()
	cfg := config.Config{
		Settings: config.Settings{MonthlyTotal: 1000, DefaultMinMonthly: 25, HorizonMonths: 6, TolerancePP: 0.5, MaxHorizon: 36},
		Positions: []config.Position{
			{Name: "Prime Global", ISIN: "AAA", Target: 50},
			{Name: "Prime EM", ISIN: "BBB", Target: 30},
			{Name: "Europe SC", ISIN: "CCC", Target: 20},
		},
	}
	st := state.State{Values: map[string]int64{"AAA": 981800, "BBB": 488000, "CCC": 162900}}
	path := filepath.Join(t.TempDir(), "config.toml")
	return New(cfg, st, path, "")
}

// TestViewRenders checks the static screen composes and carries its key parts.
func TestViewRenders(t *testing.T) {
	m := sampleModel(t)
	m.width = 64
	content := m.View().Content

	for _, want := range []string{"re.balance", "total", "Position", "Prime Global", "Drift", "Monthly", "Horizon", "max drift", "quit"} {
		if !strings.Contains(content, want) {
			t.Errorf("view missing %q\n---\n%s", want, content)
		}
	}
	t.Logf("\n%s", content)
}

// TestFocusMarker: the focused row shows the ▸ marker.
func TestFocusMarker(t *testing.T) {
	m := sampleModel(t)
	m.focus = 1 // second position
	if !strings.Contains(m.View().Content, "▸") {
		t.Fatal("focused row has no marker")
	}
}

// TestSaveDefaults writes the current monthly/horizon to config.
func TestSaveDefaults(t *testing.T) {
	m := sampleModel(t)
	m.monthly = 1200
	m.horizon = 9
	m.saveDefaults()

	if !strings.Contains(m.status, "saved") {
		t.Fatalf("status = %q, want a saved confirmation", m.status)
	}
	got, err := config.Load(m.cfgPath)
	if err != nil {
		t.Fatalf("reloading written config: %v", err)
	}
	if got.Settings.MonthlyTotal != 1200 || got.Settings.HorizonMonths != 9 {
		t.Fatalf("saved settings = %+v, want monthly 1200 horizon 9", got.Settings)
	}
}

// TestRecomputeOnBadMonthly: dropping monthly below the floors raises an issue
// so the banner can show it.
func TestRecomputeOnBadMonthly(t *testing.T) {
	m := sampleModel(t)
	m.monthly = 0 // below the floors
	m.recompute()
	if len(m.issues) == 0 {
		t.Fatal("expected a validation issue at monthly 0")
	}
	if m.bannerView() == "" {
		t.Fatal("banner should be non-empty when there are issues")
	}
}
