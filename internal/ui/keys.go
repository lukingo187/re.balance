package ui

import "charm.land/bubbles/v2/key"

// keyMap is the TUI keybindings. It also satisfies help.KeyMap so the footer is
// generated, not hand-rolled (spec §8).
type keyMap struct {
	Next key.Binding
	Prev key.Binding
	Save key.Binding
	Help key.Binding
	Quit key.Binding
}

func newKeyMap() keyMap {
	return keyMap{
		Next: key.NewBinding(key.WithKeys("tab"), key.WithHelp("tab", "next")),
		Prev: key.NewBinding(key.WithKeys("shift+tab"), key.WithHelp("shift+tab", "prev")),
		Save: key.NewBinding(key.WithKeys("w"), key.WithHelp("w", "save defaults")),
		Help: key.NewBinding(key.WithKeys("?"), key.WithHelp("?", "help")),
		Quit: key.NewBinding(key.WithKeys("q", "ctrl+c"), key.WithHelp("q", "quit")),
	}
}

func (k keyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.Next, k.Save, k.Help, k.Quit}
}

func (k keyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{{k.Next, k.Prev}, {k.Save, k.Help, k.Quit}}
}
