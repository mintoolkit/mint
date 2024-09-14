package tui

import (
	"os"

	"log"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/mintoolkit/mint/pkg/app/master/tui/common"
)

// RunTUI starts the TUI program.
func RunTUI(model tea.Model, standalone bool) {
	f, err := tea.LogToFile("debug.log", "debug")
	if err != nil {
		log.Printf("RunTUI Logging - %v", err)
		os.Exit(1)
	}
	defer f.Close()
	// We are running the tui via `debug --tui` || `images --tui`
	if standalone {
		common.Models = []tea.Model{model}
	} else {
		common.Models = []tea.Model{model}

		// TODO - rather than using mockdata, determine the flow to hydrate the model
		// with images data.
		// debugModel := debug.InitialModel(false)
		// imagesModel := images.LoadModel()
		// common.Models = []tea.Model{model, imagesModel}
		// common.Models = []tea.Model{model, images.InitialModel(mockData, false), debugModel}
	}
	// We are running the tui via `mint --tui`
	common.P = tea.NewProgram(model, tea.WithAltScreen())

	if _, err := common.P.Run(); err != nil {
		log.Printf("RunTUI error - %v", err)
		os.Exit(1)
	}
}
