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
	// We are running the tui via `mint tui`
	if !standalone {
		common.ModelsInstance.Home = model
	}
	common.P = tea.NewProgram(model, tea.WithAltScreen())

	if _, err := common.P.Run(); err != nil {
		log.Printf("RunTUI error - %v", err)
		os.Exit(1)
	}
}
