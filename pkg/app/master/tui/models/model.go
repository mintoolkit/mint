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

// Images represents the state of the TUI.
type Images struct {
	table  table.Model
	loaded bool
}

// InitialImages returns the initial state of the model.
func InitialImages(images map[string]crt.BasicImageInfo) *Images {
	var rows []table.Row
	for k, v := range images {
		imageRow := []string{k, dockerutil.CleanImageID(v.ID)[:12], humanize.Time(time.Unix(v.Created, 0)), humanize.Bytes(uint64(v.Size))}
		rows = append(rows, imageRow)
	}

	m := &Images{}
	columns := []table.Column{
		{Title: "Name", Width: 50},
		{Title: "Image ID", Width: 15},
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

func (m Images) Init() tea.Cmd {
	// Just return `nil`, which means "no I/O right now, please."
	return nil
}

// Update is called to handle user input and update the model's state.
func (m Images) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
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
func (m Images) View() string {
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

// Default Model
type Model struct{}

func InitialModel() *Model {
	m := &Model{}
	return m
}

func (m Model) Init() tea.Cmd {
	// Just return `nil`, which means "no I/O right now, please."
	return nil
}

// Update is called to handle user input and update the model's state.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
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
	content := "Coming soon..."

	footerStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#282828")).
		Background(lipgloss.Color("#7c6f64"))

	footerStr := "Press q to quit"
	footer := footerStyle.Render(footerStr)
	return lipgloss.JoinVertical(lipgloss.Left,
		content,
		footer,
	)
}
