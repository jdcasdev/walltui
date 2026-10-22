package ui

import "charm.land/bubbles/v2/key"

// keymap holds every binding; matched with key.Matches on KeyPressMsg only.
// While search mode is active the root bypasses it entirely (typed letters
// are text, like fzf's enable-search).
type keymap struct {
	up     key.Binding
	down   key.Binding
	pageUp key.Binding
	pgDn   key.Binding
	home   key.Binding
	end    key.Binding
	mark   key.Binding
	trash  key.Binding
	search key.Binding
	apply  key.Binding
	quit   key.Binding
}

func newKeymap() keymap {
	return keymap{
		up: key.NewBinding(
			key.WithKeys("up", "k"),
			key.WithHelp("↑/k", "up"),
		),
		down: key.NewBinding(
			key.WithKeys("down", "j"),
			key.WithHelp("↓/j", "down"),
		),
		pageUp: key.NewBinding(
			key.WithKeys("pgup", "b"),
			key.WithHelp("pgup", "page up"),
		),
		pgDn: key.NewBinding(
			key.WithKeys("pgdown", "f"),
			key.WithHelp("pgdn", "page down"),
		),
		home: key.NewBinding(
			key.WithKeys("home", "g"),
			key.WithHelp("g/home", "first"),
		),
		end: key.NewBinding(
			key.WithKeys("end", "G"),
			key.WithHelp("G/end", "last"),
		),
		mark: key.NewBinding(
			key.WithKeys("space"),
			key.WithHelp("space", "mark"),
		),
		trash: key.NewBinding(
			key.WithKeys("d"),
			key.WithHelp("d", "trash"),
		),
		search: key.NewBinding(
			key.WithKeys("s"),
			key.WithHelp("s", "search"),
		),
		apply: key.NewBinding(
			key.WithKeys("enter"),
			key.WithHelp("enter", "apply"),
		),
		quit: key.NewBinding(
			key.WithKeys("esc", "ctrl+c"),
			key.WithHelp("esc", "quit"),
		),
	}
}

func (k keymap) ShortHelp() []key.Binding {
	return []key.Binding{k.up, k.down, k.mark, k.search, k.trash, k.apply, k.quit}
}

// FullHelp implements help.KeyMap: the grouped view (unused for now).
func (k keymap) FullHelp() [][]key.Binding {
	return [][]key.Binding{{k.up, k.down, k.pageUp, k.pgDn, k.home, k.end}, {k.mark, k.search, k.trash, k.apply, k.quit}}
}

// searchKeys is the footer key list while the search input has focus.
type searchKeys struct{}

func (searchKeys) bindings() []key.Binding {
	return []key.Binding{
		key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", "back to select")),
		key.NewBinding(key.WithKeys("up", "down"), key.WithHelp("↑/↓", "move")),
		key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "apply")),
	}
}

func (s searchKeys) ShortHelp() []key.Binding  { return s.bindings() }
func (s searchKeys) FullHelp() [][]key.Binding { return [][]key.Binding{s.bindings()} }
