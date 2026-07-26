package ui

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/lukingo187/re.balance/internal/money"
	"github.com/lukingo187/re.balance/internal/rebal"
	"github.com/lukingo187/re.balance/internal/ui/chart"
)

func (m Model) View() tea.View {
	in := m.solverInput()

	var sections []string
	if b := m.bannerView(); b != "" {
		sections = append(sections, b, "")
	}
	sections = append(sections, m.tableView(in), "", m.controlsView())
	if m.editing {
		sections = append(sections, m.editView())
	}
	sections = append(sections, "", m.chartView(in))
	if m.status != "" {
		sections = append(sections, "", m.styles.Status.Render(m.status))
	}
	sections = append(sections, "", m.helpView())

	body := lipgloss.JoinVertical(lipgloss.Left, sections...)
	width := lipgloss.Width(body)
	head := m.headerView(in, width)
	rule := m.styles.Dim.Render(strings.Repeat("─", width))
	content := lipgloss.JoinVertical(lipgloss.Left, head, rule, "", body)

	v := tea.NewView(m.styles.Box.Render(content))
	v.AltScreen = true
	return v
}

// headerView is the title on the left and the portfolio total on the right.
func (m Model) headerView(in rebal.Input, width int) string {
	var total int64
	for _, v := range in.Values {
		total += v
	}
	title := m.styles.Title.Render("re.balance")
	right := m.styles.Total.Render(money.Euro(total) + " total")
	gap := width - lipgloss.Width(title) - lipgloss.Width(right)
	if gap < 1 {
		gap = 1
	}
	return title + strings.Repeat(" ", gap) + right
}

// bannerView surfaces validation issues (e.g. monthly below the floors) as a
// non-blocking red line, or "" when the config is valid.
func (m Model) bannerView() string {
	if len(m.issues) == 0 {
		return ""
	}
	return m.styles.Banner.Render(fmt.Sprintf("⚠ %d issue(s): %s", len(m.issues), m.issues[0].Message))
}

// tableView renders the position table: value, current and target weight, drift
// and the resulting plan. The focused row is marked and bolded.
func (m Model) tableView(in rebal.Input) string {
	var activePool int64
	for i := range in.Positions {
		if !in.Positions[i].WindDown {
			activePool += in.Values[i]
		}
	}

	rows := [][]vcell{{
		{text: ""},
		{text: "Position", style: m.styles.Header},
		{text: "Value", style: m.styles.Header, right: true},
		{text: "Now", style: m.styles.Header, right: true},
		{text: "Target", style: m.styles.Header, right: true},
		{text: "Drift", style: m.styles.Header, right: true},
		{text: "Plan", style: m.styles.Header, right: true},
	}}

	for i := range in.Positions {
		p := in.Positions[i]
		mark, nameStyle := " ", m.styles.Name
		if m.focus == i {
			mark, nameStyle = "▸", m.styles.NameFocus
		}
		val := vcell{text: money.Euro(in.Values[i]), style: m.styles.Value, right: true}
		dash := vcell{text: "—", style: m.styles.DriftZero, right: true}

		row := []vcell{
			{text: mark, style: m.styles.Cursor},
			{text: p.Name, style: nameStyle},
			val,
		}
		switch {
		case p.WindDown:
			row = append(row, dash, dash, dash,
				vcell{text: "frozen", style: m.styles.DriftZero, right: true})
		case activePool == 0:
			row = append(row,
				dash,
				vcell{text: fmt.Sprintf("%.0f%%", p.TargetPct), style: m.styles.Value, right: true},
				dash,
				vcell{text: fmt.Sprintf("%d €", m.plan.Monthly[i]), style: m.styles.Value, right: true})
		default:
			nowPct := 100 * float64(in.Values[i]) / float64(activePool)
			drift := nowPct - p.TargetPct
			row = append(row,
				vcell{text: fmt.Sprintf("%.1f%%", nowPct), style: m.styles.Value, right: true},
				vcell{text: fmt.Sprintf("%.0f%%", p.TargetPct), style: m.styles.Value, right: true},
				vcell{text: fmt.Sprintf("%+.1fpp", drift), style: m.driftStyle(drift), right: true},
				vcell{text: fmt.Sprintf("%d €", m.plan.Monthly[i]), style: m.styles.Value, right: true})
		}
		rows = append(rows, row)
	}
	return renderCells(rows)
}

// controlsView is the monthly field (highlighted when focused) and the horizon
// (always accented — it's driven live by the arrow keys).
func (m Model) controlsView() string {
	mStyle := m.styles.Field
	if m.focus == m.focusMonthly() && !m.editing {
		mStyle = m.styles.FieldFocus
	}
	monthly := m.styles.Dim.Render("Monthly ") +
		mStyle.Render(fmt.Sprintf("[ %d ]", m.monthly)) + m.styles.Dim.Render(" €")
	horizon := m.styles.Dim.Render("Horizon ") +
		m.styles.FieldFocus.Render(fmt.Sprintf("%d", m.horizon)) +
		m.styles.Dim.Render(" months  ←/→")
	return monthly + m.styles.Dim.Render("     ") + horizon
}

// editView is the inline field shown while editing a value or the monthly amount.
func (m Model) editView() string {
	label := "monthly amount"
	if m.focus < len(m.cfg.Positions) {
		label = m.cfg.Positions[m.focus].Name + " value"
	}
	return m.styles.FieldFocus.Render("edit "+label+": ") + m.input.View() +
		m.styles.Dim.Render("   enter save · esc cancel")
}

// chartView renders the horizon chart with a marker at the current horizon.
func (m Model) chartView(in rebal.Input) string {
	max := m.cfg.Settings.MaxHorizon
	if max < 1 {
		max = 36
	}
	plans := rebal.Sweep(in, max)
	s := chart.Series{X: make([]int, max), Y: make([]float64, max)}
	for i := range plans {
		s.X[i] = i + 1
		s.Y[i] = plans[i].MaxDriftPP
	}
	return chart.Render(s, chart.Opts{
		Tolerance:      m.cfg.Settings.TolerancePP,
		Cursor:         m.horizon,
		StyleDrift:     func(x string) string { return m.styles.DriftUp.Render(x) },
		StyleConverged: func(x string) string { return m.styles.DriftDown.Render(x) },
		StyleDim:       func(x string) string { return m.styles.Dim.Render(x) },
	})
}

func (m Model) helpView() string {
	if m.full {
		return m.help.FullHelpView(m.keys.FullHelp())
	}
	return m.help.ShortHelpView(m.keys.ShortHelp())
}

func (m Model) driftStyle(pp float64) lipgloss.Style {
	switch {
	case pp > 0.05:
		return m.styles.DriftUp
	case pp < -0.05:
		return m.styles.DriftDown
	default:
		return m.styles.DriftZero
	}
}

// vcell is one table cell: text, its style, and whether to right-align.
type vcell struct {
	text  string
	style lipgloss.Style
	right bool
}

// renderCells lays rows out as an aligned grid. Column widths come from the
// visible text (lipgloss.Width ignores styling), so colour never shifts columns.
func renderCells(rows [][]vcell) string {
	var widths []int
	for _, r := range rows {
		for j, c := range r {
			for len(widths) <= j {
				widths = append(widths, 0)
			}
			if w := lipgloss.Width(c.text); w > widths[j] {
				widths[j] = w
			}
		}
	}
	lines := make([]string, len(rows))
	for i, r := range rows {
		parts := make([]string, len(r))
		for j, c := range r {
			st := c.style.Width(widths[j])
			if c.right {
				st = st.AlignHorizontal(lipgloss.Right)
			}
			parts[j] = st.Render(c.text)
		}
		lines[i] = strings.Join(parts, "  ")
	}
	return strings.Join(lines, "\n")
}
