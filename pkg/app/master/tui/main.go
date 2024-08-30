package tui

import (
	"os"

	log "github.com/sirupsen/logrus"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/mintoolkit/mint/pkg/app/master/tui/models"
)

// RunTUI starts the TUI program.
func RunTUI() {
	p := tea.NewProgram(models.InitialModel())
	if _, err := p.Run(); err != nil {
		log.WithError(err).Error("RunTUI error")

		os.Exit(1)
	}
}
