package home

import (
	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/mintoolkit/mint/pkg/app/master/command"
	"github.com/mintoolkit/mint/pkg/app/master/command/images"
	"github.com/mintoolkit/mint/pkg/app/master/tui/common"
	"github.com/mintoolkit/mint/pkg/app/master/tui/debug"
	"github.com/mintoolkit/mint/pkg/app/master/tui/keys"
)

type mode int

// Default TUI
type TUI struct {
	Gcvalues *command.GenericParams
}

func InitialTUI(gcvalues *command.GenericParams) (tea.Model, tea.Cmd) {
	m := &TUI{Gcvalues: gcvalues}

	return m, nil
}

func (m TUI) Init() tea.Cmd {
	// Just return `nil`, which means "no I/O right now, please."
	return nil
}

// Update is called to handle user input and update the model's state.
func (m TUI) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch {
		case key.Matches(msg, keys.Global.Quit):
			return m, tea.Quit // Quit the program.
		case key.Matches(msg, keys.Home.Images):
			getImagesEvent := common.Event{
				Type: common.GetImagesEvent,
				Data: m.Gcvalues,
			}

			LoadTUI := images.LoadTUI()
			common.TUIsInstance.Images = LoadTUI
			return LoadTUI.Update(getImagesEvent)
		case key.Matches(msg, keys.Home.Debug):
			debugModel := debug.InitialTUI(false)
			common.TUIsInstance.Debug = debugModel
			return debugModel.Update(nil)
		}
	}
	return m, nil
}

// View returns the view that should be displayed.
func (m TUI) View() string {
	content := "Dashboard\n Select which view you would like to open\n"

	return lipgloss.JoinVertical(lipgloss.Left,
		content,
		m.help(),
	)
}
func (m TUI) help() string {
	return common.HelpStyle("• i: Open images view • d: Open debug view • q: quit")
}
