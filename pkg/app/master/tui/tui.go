package tui

import (
	"os"

	"log"

	tea "github.com/charmbracelet/bubbletea"
)

// RunTUI starts the TUI program.
func RunTUI(model tea.Model) {
	f, err := tea.LogToFile("debug.log", "debug")
	if err != nil {
		log.Printf("RunTUI Logging - %v", err)
		os.Exit(1)
	}

	defer f.Close()

	p := tea.NewProgram(model)
	if _, err := p.Run(); err != nil {
		log.Printf("RunTUI error - %v", err)
		os.Exit(1)
	}
}
