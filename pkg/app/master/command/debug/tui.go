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
	choice  int
	runtime string

	// Handle kubernetes session connections
	subscriptionHandler subscriptionHandler
	isListening         bool
	runtimeCommunicator *RuntimeCommunicator
	exitedSession       bool
}

// Styles - move to `common`
const (
	gray      = lipgloss.Color("#737373")
	lightGray = lipgloss.Color("#d3d3d3")
	white     = lipgloss.Color("#ffffff")
)

var (
	// TitleStyle is the lipgloss style used for the view title.
	TitleStyle = lipgloss.NewStyle().Bold(true)
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

type InputKey struct {
	Rune    rune
	Special SpecialKey
}

type SpecialKey int

const (
	NotSpecial SpecialKey = iota
	Enter
	Backspace
	Up
	Down
	Left
	Right
)

type terminalStartMessage string

// subscriptionHandler struct for handling subscription data and time
type subscriptionHandler struct {
	dataChan    chan terminalStartMessage
	currentData string
}

// newSubscription creates a new subscription handler with an async data channel
func newSubscription(gcvalues *command.GenericParams, runtimeCommunicator *RuntimeCommunicator) subscriptionHandler {
	dataChan := make(chan terminalStartMessage)
	go launchSessionHandler(dataChan, gcvalues, runtimeCommunicator)
	return subscriptionHandler{
		dataChan: dataChan,
	}
}

func launchSessionHandler(dataChan chan terminalStartMessage, gcvalues *command.GenericParams, runtimeCommunicator *RuntimeCommunicator) {
	// Create a subscription channel and define subscriptionChannels map for passing data
	subscriptionChannel := make(chan interface{})
	subscriptionChannels := map[string]chan interface{}{
		"sessionData": subscriptionChannel,
	}

	// Define an execution context
	xc := app.NewExecutionContext(
		"tui",
		true,
		"subscription",
		subscriptionChannels,
	)

	// Define command parameters for docker runtime
	// + Hard coded values at the moment for this PoC
	cparams := &CommandParams{
		Runtime:                "docker",
		TargetRef:              "docker-amor",
		DebugContainerImage:    BusyboxImage,
		DoFallbackToTargetUser: true,
		DoRunAsTargetShell:     true,
		DoTerminal:             true,
		RuntimeCommunicator:    runtimeCommunicator,
		TUI:                    true,
	}

	// Connect to active session | Kubernetes session
	// cparams := &CommandParams{
	// 	Runtime:   "docker",
	// 	TargetRef: "docker-amor",
	// 	// Kubeconfig:             crt.KubeconfigDefault,
	// 	// TargetNamespace:        "default",
	// 	DebugContainerImage:    BusyboxImage,
	// 	DoFallbackToTargetUser: true,
	// 	DoRunAsTargetShell:     true,
	// 	DoTerminal:             true,
	// 	RuntimeCommunicator:               runtimeCommunicator,
	// 	TUI:                    true,
	// 	ActionConnectSession:   true,
	// }

	// TODO - Pass runtime communicator
	go OnCommand(xc, gcvalues, cparams)

	// Listen to subscription data and handle specific messages
	doneCh := make(chan struct{})
	go func() {
		for subscriptionData := range subscriptionChannel {
			channelResponse, ok := subscriptionData.(map[string]string)
			if !ok || channelResponse == nil {
				continue
			}

			log.Debugf("Channel response in tui: %v", channelResponse)

			// Handle specific states and info values
			if stateValue, exists := channelResponse["state"]; exists {
				log.Debugf("State value: %s", stateValue)
				if stateValue == "kubernetes.runtime.handler.started" {
					// Handle runtime start if needed
				} else if stateValue == "completed" {
					log.Debug("Exiting channel listening loop in update. State is complete.")
					break
				}
			}

			if infoValue, exists := channelResponse["info"]; exists {
				if infoValue == "terminal.start" {
					dataChan <- terminalStartMessage("Session ready. Opening session below...\nPress esc to exit session.\n")
					runtimeCommunicator.InputChan <- InputKey{Special: Enter}
				}
			}
		}
		close(doneCh)
	}()

	<-doneCh
	log.Debug("Exiting debug session update handler")
}

// listenToAsyncData listens to the async data channel and sends messages to the TUI
func listenToAsyncData(dataChan chan terminalStartMessage) tea.Cmd {
	return func() tea.Msg {
		return terminalStartMessage(<-dataChan)
	}
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

// We want to genericize this handler to:
// a general RuntimeCommunicationHandler.
// This handler is what will be passed to:
// Docker, Podman, Kubernetes & Containerd.
// This handler should not live on CommandParams,
// but be passed in to OnCommand, then to the respective
// runtime handler.
type RuntimeCommunicator struct {
	InputChan chan InputKey
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
		subscriptionChannel := make(chan interface{})
		// NOTE -> the names of both the channel map and the channel are misleading
		// as more than just the debuggable container information is dumped on it
		// at the moment.
		subscriptionChannels := map[string]chan interface{}{
			"debuggableContainers": subscriptionChannel,
		}
		// In addition to passing the channel(s) we will use to transport data
		// we should pass:
		// the outputs we want to subscribe to: State | Info | Error
		xc := app.NewExecutionContext(
			"tui",
			true,
			"subscription",
			subscriptionChannels,
		)

		cparams := &CommandParams{
			Runtime: m.runtime,
			// Passing these three fields  all the time does not make sense.
			ActionListDebuggableContainers: true,
			Kubeconfig:                     crt.KubeconfigDefault,
			TargetNamespace:                "default",
		}

		gcValue, ok := msg.Data.(*command.GenericParams)
		if !ok || gcValue == nil {
			return nil, nil
		}

		// TODO - Pass runtime communicator
		go OnCommand(xc, gcValue, cparams)

		counter := 0
		var counterCeiling int
		var debuggableContainers []DebuggableContainer

		doneCh := make(chan struct{})
		go func() {
			for subscriptionData := range subscriptionChannel {
				channelResponse, ok := subscriptionData.(map[string]string)
				if !ok || channelResponse == nil {
					continue
				}

				log.Debugf("Channel response in tui: %v", channelResponse)
				stateValue, stateExists := channelResponse["state"]
				if stateExists {
					log.Debugf("State value: %s", stateValue)
					if stateValue == "kubernetes.runtime.handler.started" {
						// TODO - what would we like to do with this information?
						// && we likely will want to add similar handling for the other runtimes.
					} else if stateValue == "completed" {
						// We get 'completed' then we get 'done'
						log.Debug("Exiting channel listening loop in update. State is complete.")
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
						log.Debugln("-----------------------Got debuggable container-----------------------")
						debuggableContainers = append(debuggableContainers, DebuggableContainer{
							Name:  channelResponse["name"],
							Image: channelResponse["image"],
						})
						counter++
					}
				}

				// The notion of a count[er] does not exist for the k8s
				// But it does for podman & docker.
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
		log.Debugf("tea.KeyMsg - %v", msg)
		// Send keypresses to the container if there is an active session.
		// otherwise, route them to the TUI.
		if m.isListening {
			// End session if `esc` is input -> do we want this?
			if key.Matches(msg, keys.Global.Back) {
				m.isListening = false
				m.exitedSession = true
				// Reset the subscription handler data
				m.subscriptionHandler.currentData = ""
				// Wipe the shell rendering & output
				return m, tea.ClearScreen
			}

			// Handle ctrl c
			if key.Matches(msg, keys.Global.CtrlC) {
				return m, tea.Quit
			}

			var inputKey InputKey
			switch msg.Type {
			case tea.KeyEnter:
				inputKey = InputKey{Special: Enter}
			case tea.KeyBackspace:
				inputKey = InputKey{Special: Backspace}
			case tea.KeyUp:
				inputKey = InputKey{Special: Up}
			case tea.KeyDown:
				inputKey = InputKey{Special: Down}
			case tea.KeyLeft:
				inputKey = InputKey{Special: Left}
			case tea.KeyRight:
				inputKey = InputKey{Special: Right}
			default:
				inputKey = InputKey{Rune: msg.Runes[0]} // Many gaps to cover here.
			}

			select {
			case m.runtimeCommunicator.InputChan <- inputKey:
				// Key sent successfully
			default:
				// Channel is full or closed, handle accordingly
				log.Debugf("Failed to send key to container %v", inputKey)
			}

			return m, nil
		}
		// End keypress forwarding to container.

		// Give keypress capture to ^ if there is an active session.
		switch {
		case key.Matches(msg, keys.Global.Quit):
			return m, tea.Quit
		case key.Matches(msg, keys.Global.Back):
			if m.standalone {
				return m, nil
			}
			return common.TUIsInstance.Home, nil
		case key.Matches(msg, keys.Debug.LoadDebuggableContainers):
			if m.showRuntimeSelectorView {
				return m, nil
			}
			loadDebuggableContainers := common.Event{
				Type: common.LaunchDebugEvent,
				Data: m.gcvalues,
			}
			m, _ := m.Update(loadDebuggableContainers)
			return m, nil
		case key.Matches(msg, keys.Debug.ChangeRuntime):
			m.showDebuggableContainers = false
			m.showRuntimeSelectorView = !m.showRuntimeSelectorView
			return m, nil
		case key.Matches(msg, keys.Debug.StartSession):
			if m.isListening {
				return m, nil
			}
			// TODO - extend this section to indicate to the user that the session is starting.
			// this can be done by rendering state output,
			m.isListening = true
			log.Debug("Start listening")
			runtimeCommunicator := &RuntimeCommunicator{
				InputChan: make(chan InputKey, 100),
			}
			m.runtimeCommunicator = runtimeCommunicator
			m.subscriptionHandler = newSubscription(m.gcvalues, runtimeCommunicator)
			return m, listenToAsyncData(m.subscriptionHandler.dataChan)

		}
	case terminalStartMessage:
		log.Debug("Received terminal start message")
		if m.isListening {
			m.subscriptionHandler.currentData = string(msg)
			return m, tea.ClearScreen
		}
	}
	return updateChoices(msg, m)
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
			m.showRuntimeSelectorView = false
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

	template := "Choose runtime for debug mode\n\n"
	template += "%s\n\n"

	// NOTE -> the chocies we display here should only be runtiems we can
	// establish a connection to.
	// Otherwise, we set the user up for failure.

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

// View returns the view that should be displayed.
func (m TUI) View() string {
	log.Debugf("Called Update View. Current model: %v", m)

	// What do you want to do?
	// 1. List debuggable containers
	// 2. List debug images
	// 3. List debug sessions
	// 4. Connect to a debug session
	// 5. Start a new debug session

	header := TitleStyle.Render("Debug Dashboard")
	currentRuntime := fmt.Sprintf("Current Runtime: %s.\n", m.runtime)
	var components []string
	components = append(components, header, currentRuntime)

	if m.showDebuggableContainers {
		header := "Debuggable Containers\n"
		components = append(components, header, m.table.String())
	}

	if m.showRuntimeSelectorView {
		var runtimeSelectorContent string
		runtimeSelectorContent = choicesView(m)
		components = append(components, runtimeSelectorContent)
	}

	// TODO - stop showing this message after the user performs a following keypress
	if m.exitedSession {
		components = append(components, "Session exited.")
	}
	// Indicate to the user the session is starting
	if m.subscriptionHandler.currentData != "" {
		components = append(components, m.subscriptionHandler.currentData)
	} else {
		// Hide help while in an active session
		components = append(components, m.help())
	}

	leftBar := lipgloss.JoinVertical(lipgloss.Left,
		components...,
	)

	return leftBar
}

func (m TUI) help() string {
	var debuggableContainersHelp, runtimeSelectorHelp, startSessionHelp string

	startSessionHelp = "• s: start debug session "
	if m.showRuntimeSelectorView {
		// Only display the navigation controls if the using is changing their runtime
		runtimeSelectorHelp = "cancel • j/k, up/down: select • enter: choose"
	} else {
		runtimeSelectorHelp = "change runtime"

		// Hide debuggable container help when selecting runtime
		if m.showDebuggableContainers {
			debuggableContainersHelp = "• l: hide debuggable containers"
		} else {
			debuggableContainersHelp = "• l: list debuggable containers"
		}

	}

	if m.standalone {
		return common.HelpStyle(startSessionHelp + debuggableContainersHelp + " • r: " + runtimeSelectorHelp + " • q: quit")
	}

	return common.HelpStyle(startSessionHelp + debuggableContainersHelp + " • r: " + runtimeSelectorHelp + " • esc: back • q: quit")
}
