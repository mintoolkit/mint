package models

import (
	"time"

	"github.com/charmbracelet/bubbles/table"
	"github.com/charmbracelet/lipgloss"
	"github.com/dustin/go-humanize"
	"github.com/mintoolkit/mint/pkg/crt"
	"github.com/mintoolkit/mint/pkg/crt/docker/dockerutil"

	tea "github.com/charmbracelet/bubbletea"
)

// Model represents the state of the TUI.
type Model struct {
	table  table.Model
	loaded bool
}

// InitialImagesModel returns the initial state of the model.
func InitialImagesModel(images map[string]crt.BasicImageInfo) *Model {
	var rows []table.Row
	for k, v := range images {
		imageRow := []string{k, dockerutil.CleanImageID(v.ID)[:12], humanize.Time(time.Unix(v.Created, 0)), humanize.Bytes(uint64(v.Size))}
		rows = append(rows, imageRow)
	}

	m := &Model{}
	columns := []table.Column{
		{Title: "Name", Width: 50},
		{Title: "Image ID", Width: 10},
		{Title: "Created", Width: 30},
		{Title: "Size", Width: 8},
	}
	table := table.New(
		table.WithColumns(columns),
		table.WithRows(rows),
		table.WithHeight(7),
	)

	m.table = table
	return m
}

func (m Model) Init() tea.Cmd {
	// Just return `nil`, which means "no I/O right now, please."
	return nil
}

// Update is called to handle user input and update the model's state.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		if !m.loaded {
			m.loaded = true
		}
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit // Quit the program.
		}
	}
	return m, nil
}

// View returns the view that should be displayed.
func (m Model) View() string {
	if m.loaded {
		content := m.table.View()

		footerStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("#282828")).
			Background(lipgloss.Color("#7c6f64"))

		footerStr := "Press q to quit"
		footer := footerStyle.Render(footerStr)
		return lipgloss.JoinVertical(lipgloss.Left,
			content,
			footer,
		)
	} else {
		return "loading"
	}
}
