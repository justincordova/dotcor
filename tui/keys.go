package tui

import (
	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"
)

type keyMap struct {
	Up       key.Binding
	Down     key.Binding
	Enter    key.Binding
	Tab      key.Binding
	Esc      key.Binding
	Stow     key.Binding
	Unstow   key.Binding
	Sync     key.Binding
	Add      key.Binding
	Diff     key.Binding
	History  key.Binding
	Logs     key.Binding
	Help     key.Binding
	Search   key.Binding
	Push     key.Binding
	Pull     key.Binding
	Settings key.Binding
	Quit     key.Binding
}

func newKeyMap() keyMap {
	return keyMap{
		Up:       key.NewBinding(key.WithKeys("up", "k"), key.WithHelp("↑/k", "up")),
		Down:     key.NewBinding(key.WithKeys("down", "j"), key.WithHelp("↓/j", "down")),
		Enter:    key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "expand")),
		Tab:      key.NewBinding(key.WithKeys("tab"), key.WithHelp("tab", "next")),
		Esc:      key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", "back")),
		Stow:     key.NewBinding(key.WithKeys("s"), key.WithHelp("s", "stow")),
		Unstow:   key.NewBinding(key.WithKeys("u"), key.WithHelp("u", "unstow")),
		Sync:     key.NewBinding(key.WithKeys("S"), key.WithHelp("S", "sync")),
		Add:      key.NewBinding(key.WithKeys("a"), key.WithHelp("a", "add")),
		Diff:     key.NewBinding(key.WithKeys("D"), key.WithHelp("D", "diff")),
		History:  key.NewBinding(key.WithKeys("H"), key.WithHelp("H", "history")),
		Logs:     key.NewBinding(key.WithKeys("L"), key.WithHelp("L", "logs")),
		Help:     key.NewBinding(key.WithKeys("?"), key.WithHelp("?", "help")),
		Search:   key.NewBinding(key.WithKeys("/"), key.WithHelp("/", "search")),
		Push:     key.NewBinding(key.WithKeys("p"), key.WithHelp("p", "push")),
		Pull:     key.NewBinding(key.WithKeys("P"), key.WithHelp("P", "pull")),
		Settings: key.NewBinding(key.WithKeys("g"), key.WithHelp("g", "settings")),
		Quit:     key.NewBinding(key.WithKeys("q", "ctrl+c"), key.WithHelp("q", "quit")),
	}
}

func (k keyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.Help, k.Add, k.Stow, k.Unstow, k.Sync, k.Search, k.Quit}
}

func (k keyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{k.Up, k.Down, k.Enter, k.Esc},
		{k.Stow, k.Unstow, k.Add, k.Sync},
		{k.Push, k.Pull, k.Diff, k.History},
		{k.Logs, k.Settings, k.Search, k.Help},
		{k.Quit},
	}
}

func newHelpModel() help.Model {
	h := help.New()
	h.Styles.ShortKey = keyStyle
	h.Styles.ShortDesc = descStyle
	h.Styles.FullKey = keyStyle
	h.Styles.FullDesc = descStyle
	h.Styles.FullSeparator = descStyle
	h.ShowAll = false
	return h
}
