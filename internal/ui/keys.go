package ui

import "charm.land/bubbles/v2/key"

// keyMap is the TUI keybindings. It also satisfies help.KeyMap so the footer is
// generated, not hand-rolled (spec §8).
type keyMap struct {
	Next      key.Binding
	Prev      key.Binding
	Edit      key.Binding
	Left      key.Binding
	Right     key.Binding
	JumpLeft  key.Binding
	JumpRight key.Binding
	Home      key.Binding
	End       key.Binding
	Save      key.Binding
	Help      key.Binding
	Quit      key.Binding
}

func newKeyMap() keyMap {
	return keyMap{
		Next:      key.NewBinding(key.WithKeys("tab"), key.WithHelp("tab", "next field")),
		Prev:      key.NewBinding(key.WithKeys("shift+tab"), key.WithHelp("shift+tab", "prev field")),
		Edit:      key.NewBinding(key.WithKeys("e", "enter"), key.WithHelp("e", "edit value")),
		Left:      key.NewBinding(key.WithKeys("left"), key.WithHelp("←/→", "horizon")),
		Right:     key.NewBinding(key.WithKeys("right")),
		JumpLeft:  key.NewBinding(key.WithKeys("shift+left")),
		JumpRight: key.NewBinding(key.WithKeys("shift+right")),
		Home:      key.NewBinding(key.WithKeys("home")),
		End:       key.NewBinding(key.WithKeys("end")),
		Save:      key.NewBinding(key.WithKeys("w"), key.WithHelp("w", "save defaults")),
		Help:      key.NewBinding(key.WithKeys("?"), key.WithHelp("?", "help")),
		Quit:      key.NewBinding(key.WithKeys("q", "ctrl+c"), key.WithHelp("q", "quit")),
	}
}

func (k keyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.Next, k.Edit, k.Left, k.Save, k.Quit}
}

func (k keyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{k.Next, k.Prev, k.Edit},
		{k.Left, k.Home, k.End},
		{k.Save, k.Help, k.Quit},
	}
}
