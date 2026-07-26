package ui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"charm.land/bubbles/v2/textinput"

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
	dir := t.TempDir()
	return New(cfg, st, filepath.Join(dir, "config.toml"), filepath.Join(dir, "state.json"))
}

// editField opens the field for the current focus and sets its text.
func editField(m *Model, text string) {
	m.input = textinput.New()
	m.input.SetValue(text)
	m.editing = true
}

// TestViewRenders checks the static screen composes and carries its key parts.
func TestViewRenders(t *testing.T) {
	m := sampleModel(t)
	m.width = 64
	content := m.View().Content

	for _, want := range []string{"re.balance", "total", "Position", "Prime Global", "Drift", "Monthly", "Target horizon", "max drift", "quit"} {
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

// TestHorizonClamp: the cursor stays within [1, maxHorizon].
func TestHorizonClamp(t *testing.T) {
	m := sampleModel(t)
	m.setHorizon(100)
	if m.horizon != 36 {
		t.Fatalf("horizon = %d, want clamped to 36", m.horizon)
	}
	m.setHorizon(0)
	if m.horizon != 1 {
		t.Fatalf("horizon = %d, want clamped to 1", m.horizon)
	}
	m.setHorizon(12)
	if m.horizon != 12 {
		t.Fatalf("horizon = %d, want 12", m.horizon)
	}
}

// TestHorizonDriftReadout: the projected-drift readout tracks the horizon — a
// short horizon shows a bigger drift than a long one.
func TestHorizonDriftReadout(t *testing.T) {
	m := sampleModel(t)
	m.setHorizon(2)
	short := m.horizonDrift()
	m.setHorizon(18)
	long := m.horizonDrift()

	if short == "" || long == "" {
		t.Fatal("readout should be non-empty for a valid config")
	}
	if short == long {
		t.Fatal("readout should differ between horizons")
	}
	if m.plan.MaxDriftPP > m.cfg.Settings.TolerancePP {
		t.Fatalf("at horizon 18 drift %.2f should be within tolerance", m.plan.MaxDriftPP)
	}
}

// TestEditValuePersists: committing a position value updates state and writes it.
func TestEditValuePersists(t *testing.T) {
	m := sampleModel(t)
	m.focus = 0 // Prime Global / AAA
	editField(&m, "10000")

	next, _ := m.commitEdit()
	mm := next.(Model)
	if mm.editing {
		t.Fatal("should have left edit mode")
	}
	if mm.values["AAA"] != 1000000 {
		t.Fatalf("AAA = %d, want 1000000 cents", mm.values["AAA"])
	}
	loaded, err := state.Load(mm.statePath)
	if err != nil {
		t.Fatalf("loading written state: %v", err)
	}
	if loaded.Values["AAA"] != 1000000 {
		t.Fatalf("persisted AAA = %d, want 1000000", loaded.Values["AAA"])
	}
}

// TestEditMonthlyTransient: committing monthly updates the what-if value but does
// not write config.
func TestEditMonthlyTransient(t *testing.T) {
	m := sampleModel(t)
	m.focus = m.focusMonthly()
	editField(&m, "500")

	next, _ := m.commitEdit()
	mm := next.(Model)
	if mm.monthly != 500 {
		t.Fatalf("monthly = %d, want 500", mm.monthly)
	}
	if _, err := os.Stat(mm.cfgPath); err == nil {
		t.Fatal("editing monthly wrote config — it should be transient")
	}
}

// TestEditInvalidKeepsEditing: a bad amount is rejected without leaving the field.
func TestEditInvalidKeepsEditing(t *testing.T) {
	m := sampleModel(t)
	m.focus = 0
	editField(&m, "not-a-number")

	next, _ := m.commitEdit()
	mm := next.(Model)
	if !mm.editing {
		t.Fatal("should stay in edit mode on a bad amount")
	}
	if !strings.Contains(mm.status, "invalid") {
		t.Fatalf("status = %q, want an invalid-amount message", mm.status)
	}
}
