package debug

import (
	"fmt"
	"strconv"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/lipgloss/table"
	log "github.com/sirupsen/logrus"

	"github.com/mintoolkit/mint/pkg/app"
	"github.com/mintoolkit/mint/pkg/app/master/command"
	"github.com/mintoolkit/mint/pkg/app/master/tui/common"
	"github.com/mintoolkit/mint/pkg/app/master/tui/keys"
	"github.com/mintoolkit/mint/pkg/crt"

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
	showRuntimeSelectorView  bool

	gcvalues *command.GenericParams

	// runtime selection controls
	choice int
	chosen bool

	runtime string
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
	// CheckboxStyle is the lipgloss style used for the runtime selector
	CheckboxStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("212"))
)

// End Styles - move to common - block

func LoadTUI() *TUI {
	m := &TUI{
		width:   20,
		height:  15,
		runtime: crt.AutoSelectRuntime(),
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
		runtime:    crt.AutoSelectRuntime(),
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
		log.Println("==================== UPDATE DEBUG TUI ==========================")
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
			Runtime: m.runtime,
			// Note -> we should not pass this by default, and instead pass it when a user asks.
			ActionListDebuggableContainers: true,
			// Passing this all the time does not make sense.
			Kubeconfig: crt.KubeconfigDefault,
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

				log.Printf("Channel response in tui: %v", channelResponse)
				stateValue, stateExists := channelResponse["state"]
				log.Printf("State value: %s", stateValue)
				log.Printf("State exists: %s", stateValue)
				if stateExists {
					if stateValue == "kubernetes.runtime.handler.started" {
						log.Println("Got k8s state value")
						// 		break
					} else if stateValue == "completed" {
						// we get 'completed' then we get 'done'
						log.Println("Exiting channel listening loop in update. State is complete.")
						break
					}
				}

				infoValue, infoExists := channelResponse["info"]
				if infoExists {
					if infoValue == "debuggable.containers" && counterCeiling == 0 {
						// Start docker runtime driven debuggable container handling
						// Set total debuggable container counter ceiling
						countInt, err := strconv.Atoi(channelResponse["count"])
						if err != nil {
							continue
						}
						counterCeiling = countInt
					} else if infoValue == "debuggable.container" {
						log.Println("-----------------------Got debuggable container-----------------------")
						debuggableContainers = append(debuggableContainers, DebuggableContainer{
							Name:  channelResponse["name"],
							Image: channelResponse["image"],
						})
						counter++
					}
				}

				// The notion of a count[er] does not exist for the k8s
				if counterCeiling > 0 && counter == counterCeiling {
					break
				}
				// End debuggable container handling
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
			if m.standalone {
				return m, nil
			}
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

		case key.Matches(msg, keys.Debug.ChangeRuntime):
			m.showRuntimeSelectorView = !m.showRuntimeSelectorView
			return m, nil
		}
	}
	// If the user has not made a choice, handle choice updates
	if !m.chosen {
		return updateChoices(msg, m)
	}
	// if m.chosen {

	// }
	// NEXT UP ->
	// After a user has pressed enter:
	// Reset the showRuntimeSelectorView -> we no longer want to see `cancel`
	// Actually use the new runtime (for what?)
	// Otherwise...
	// TODO - loading state after a user has selected a choice
	return m, nil
}

func updateChoices(msg tea.Msg, m TUI) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "j", "down":
			m.choice++
			if m.choice > 3 {
				m.choice = 0
			}
		case "k", "up":
			m.choice--
			if m.choice < 0 {
				m.choice = 3
			}
		case "enter":
			m.runtime = setNewRuntime(m.choice)
			m.chosen = true
			m.showRuntimeSelectorView = false
			// loadDebuggableContainers := common.Event{
			// 	Type: common.LaunchDebugEvent,
			// 	Data: m.gcvalues,
			// }
			// m, _ := m.Update(loadDebuggableContainers)
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

func choicesView(m TUI) string {
	choice := m.choice

	template := "Choose runtime for debug\n\n"
	template += "%s\n\n"
	choices := fmt.Sprintf(
		"%s\n%s\n%s\n%s",
		checkbox("Docker", choice == 0),
		checkbox("Containerd", choice == 1),
		checkbox("Podman", choice == 2),
		checkbox("Kubernetes", choice == 3),
	)
	return fmt.Sprintf(template, choices)
}

func checkbox(label string, checked bool) string {
	if checked {
		return CheckboxStyle.Render("[x] " + label)
	}
	return fmt.Sprintf("[ ] %s", label)
}

// NOTE -> the chocies we display here should only be runtiems we can
// establish a connection to.
// Otherwise, we set the user up for failure.
// const (
// 	dockerRuntime     = "docker"
// 	containerdRuntime = "containerd"
// 	podmanRuntime     = "podman"
// 	kubernetesRuntime = "k8s"
// )

func setNewRuntime(choice int) string {
	switch choice {
	case 0:
		return crt.DockerRuntime
	case 1:
		return crt.ContainerdRuntime
	case 2:
		return crt.PodmanRuntime
	case 3:
		return crt.KubernetesRuntime
	default:
		return crt.AutoRuntime
	}
}

// func chosenView(m TUI) string {

// 	return fmt.Sprintf("Loading %s runtime...", m.runtime)
// }

// View returns the view that should be displayed.
func (m TUI) View() string {
	var components []string

	// What do you want to do?
	// 1. List debuggable containers
	// 2. List debug images
	// 3. List debug sessions
	// 4. Connect to a debug session
	// 5. Start a new debug session

	header := "Debug Dashboard\n"

	currentRuntime := fmt.Sprintf("Current Runtime: %s.\n", m.runtime) // TODO - map to human readable

	components = append(components, header, currentRuntime)

	if m.showDebuggableContainers {
		header := "Debuggable Containers\n"
		components = append(components, header, m.table.String())
	}

	if m.showRuntimeSelectorView {
		var runtimeSelectorContent string
		if !m.chosen {
			runtimeSelectorContent = choicesView(m)
		}
		// else {
		// 	runtimeSelectorContent = chosenView(m)
		// }
		components = append(components, runtimeSelectorContent)
	}

	components = append(components, m.help())

	return lipgloss.JoinVertical(lipgloss.Left,
		components...,
	)
}

func (m TUI) help() string {
	var debuggableContainersHelp string

	if m.showDebuggableContainers {
		debuggableContainersHelp = "hide"
	} else {
		debuggableContainersHelp = "list"
	}

	var runtimeSelectorHelp string

	if m.showRuntimeSelectorView {
		// Only display the navigation controls if the using is changing their runtime
		runtimeSelectorHelp = "cancel • j/k, up/down: select • enter: choose"
	} else {
		runtimeSelectorHelp = "change runtime"
	}

	if m.standalone {
		return common.HelpStyle("• l: " + debuggableContainersHelp + " debuggable containers • r: " + runtimeSelectorHelp + " • q: quit")
	}

	return common.HelpStyle("• l: " + debuggableContainersHelp + " debuggable containers • r: " + runtimeSelectorHelp + " • j/k, up/down: select • enter: choose • esc: back • q: quit")
}
