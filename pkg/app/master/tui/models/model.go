package models

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
)

// Model represents the state of the TUI.
type Model struct {
	// Add fields that represent the state of your TUI.
	count int
}

// InitialModel returns the initial state of the model.
func InitialModel() Model {
	return Model{
		count: 0, // Initialize any state variables here.
	}
}

func (m Model) Init() tea.Cmd {
	// Just return `nil`, which means "no I/O right now, please."
	return nil
}

// Update is called to handle user input and update the model's state.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit // Quit the program.
		case "up":
			m.count++
		case "down":
			m.count--
		}
	}
	return m, nil
}

// View returns the view that should be displayed.
func (m Model) View() string {
	return "Press q to quit.\n" + "Count: " + fmt.Sprint(m.count)
}
