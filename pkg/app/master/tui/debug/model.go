package debug

import (
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/lipgloss"
	"github.com/mintoolkit/mint/pkg/app/master/tui/common"
	"github.com/mintoolkit/mint/pkg/app/master/tui/keys"

	tea "github.com/charmbracelet/bubbletea"
)

// Model represents the state of the TUI.
type Model struct {
	standalone bool
}

// InitialModel returns the initial state of the model.
func InitialModel(standalone bool) *Model {
	m := &Model{
		standalone: standalone,
	}

	return m
}

func (m Model) Init() tea.Cmd {
	// Just return `nil`, which means "no I/O right now, please."
	return nil
}

// Update is called to handle user input and update the model's state.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch {
		case key.Matches(msg, keys.Global.Quit):
			return m, tea.Quit
		// NOTE -> We should only support this back navigation,
		// if the images tui is not standalone
		case key.Matches(msg, keys.Global.Back):
			return common.TUIsInstance.Home, nil
		}
	}
	return m, nil
}

// View returns the view that should be displayed.
func (m Model) View() string {
	var components []string

	content := "Debug support coming soon"

	components = append(components, content)

	components = append(components, m.help())

	return lipgloss.JoinVertical(lipgloss.Left,
		components...,
	)
}

func (m Model) help() string {
	if m.standalone {
		return common.HelpStyle("• q: quit")
	}
	return common.HelpStyle("• esc: back • q: quit")
}
