package common

import "github.com/charmbracelet/lipgloss"

var (
	Bold      = Regular.Bold(true)
	Border    = Regular.Border(lipgloss.NormalBorder())
	Regular   = lipgloss.NewStyle()
	Padded    = Regular.Padding(0, 1)
	HelpStyle = Regular.Padding(0, 1).Background(lipgloss.Color("#ffffff")).Foreground(lipgloss.Color("#000000")).Render
)
