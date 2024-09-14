package home

import (
	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/mintoolkit/mint/pkg/app/master/command"
	"github.com/mintoolkit/mint/pkg/app/master/command/images"
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
	Gcvalues *command.GenericParams
	mode     mode
}

func InitialModel(gcvalues *command.GenericParams) (tea.Model, tea.Cmd) {
	m := &Model{mode: nav, Gcvalues: gcvalues}

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
		case key.Matches(msg, keys.Home.Images):
			m.mode = image
			// TODO - Unhardcode the model index
			// Get the `images` data.
			getImagesEvent := common.Event{
				Type: common.GetImagesEvent,
				Data: m.Gcvalues,
			}
			loadModel := images.LoadModel()
			common.Models = append(common.Models, loadModel)
			return common.Models[1].Update(getImagesEvent)

			// TODO - support debug model
			// case key.Matches(msg, keys.Home.Debug):
			// 	m.mode = debug
			// 	// TODO - create blank debug model
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
	return common.HelpStyle("• i: Open images view • d: Open debug view • q: quit")
}
