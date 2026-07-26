// Package ui is the re.balance TUI: a single screen over the same engine and
// config the CLI uses. It edits the "what-if" knobs live — monthly amount,
// horizon, current values — and re-solves synchronously on every change. Config
// structure (positions, targets, wind-down) stays a CLI-only operation; the
// TUI only ever persists current values, plus monthly/horizon on an explicit
// save (spec §8).
package ui

import (
	"fmt"

	"charm.land/bubbles/v2/help"
	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"

	"github.com/lukingo187/re.balance/internal/config"
	"github.com/lukingo187/re.balance/internal/rebal"
	"github.com/lukingo187/re.balance/internal/state"
)

// Model is the whole TUI state. The solver runs in microseconds, so everything
// recomputes synchronously in Update — no commands, goroutines or spinners.
type Model struct {
	cfg       config.Config
	values    map[string]int64
	cfgPath   string
	statePath string

	monthly int // live what-if value, starts from config
	horizon int // live what-if value, starts from config

	dark   bool
	width  int
	height int

	focus  int // 0..len(positions)-1 rows, then monthly, then chart
	nFocus int
	full   bool // full help shown
	status string

	keys   keyMap
	help   help.Model
	styles Styles

	plan   rebal.Plan
	issues []rebal.Issue
}

// New builds the model from an already-loaded config and state.
func New(cfg config.Config, st state.State, cfgPath, statePath string) Model {
	m := Model{
		cfg:       cfg,
		values:    st.Values,
		cfgPath:   cfgPath,
		statePath: statePath,
		monthly:   cfg.Settings.MonthlyTotal,
		horizon:   cfg.Settings.HorizonMonths,
		keys:      newKeyMap(),
		help:      help.New(),
		styles:    NewStyles(true),
	}
	if m.horizon < 1 {
		m.horizon = 1
	}
	if m.values == nil {
		m.values = map[string]int64{}
	}
	m.nFocus = len(cfg.Positions) + 2 // rows + monthly field + chart
	m.recompute()
	return m
}

// focusMonthly / focusChart are the two non-row focus stops.
func (m Model) focusMonthly() int { return len(m.cfg.Positions) }
func (m Model) focusChart() int   { return len(m.cfg.Positions) + 1 }

// recompute re-solves the plan for the current what-if values. Validation errors
// leave the previous plan in place; the banner reports them.
func (m *Model) recompute() {
	in := m.input()
	m.issues = rebal.Validate(in)
	if len(m.issues) == 0 {
		if p, err := rebal.Solve(in); err == nil {
			m.plan = p
		}
	}
}

// input builds the solver input from config plus the live what-if values.
func (m Model) input() rebal.Input {
	in := m.cfg.ToInput(m.values)
	in.Monthly = m.monthly
	in.Months = m.horizon
	return in
}

func (m Model) Init() tea.Cmd {
	return tea.RequestBackgroundColor
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
	case tea.BackgroundColorMsg:
		m.dark = msg.IsDark()
		m.styles = NewStyles(m.dark)
	case tea.KeyPressMsg:
		return m.handleKey(msg)
	}
	return m, nil
}

func (m Model) handleKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, m.keys.Quit):
		return m, tea.Quit
	case key.Matches(msg, m.keys.Next):
		m.focus = (m.focus + 1) % m.nFocus
		m.status = ""
	case key.Matches(msg, m.keys.Prev):
		m.focus = (m.focus - 1 + m.nFocus) % m.nFocus
		m.status = ""
	case key.Matches(msg, m.keys.Help):
		m.full = !m.full
	case key.Matches(msg, m.keys.Save):
		m.saveDefaults()
	}
	return m, nil
}

// saveDefaults writes the current monthly and horizon back to config as the new
// defaults — the one place the TUI persists a config change, and only on an
// explicit keypress.
func (m *Model) saveDefaults() {
	m.cfg.Settings.MonthlyTotal = m.monthly
	m.cfg.Settings.HorizonMonths = m.horizon
	if err := config.Write(m.cfgPath, m.cfg); err != nil {
		m.status = "save failed: " + err.Error()
		return
	}
	m.status = fmt.Sprintf("saved defaults — %d €/month · %d-month horizon", m.monthly, m.horizon)
}

// Run launches the program over an already-loaded config and state.
func Run(cfg config.Config, st state.State, cfgPath, statePath string) error {
	_, err := tea.NewProgram(New(cfg, st, cfgPath, statePath)).Run()
	return err
}
