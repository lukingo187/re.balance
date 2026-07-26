package ui

import "charm.land/lipgloss/v2"

// Styles holds every style the view uses, built once from the terminal's
// background (light or dark). No colours appear inline in the view — spec §9.
type Styles struct {
	Title      lipgloss.Style
	Total      lipgloss.Style
	Header     lipgloss.Style
	Name       lipgloss.Style
	NameFocus  lipgloss.Style
	Value      lipgloss.Style
	DriftUp    lipgloss.Style // over target
	DriftDown  lipgloss.Style // under target
	DriftZero  lipgloss.Style // within a hair of target
	Dim        lipgloss.Style
	Field      lipgloss.Style
	FieldFocus lipgloss.Style
	Cursor     lipgloss.Style
	Banner     lipgloss.Style
	Status     lipgloss.Style
	Box        lipgloss.Style
}

// NewStyles builds the palette for a dark or light terminal. Lip Gloss v2 has no
// AdaptiveColor, so we pick each colour explicitly via LightDark (spec §9).
func NewStyles(dark bool) Styles {
	ld := lipgloss.LightDark(dark)
	var (
		fg     = ld(lipgloss.Color("#1a1a1a"), lipgloss.Color("#e6e6e6"))
		muted  = ld(lipgloss.Color("#8a8a8a"), lipgloss.Color("#707070"))
		green  = ld(lipgloss.Color("#2e7d32"), lipgloss.Color("#5fd47a"))
		red    = ld(lipgloss.Color("#c62828"), lipgloss.Color("#ff6b6b"))
		accent = ld(lipgloss.Color("#1565c0"), lipgloss.Color("#5aa9ff"))
	)
	base := lipgloss.NewStyle()
	return Styles{
		Title:      base.Bold(true).Foreground(accent),
		Total:      base.Foreground(muted),
		Header:     base.Bold(true).Foreground(muted),
		Name:       base.Foreground(fg),
		NameFocus:  base.Bold(true).Foreground(accent),
		Value:      base.Foreground(fg),
		DriftUp:    base.Foreground(red),
		DriftDown:  base.Foreground(green),
		DriftZero:  base.Foreground(muted),
		Dim:        base.Foreground(muted),
		Field:      base.Foreground(fg),
		FieldFocus: base.Bold(true).Foreground(accent),
		Cursor:     base.Foreground(accent),
		Banner:     base.Foreground(red).Bold(true),
		Status:     base.Foreground(green),
		Box:        base.Border(lipgloss.RoundedBorder()).BorderForeground(muted).Padding(0, 1),
	}
}
