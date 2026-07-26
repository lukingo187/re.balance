// Package ui is the re.balance TUI: a single screen over the same engine and
// config the CLI uses. It edits the "what-if" knobs live — monthly amount,
// horizon, current values — and re-solves synchronously on every change. Config
// structure (positions, targets, wind-down) stays a CLI-only operation; the
// TUI only ever persists current values, plus monthly/horizon on an explicit
// save (spec §8).
package ui

import (
	"fmt"
	"strconv"
	"time"

	"charm.land/bubbles/v2/help"
	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"

	"github.com/lukingo187/re.balance/internal/config"
	"github.com/lukingo187/re.balance/internal/money"
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

	monthly    int // live what-if value, starts from config
	horizon    int // live what-if value, starts from config
	maxHorizon int // sweep upper bound

	dark   bool
	width  int
	height int

	focus  int // 0..len(positions)-1 rows, then the monthly field
	nFocus int
	full   bool // full help shown
	status string

	editing bool // a value/monthly field is open for editing
	input   textinput.Model

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
	m.maxHorizon = cfg.Settings.MaxHorizon
	if m.maxHorizon < 1 {
		m.maxHorizon = 36
	}
	if m.horizon < 1 {
		m.horizon = 1
	}
	if m.horizon > m.maxHorizon {
		m.horizon = m.maxHorizon
	}
	if m.values == nil {
		m.values = map[string]int64{}
	}
	m.nFocus = len(cfg.Positions) + 1 // rows + the monthly field
	m.recompute()
	return m
}

// focusMonthly is the focus index of the monthly field (after the rows). The
// horizon is driven by the arrow keys, not the focus ring.
func (m Model) focusMonthly() int { return len(m.cfg.Positions) }

// recompute re-solves the plan for the current what-if values. Validation errors
// leave the previous plan in place; the banner reports them.
func (m *Model) recompute() {
	in := m.solverInput()
	m.issues = rebal.Validate(in)
	if len(m.issues) == 0 {
		if p, err := rebal.Solve(in); err == nil {
			m.plan = p
		}
	}
}

// solverInput builds the solver input from config plus the live what-if values.
func (m Model) solverInput() rebal.Input {
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
	if m.editing {
		return m.handleEditKey(msg)
	}
	switch {
	case key.Matches(msg, m.keys.Quit):
		return m, tea.Quit
	case key.Matches(msg, m.keys.Next):
		m.focus = (m.focus + 1) % m.nFocus
		m.status = ""
	case key.Matches(msg, m.keys.Prev):
		m.focus = (m.focus - 1 + m.nFocus) % m.nFocus
		m.status = ""
	case key.Matches(msg, m.keys.Left):
		m.setHorizon(m.horizon - 1)
	case key.Matches(msg, m.keys.Right):
		m.setHorizon(m.horizon + 1)
	case key.Matches(msg, m.keys.JumpLeft):
		m.setHorizon(m.horizon - 6)
	case key.Matches(msg, m.keys.JumpRight):
		m.setHorizon(m.horizon + 6)
	case key.Matches(msg, m.keys.Home):
		m.setHorizon(1)
	case key.Matches(msg, m.keys.End):
		m.setHorizon(m.maxHorizon)
	case key.Matches(msg, m.keys.Edit):
		return m.startEdit()
	case key.Matches(msg, m.keys.Help):
		m.full = !m.full
	case key.Matches(msg, m.keys.Save):
		m.saveDefaults()
	}
	return m, nil
}

// setHorizon moves the horizon cursor, clamped to [1, maxHorizon], and re-solves.
func (m *Model) setHorizon(h int) {
	switch {
	case h < 1:
		h = 1
	case h > m.maxHorizon:
		h = m.maxHorizon
	}
	m.horizon = h
	m.status = ""
	m.recompute()
}

// startEdit opens a text field over the focused value or the monthly amount.
func (m Model) startEdit() (tea.Model, tea.Cmd) {
	ti := textinput.New()
	ti.SetVirtualCursor(true)
	ti.SetWidth(14)
	if m.focus == m.focusMonthly() {
		ti.SetValue(strconv.Itoa(m.monthly))
	} else {
		ti.SetValue(plainAmount(m.values[m.cfg.Positions[m.focus].ISIN]))
	}
	m.input = ti
	m.editing = true
	m.status = ""
	return m, m.input.Focus()
}

func (m Model) handleEditKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "enter":
		return m.commitEdit()
	case "esc":
		m.editing = false
		m.input.Blur()
		return m, nil
	}
	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	return m, cmd
}

// commitEdit parses the field and applies it: a position value persists to state,
// the monthly amount is a transient what-if. A bad amount keeps the field open.
func (m Model) commitEdit() (tea.Model, tea.Cmd) {
	cents, err := money.Parse(m.input.Value())
	if err != nil || cents < 0 {
		m.status = "invalid amount — fix or press esc"
		return m, nil
	}
	if m.focus == m.focusMonthly() {
		m.monthly = int(cents / 100) // whole euros; transient
	} else {
		m.values[m.cfg.Positions[m.focus].ISIN] = cents
		m.saveValues()
	}
	m.editing = false
	m.input.Blur()
	m.recompute()
	return m, nil
}

// saveValues persists current values to state.json — the TUI's one automatic
// write (values are facts worth keeping; monthly/horizon are not).
func (m *Model) saveValues() {
	st := state.State{Values: m.values, Updated: time.Now().UTC().Format(time.RFC3339)}
	if err := state.Save(m.statePath, st); err != nil {
		m.status = "value set (save failed: " + err.Error() + ")"
		return
	}
	m.status = "saved value"
}

// plainAmount renders cents as a bare editable number: 981800 -> "9818,00",
// 100000 -> "1000".
func plainAmount(cents int64) string {
	if cents%100 == 0 {
		return strconv.FormatInt(cents/100, 10)
	}
	return fmt.Sprintf("%d,%02d", cents/100, cents%100)
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
