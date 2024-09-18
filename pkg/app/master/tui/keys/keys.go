package keys

import (
	"github.com/charmbracelet/bubbles/key"
)

type global struct {
	Filter key.Binding
	Quit   key.Binding
	Help   key.Binding
	Back   key.Binding
}

var Global = global{
	Filter: key.NewBinding(
		key.WithKeys("/"),
		key.WithHelp(`/`, "filter"),
	),
	Quit: key.NewBinding(
		key.WithKeys("ctrl+c", "q"),
		key.WithHelp("ctrl+c/q", "quit"),
	),
	Back: key.NewBinding(
		key.WithKeys("esc"),
		key.WithHelp("esc", "back"),
	),
}

type home struct {
	Images key.Binding
	Debug  key.Binding
}

var Home = home{
	Images: key.NewBinding(
		key.WithKeys("i"),
		key.WithHelp("i", "Open images view"),
	),
	Debug: key.NewBinding(
		key.WithKeys("d"),
		key.WithHelp("d", "Open debug view"),
	),
}
