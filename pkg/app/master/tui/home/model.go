package home

import (
	"log"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/mintoolkit/mint/pkg/app/master/tui/common"
	"github.com/mintoolkit/mint/pkg/app/master/tui/keys"
)

type mode int

const (
	nav mode = iota
	image
	debug
)

// Default Model
type Model struct {
	mode mode
}

func InitialModel() (tea.Model, tea.Cmd) {
	m := &Model{mode: nav}
	log.Printf("Welcome model initialized: %v", m)
	return m, nil
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
			return m, tea.Quit // Quit the program.
		case key.Matches(msg, keys.Welcome.Images):
			m.mode = image
			// TODO - Unhardcode the model index
			return common.Models[1].Update(nil)

			// TODO - support debug model
			// case key.Matches(msg, keys.Welcome.Debug):
			// 	m.mode = debug
			// 	// TODO - Unhardcode the model index
			// 	return common.Models[2].Update(nil)
		}
	}
	return m, nil
}

// View returns the view that should be displayed.
func (m Model) View() string {
	content := "Dashboard\n Select which view you would like to open\n"

	return lipgloss.JoinVertical(lipgloss.Left,
		content,
		m.help(),
	)
}
func (m Model) help() string {
	return common.HelpStyle("• i: Open images view • q: quit")
	// TODO - support debug view
	// return common.HelpStyle("• i: Open images view • d: Open debug view • q: quit")
}