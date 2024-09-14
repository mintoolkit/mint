package images

import (
	"log"
	"time"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/lipgloss/table"
	"github.com/dustin/go-humanize"
	"github.com/mintoolkit/mint/pkg/app"

	"github.com/mintoolkit/mint/pkg/app/master/command"
	"github.com/mintoolkit/mint/pkg/app/master/tui/common"
	"github.com/mintoolkit/mint/pkg/app/master/tui/keys"
	"github.com/mintoolkit/mint/pkg/crt"
	"github.com/mintoolkit/mint/pkg/crt/docker/dockerutil"

	tea "github.com/charmbracelet/bubbletea"
)

// Model represents the state of the TUI.
type Model struct {
	table      table.Table
	width      int
	height     int
	standalone bool
	loading    bool
}

// Styles - move to `common`
const (
	gray      = lipgloss.Color("#737373")
	lightGray = lipgloss.Color("#d3d3d3")
	white     = lipgloss.Color("#ffffff")
)

var (
	// HeaderStyle is the lipgloss style used for the table headers.
	HeaderStyle = lipgloss.NewStyle().Foreground(white).Bold(true).Align(lipgloss.Center)
	// CellStyle is the base lipgloss style used for the table rows.
	CellStyle = lipgloss.NewStyle().Padding(0, 1).Width(14)
	// OddRowStyle is the lipgloss style used for odd-numbered table rows.
	OddRowStyle = CellStyle.Foreground(gray)
	// EvenRowStyle is the lipgloss style used for even-numbered table rows.
	EvenRowStyle = CellStyle.Foreground(lightGray)
	// BorderStyle is the lipgloss style used for the table border.
	BorderStyle = lipgloss.NewStyle().Foreground(white)
)

// End styles

func LoadModel() *Model {
	m := &Model{
		width:   20,
		height:  15,
		loading: true,
	}
	return m
}

// InitialModel returns the initial state of the model.
func InitialModel(images map[string]crt.BasicImageInfo, standalone bool) *Model {
	log.Printf("Images.InitialModel. images: %v", images)
	m := &Model{
		width:      20,
		height:     15,
		standalone: standalone,
	}
	var rows [][]string
	for k, v := range images {
		imageRow := []string{k, dockerutil.CleanImageID(v.ID)[:12], humanize.Time(time.Unix(v.Created, 0)), humanize.Bytes(uint64(v.Size))}
		rows = append(rows, imageRow)
	}

	t := table.New().
		Border(lipgloss.NormalBorder()).
		BorderStyle(BorderStyle).
		StyleFunc(func(row, col int) lipgloss.Style {
			var style lipgloss.Style

			switch {
			case row == 0:
				return HeaderStyle
			case row%2 == 0:
				style = EvenRowStyle
			default:
				style = OddRowStyle
			}

			return style
		}).
		Headers("Name", "ID", "Created", "Size").
		Rows(rows...)

	m.table = *t
	return m
}

func (m Model) Init() tea.Cmd {
	// Just return `nil`, which means "no I/O right now, please."
	return nil
}

// Update is called to handle user input and update the model's state.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {

	switch msg := msg.(type) {
	case common.Event:
		switch msg.Type {
		case common.HydrateImagesEvent:

			images, ok := msg.Data.(map[string]crt.BasicImageInfo)
			if !ok {
				return m, nil
			}
			m = Model{
				width:      20,
				height:     15,
				standalone: false,
			}
			var rows [][]string
			for k, v := range images {
				log.Printf("Image k: %v", k)
				log.Printf("Image v: %v", v)
				imageRow := []string{k, dockerutil.CleanImageID(v.ID)[:12], humanize.Time(time.Unix(v.Created, 0)), humanize.Bytes(uint64(v.Size))}
				rows = append(rows, imageRow)
			}

			t := table.New().
				Border(lipgloss.NormalBorder()).
				BorderStyle(BorderStyle).
				StyleFunc(func(row, col int) lipgloss.Style {
					var style lipgloss.Style

					switch {
					case row == 0:
						return HeaderStyle
					case row%2 == 0:
						style = EvenRowStyle
					default:
						style = OddRowStyle
					}

					return style
				}).
				Headers("Name", "ID", "Created", "Size").
				Rows(rows...)

			m.table = *t
			log.Printf("Hydrated m: %v", m)
			return m, nil
		case common.GetImagesEvent:
			xc := app.NewExecutionContext(
				"tui",
				true,
				"json",
			)

			cparams := &CommandParams{
				Runtime:   crt.AutoRuntime,
				GlobalTUI: true,
			}

			gcValue, ok := msg.Data.(*command.GenericParams)
			if !ok || gcValue == nil {
				return nil, nil
			}
			images := OnCommand(xc, gcValue, cparams)
			var rows [][]string
			for k, v := range images {
				imageRow := []string{k, dockerutil.CleanImageID(v.ID)[:12], humanize.Time(time.Unix(v.Created, 0)), humanize.Bytes(uint64(v.Size))}
				rows = append(rows, imageRow)
			}

			t := table.New().
				Border(lipgloss.NormalBorder()).
				BorderStyle(BorderStyle).
				StyleFunc(func(row, col int) lipgloss.Style {
					var style lipgloss.Style

					switch {
					case row == 0:
						return HeaderStyle
					case row%2 == 0:
						style = EvenRowStyle
					default:
						style = OddRowStyle
					}

					return style
				}).
				Headers("Name", "ID", "Created", "Size").
				Rows(rows...)

			m.table = *t
			return m, nil
		}

	case tea.WindowSizeMsg:
		m.table.Width(msg.Width)
		m.table.Height(msg.Height)

	case tea.KeyMsg:
		switch {
		case key.Matches(msg, keys.Global.Quit):
			return m, tea.Quit
		case key.Matches(msg, keys.Global.Back):
			return common.Models[0], nil
		}
	}
	return m, nil
}

// View returns the view that should be displayed.
func (m Model) View() string {
	var components []string

	content := m.table.String()

	components = append(components, content)

	components = append(components, m.help())

	return lipgloss.JoinVertical(lipgloss.Left,
		components...,
	)
}

func (m Model) help() string {
	if m.standalone {
		return common.HelpStyle("• q: quit")
	}
	return common.HelpStyle("• esc: back • q: quit")
}
