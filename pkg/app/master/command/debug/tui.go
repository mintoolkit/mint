package debug

import (
	"strconv"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/lipgloss/table"

	"github.com/mintoolkit/mint/pkg/app"
	"github.com/mintoolkit/mint/pkg/app/master/command"
	"github.com/mintoolkit/mint/pkg/app/master/tui/common"
	"github.com/mintoolkit/mint/pkg/app/master/tui/keys"

	tea "github.com/charmbracelet/bubbletea"
)

// TUI represents the internal state of the terminal user interface.
type TUI struct {
	width      int
	height     int
	standalone bool
	loading    bool
	table      table.Table

	showDebuggableContainers bool

	gcvalues *command.GenericParams
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

// End Styles - move to common - block

func LoadTUI() *TUI {
	m := &TUI{
		width:   20,
		height:  15,
		loading: true,
	}
	return m
}

// InitialTUI returns the initial state of the model.
func InitialTUI(standalone bool, gcvalues *command.GenericParams) *TUI {
	m := &TUI{
		standalone: standalone,
		width:      20,
		height:     15,
		gcvalues:   gcvalues,
	}

	return m
}

func (m TUI) Init() tea.Cmd {
	// Just return `nil`, which means "no I/O right now, please."
	return nil
}

type DebuggableContainer struct {
	Name  string
	Image string
}

// Update is called to handle user input and update the model's state.
func (m TUI) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case common.Event:
		debuggableContainersCh := make(chan interface{})
		// NOTE -> the names of both the channel map and the channel are misleading
		// as more than just the debuggable container information is dumped on it
		// at the moment.
		debuggableContainersChannelMap := map[string]chan interface{}{
			"debuggableContainers": debuggableContainersCh,
		}
		// In addition to passing the channel(s) we will use to transport data
		// we should pass:
		// the outputs we want to subscribe to: State | Info | Error
		xc := app.NewExecutionContext(
			"tui",
			// Quiet -> when set to true, returns on the first line for each
			// Execution context method
			true,
			"subscription",
			debuggableContainersChannelMap,
		)

		cparams := &CommandParams{
			// NOTE -> should not always pass docker here.
			Runtime: "docker",
			// Note -> we should not pass this by default, and instead pass it when a user asks.
			ActionListDebuggableContainers: true,
			// How to pass the target ref:
			// TargetRef: "my-nginx"
		}

		gcValue, ok := msg.Data.(*command.GenericParams)
		if !ok || gcValue == nil {
			return nil, nil
		}

		go OnCommand(xc, gcValue, cparams)

		counter := 0
		var counterCeiling int
		var debuggableContainers []DebuggableContainer

		doneCh := make(chan struct{})
		go func() {
			for debuggableContainersData := range debuggableContainersCh {
				channelResponse, ok := debuggableContainersData.(map[string]string)
				if !ok || channelResponse == nil {
					continue
				}
				infoValue, infoExists := channelResponse["info"]
				if infoExists {
					// Set total debuggable container counter ceiling
					if infoValue == "debuggable.containers" && counterCeiling == 0 {
						countInt, err := strconv.Atoi(channelResponse["count"])
						if err != nil {
							continue
						}
						counterCeiling = countInt
					} else if infoValue == "debuggable.container" {
						debuggableContainers = append(debuggableContainers, DebuggableContainer{
							Name:  channelResponse["name"],
							Image: channelResponse["image"],
						})
						counter++
					}
				}

				if counterCeiling > 0 && counter == counterCeiling {
					break
				}
			}
			m.table = generateTable(debuggableContainers)
			close(doneCh)
		}()

		<-doneCh
		m.showDebuggableContainers = !m.showDebuggableContainers
		return m, nil
	case tea.KeyMsg:
		switch {
		case key.Matches(msg, keys.Global.Quit):
			return m, tea.Quit
		// NOTE -> We should only support this back navigation,
		// if the tui is not in standalone mode.
		case key.Matches(msg, keys.Global.Back):
			return common.TUIsInstance.Home, nil
		case key.Matches(msg, keys.Debug.LoadDebuggableContainers):
			// Kickoff loading of debuggable containers in standalone mode.
			if m.standalone {
				loadDebuggableContainers := common.Event{
					Type: common.LaunchDebugEvent,
					Data: m.gcvalues,
				}
				m, _ := m.Update(loadDebuggableContainers)
				return m, nil
			}

			// When used via `tui -> debug`
			m.showDebuggableContainers = !m.showDebuggableContainers
			return m, nil

		}
	}
	return m, nil
}

func generateTable(debuggableContainers []DebuggableContainer) table.Table {
	var rows [][]string
	for _, container := range debuggableContainers {
		rows = append(rows, []string{container.Name, container.Image})
	}
	// Note - we will start this as a lipgloss table, but once we add interaction
	// it should likely be converted to a bubble tea table.
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
		Headers("Name", "Image").
		Rows(rows...)

	return *t
}

// View returns the view that should be displayed.
func (m TUI) View() string {
	var components []string

	// What do you want to do?
	// 1. List debuggable containers
	// 2. List debug images
	// 3. List debug sessions
	// 4. Connect to a debug session
	// 5. Start a new debug session

	content := "Debug Dashboard\n"

	components = append(components, content)

	if m.showDebuggableContainers {
		header := "Debuggable Containers\n"
		components = append(components, header, m.table.String())
	}

	components = append(components, m.help())

	return lipgloss.JoinVertical(lipgloss.Left,
		components...,
	)
}

func (m TUI) help() string {
	var listOrHide string

	if m.showDebuggableContainers {
		listOrHide = "hide"
	} else {
		listOrHide = "list"
	}

	if m.standalone {
		return common.HelpStyle("• l: " + listOrHide + " debuggable containers • q: quit")
	}

	return common.HelpStyle("• l: " + listOrHide + " debuggable containers • esc: back • q: quit")
}
