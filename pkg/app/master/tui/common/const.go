package common

import (
	tea "github.com/charmbracelet/bubbletea"
)

var (
	// P the current tea program
	P              *tea.Program
	ModelsInstance Models
)

type Models struct {
	Home   tea.Model
	Images tea.Model
	Debug  tea.Model
}
