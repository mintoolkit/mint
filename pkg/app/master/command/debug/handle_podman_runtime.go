package debug

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"

	"github.com/moby/term"
	terminal "golang.org/x/term"

	"github.com/containers/common/pkg/config"
	"github.com/containers/common/pkg/resize"
	"github.com/containers/podman/v5/libpod/define"
	"github.com/containers/podman/v5/pkg/bindings/containers"
	"github.com/containers/podman/v5/pkg/bindings/images"
	lsignal "github.com/containers/podman/v5/pkg/signal"
	"github.com/containers/podman/v5/pkg/specgen"
	log "github.com/sirupsen/logrus"

	"github.com/mintoolkit/mint/pkg/app"
	"github.com/mintoolkit/mint/pkg/app/master/command"
	"github.com/mintoolkit/mint/pkg/crt"
	"github.com/mintoolkit/mint/pkg/util/errutil"
	"github.com/mintoolkit/mint/pkg/util/fsutil"
	"github.com/mintoolkit/mint/pkg/util/jsonutil"
	v "github.com/mintoolkit/mint/pkg/version"
)

var trueVal = true

// HandlePodmanRuntime implements support for the Podman runtime
func HandlePodmanRuntime(
	logger *log.Entry,
	xc *app.ExecutionContext,
	gparams *command.GenericParams,
	commandParams *CommandParams,
	sid string,
	debugContainerName string) {
	var connCtx context.Context
	if gparams.CRTConnection != "" {
		connCtx = crt.GetPodmanConnContextWithConn(gparams.CRTConnection)
	} else {
		connCtx = crt.GetPodmanConnContext()
	}

	if connCtx == nil {
		xc.Out.Info("podman.connect.service",
			ovars{
				"message": "not running",
			})

		xc.Out.State("exited",
			ovars{
				"exit.code":    -1,
				"version":      v.Current(),
				"location":     fsutil.ExeDir(),
				"podman.error": crt.PodmanConnErr,
			})
		xc.Exit(-1)
	}

	if commandParams.ActionListDebuggableContainers {
		xc.Out.State("action.list_debuggable_containers")

		result, err := listPodmanDebuggableContainers(connCtx)
		if err != nil {
			logger.WithError(err).Error("listPodmanDebuggableContainers")
			xc.FailOn(err)
		}

		xc.Out.Info("debuggable.containers", ovars{"count": len(result)})
		for cname, iname := range result {
			xc.Out.Info("debuggable.container", ovars{"name": cname, "image": iname})
		}

		return
	}

	//todo: need to check that if targetRef is not empty it is valid

	if commandParams.ActionListSessions {
		xc.Out.State("action.list_sessions", ovars{"target": commandParams.TargetRef})

		//later will track/show additional debug session info
		result, err := listPodmanDebugContainers(connCtx, commandParams.TargetRef, false)
		if err != nil {
			logger.WithError(err).Error("listPodmanDebugContainers")
			xc.FailOn(err)
		}

		var waitingCount int
		var runningCount int
		var terminatedCount int
		for _, info := range result {
			switch info.State {
			case CSWaiting:
				waitingCount++
			case CSRunning:
				runningCount++
			case CSTerminated:
				terminatedCount++
			}
		}

		xc.Out.Info("debug.session.count",
			ovars{
				"total":      len(result),
				"running":    runningCount,
				"waiting":    waitingCount,
				"terminated": terminatedCount,
			})

		for name, info := range result {
			outParams := ovars{
				"target":     info.TargetContainerName,
				"name":       name,
				"image":      info.SpecImage,
				"state":      info.State,
				"start.time": info.StartTime,
			}

			/*
				if info.State == CSTerminated {
					outParams["exit.code"] = info.ExitCode
					outParams["finish.time"] = info.FinishTime
					if info.ExitReason != "" {
						outParams["exit.reason"] = info.ExitReason
					}
					if info.ExitMessage != "" {
						outParams["exit.message"] = info.ExitMessage
					}
				}
			*/

			xc.Out.Info("debug.session", outParams)
		}

		return
	}

	if commandParams.ActionShowSessionLogs {
		xc.Out.State("action.show_session_logs",
			ovars{
				"target":  commandParams.TargetRef,
				"session": commandParams.Session})

		result, err := listPodmanDebugContainers(connCtx, commandParams.TargetRef, false)
		if err != nil {
			logger.WithError(err).Error("listPodmanDebugContainers")
			xc.FailOn(err)
		}

		if len(result) < 1 {
			xc.Out.Info("no.debug.session")
			return
		}

		//todo: need to pick the last session if commandParams.Session is empty
		var containerID string
		for _, info := range result {
			if commandParams.Session == "" {
				commandParams.Session = info.Name
			}

			if commandParams.Session == info.Name {
				containerID = info.ContainerID
			}
			break
		}

		xc.Out.Info("container.logs.target", ovars{
			"container.name": commandParams.Session,
			"container.id":   containerID})

		if err := dumpPodmanContainerLogs(logger, xc, connCtx, containerID); err != nil {
			logger.WithError(err).Error("dumpPodmanContainerLogs")
		}

		return
	}

	if commandParams.ActionConnectSession {
		xc.Out.State("action.connect_session",
			ovars{
				"target":  commandParams.TargetRef,
				"session": commandParams.Session})

		result, err := listPodmanDebugContainers(connCtx, commandParams.TargetRef, true)
		if err != nil {
			logger.WithError(err).Error("listPodmanDebugContainers")
			xc.FailOn(err)
		}

		if len(result) < 1 {
			xc.Out.Info("no.debug.session")
			return
		}

		//todo: need to pick the last session if commandParams.Session is empty
		var containerID string
		for _, info := range result {
			if commandParams.Session == "" {
				commandParams.Session = info.Name
			}

			if commandParams.Session == info.Name {
				containerID = info.ContainerID
			}
			break
		}

		//todo: need to validate that the session container exists and it's running

		r, w := io.Pipe()
		go io.Copy(w, os.Stdin)

		options := &containers.AttachOptions{
			Logs:   &trueVal,
			Stream: &trueVal,
		}
		err = containers.Attach(
			connCtx,
			containerID,
			r,
			os.Stdout,
			os.Stderr,
			nil, //attachReady chan bool
			options)
		xc.FailOn(err)
		return
	}

	err := podmanEnsureImage(logger, connCtx, commandParams.DebugContainerImage)
	errutil.FailOn(err)

	clist, err := containers.List(connCtx, nil)
	if err != nil {
		//logger.WithError(err).Error("containers.List")
		xc.Out.Error("target.container.get.list", err.Error())
		xc.Out.State("exited",
			ovars{
				"exit.code": -1,
			})
		xc.Exit(-1)
	}

	var targetContainerID string
	for idx, c := range clist {
		logger.Tracef("[%d] checking container - id=%v names='%s'...", idx, c.ID, c.Names)
		for _, name := range c.Names {
			if name == commandParams.TargetRef {
				targetContainerID = c.ID
				break
			}
		}

		if targetContainerID == "" &&
			strings.Contains(c.ID, commandParams.TargetRef) {
			targetContainerID = c.ID
		}

		if targetContainerID != "" {
			if c.Exited {
				xc.Out.Error("target.container.get", "exited")
				xc.Out.State("exited",
					ovars{
						"exit.code":    -2,
						"target":       commandParams.TargetRef,
						"container.id": targetContainerID,
					})
				os.Exit(-2)
			}

			break
		}
	}

	if targetContainerID == "" {
		xc.Out.Error("target.container.get", "not.found")
		xc.Out.State("exited",
			ovars{
				"exit.code":  -3,
				"target":     commandParams.TargetRef,
				"clist.size": len(clist),
			})
		xc.Exit(-3)
	}

	inspectOpts := &containers.InspectOptions{
		Size: &trueVal,
	}
	targetContainer, err := containers.Inspect(connCtx, targetContainerID, inspectOpts)
	xc.FailOn(err)

	if commandParams.DoRunAsTargetShell {
		logger.Trace("doRunAsTargetShell")
		commandParams.Entrypoint = ShellCommandPrefix(commandParams.DebugContainerImage)
		shellConfig := configShell(sid, false)
		if CgrCustomDebugImage == commandParams.DebugContainerImage {
			shellConfig = configShellAlt(sid, false)
		}

		commandParams.Cmd = []string{shellConfig}
	} else if len(commandParams.Entrypoint) == 0 {
		if len(commandParams.Cmd) == 0 || commandParams.CmdIsShell {
			commandParams.Entrypoint = ShellCommandPrefix(commandParams.DebugContainerImage)
			if len(commandParams.Cmd) == 0 {
				commandParams.Cmd = []string{defaultShellName}
				if CgrCustomDebugImage == commandParams.DebugContainerImage {
					commandParams.Cmd = []string{bashShellName}
				}
			}
		}
		//else keep the entrypoint (and cmd) from the debug image
	}

	cntrSpec := specgen.NewSpecGenerator(commandParams.DebugContainerImage, false)
	cntrSpec.Name = debugContainerName
	cntrSpec.Entrypoint = commandParams.Entrypoint
	cntrSpec.Command = commandParams.Cmd
	cntrSpec.Terminal = &commandParams.DoTerminal
	cntrSpec.Stdin = &trueVal
	cntrSpec.Privileged = &trueVal

	// attach network, IPC & PIDs
	cntrSpec.PidNS = specgen.Namespace{
		NSMode: specgen.FromContainer,
		Value:  targetContainerID,
	}

	cntrSpec.NetNS = specgen.Namespace{
		NSMode: specgen.FromContainer,
		Value:  targetContainerID,
	}

	if targetContainer.HostConfig.IpcMode == "shareable" {
		cntrSpec.IpcNS = specgen.Namespace{
			NSMode: specgen.FromContainer,
			Value:  targetContainerID,
		}
	}

	logger.Tracef("Debugger sidecar spec: %s", jsonutil.ToString(cntrSpec))
	cntr, err := containers.CreateWithSpec(connCtx, cntrSpec, nil)
	xc.FailOn(err)
	logger.Debugf("Debugger sidecar created - (ID=%s)", cntr.ID)

	err = containers.Start(connCtx, cntr.ID, nil)
	xc.FailOn(err)
	logger.Debugf("Debugger sidecar started - (ID=%s)", cntr.ID)

	_, err = containers.Wait(connCtx, cntr.ID, &containers.WaitOptions{
		Condition: []define.ContainerStatus{define.ContainerStateRunning},
	})
	xc.FailOn(err)

	debuggerCleanup := func() {
		logger.Trace("[EXITING] Debugger container cleanup...")
		stopOpts := &containers.StopOptions{
			Ignore: &trueVal,
		}
		err = containers.Stop(connCtx, cntr.ID, stopOpts)
		if err != nil {
			logger.Debugf("error stopping container(%s): %v", cntr.ID, err)
		}

		rmOpts := &containers.RemoveOptions{
			Force:   &trueVal,
			Volumes: &trueVal,
			Ignore:  &trueVal,
		}
		_, err = containers.Remove(connCtx, cntr.ID, rmOpts)
		if err != nil {
			logger.Debugf("error removing container(%s): %v", cntr.ID, err)
			err = containers.Kill(connCtx, cntr.ID, nil)
			if err != nil {
				logger.Debugf("error killing container(%s): %v", cntr.ID, err)
			}
		}
	}

	defer debuggerCleanup()
	xc.AddCleanupHandler(func() {
		logger.Trace("xc.cleanup")
		debuggerCleanup()
	})

	if !commandParams.DoTerminal {
		dumpPodmanContainerLogs(logger, xc, connCtx, cntr.ID)
		return
	}

	if terminal.IsTerminal(int(os.Stdin.Fd())) {
		resize := make(chan resize.TerminalSize)

		cancel, oldTermState, err := podmanTerminalAttach(connCtx, resize)
		if err != nil {
			return
		}
		defer func() {
			if err := podmanTerminalRestore(oldTermState); err != nil {
				log.Errorf("podmanTerminalRestore - %v", err)
			}
		}()
		defer cancel()
	}

	r, w := io.Pipe()
	go io.Copy(w, os.Stdin)

	xc.Out.State("debug.container.running")
	xc.Out.Info("terminal.start",
		ovars{
			"note": "press enter if you don't see any output",
		})

	logger.Trace("Connecting to the debugging container...")
	aopts := &containers.AttachOptions{
		Logs:   &trueVal,
		Stream: &trueVal,
	}
	err = containers.Attach(
		connCtx,
		cntr.ID,
		r,
		os.Stdout,
		os.Stderr,
		nil, //attachReady chan bool
		aopts)
	xc.FailOn(err)

	logger.Trace("Debugger exited...")
}

func podmanTerminalAttach(ctx context.Context, resize chan resize.TerminalSize) (context.CancelFunc, *term.State, error) {
	log.Debug("podmanTerminalAttach")

	subCtx, cancel := context.WithCancel(ctx)

	resizeTty(subCtx, resize)

	oldTermState, err := term.SaveState(os.Stdin.Fd())
	if err != nil {
		// allow caller to not have to do any cleaning up if we error here
		cancel()
		return nil, nil, fmt.Errorf("unable to save terminal state: %w", err)
	}

	log.SetFormatter(&rawFormatter{})
	if _, err := term.SetRawTerminal(os.Stdin.Fd()); err != nil {
		return cancel, nil, err
	}

	return cancel, oldTermState, nil
}

func podmanTerminalRestore(state *term.State) error {
	log.SetFormatter(&log.TextFormatter{})
	return term.RestoreTerminal(os.Stdin.Fd(), state)
}

type rawFormatter struct {
	log.TextFormatter
}

func (f *rawFormatter) Format(entry *log.Entry) ([]byte, error) {
	bytes, err := f.TextFormatter.Format(entry)
	if err != nil {
		return bytes, err
	}
	return append(bytes, '\r'), nil
}

func getResize() *resize.TerminalSize {
	winsize, err := term.GetWinsize(os.Stdin.Fd())
	if err != nil {
		log.Warnf("Could not get terminal size %v", err)
		return nil
	}
	return &resize.TerminalSize{
		Width:  winsize.Width,
		Height: winsize.Height,
	}
}

func resizeTty(ctx context.Context, resize chan resize.TerminalSize) {
	sigchan := make(chan os.Signal, 1)
	signal.Notify(sigchan, lsignal.SIGWINCH)
	go func() {
		defer close(resize)
		// Update the terminal size immediately without waiting
		// for a SIGWINCH to get the correct initial size.
		resizeEvent := getResize()
		for {
			if resizeEvent == nil {
				select {
				case <-ctx.Done():
					return
				case <-sigchan:
					resizeEvent = getResize()
				}
			} else {
				select {
				case <-ctx.Done():
					return
				case <-sigchan:
					resizeEvent = getResize()
				case resize <- *resizeEvent:
					resizeEvent = nil
				}
			}
		}
	}()
}

func podmanEnsureImage(logger *log.Entry, connCtx context.Context, image string) error {
	if image == "" {
		return nil
	}

	exists, err := images.Exists(connCtx, image, nil)
	if err != nil {
		return err
	}

	if !exists {
		logger.Tracef("Pulling image '%s'...", image)

		_, err = images.Pull(connCtx, image, nil)
		if err != nil {
			return err
		}

		logger.Tracef("Pulled - '%s'.", image)
		return nil
	}

	logger.Tracef("Already have image - '%s'", image)
	return nil
}

func podmanGetDefaultConnection() (string, string) {
	podmanConfig, err := config.Default()
	if err != nil {
		log.WithError(err).Error("config.Default")
		return "", ""
	}

	connections, err := podmanConfig.GetAllConnections()
	if err != nil {
		log.WithError(err).Error("podmanConfig.GetAllConnections")
		return "", ""
	}

	log.Tracef("Connections: %s", jsonutil.ToString(connections))

	for _, c := range connections {
		if c.Default {
			return c.URI, c.Identity
		}
	}

	log.Tracef("Config.Engine: ActiveService=%s ServiceDestinations=%s",
		podmanConfig.Engine.ActiveService,
		jsonutil.ToString(podmanConfig.Engine.ServiceDestinations))

	if podmanConfig.Engine.ActiveService != "" {
		sd, found := podmanConfig.Engine.ServiceDestinations[podmanConfig.Engine.ActiveService]
		if !found {
			return "", ""
		}

		return sd.URI, sd.Identity
	}

	return "", ""
}

func listPodmanDebuggableContainers(connCtx context.Context) (map[string]string, error) {
	const op = "debug.listPodmanDebuggableContainers"
	clist, err := containers.List(connCtx, nil)
	if err != nil {
		log.WithFields(log.Fields{
			"op": op,
		}).Error("containers.List")
		return nil, err
	}

	activeContainers := map[string]string{}
	for _, info := range clist {
		names := strings.Join(info.Names, ",")
		name := names
		if len(info.Names) > 0 {
			name = info.Names[0]
		}

		if info.State != "running" {
			log.WithFields(log.Fields{
				"op":        op,
				"container": name,
				"state":     info.State,
			}).Trace("ignoring.nonrunning.container")
			continue
		}

		if strings.HasPrefix(name, containerNamePrefix) {
			log.WithFields(log.Fields{
				"op":        op,
				"container": name,
			}).Trace("ignoring.debug.container")
			continue
		}

		activeContainers[name] = info.Image
	}

	return activeContainers, nil
}

func listDebuggablePodmanContainersWithConfig(connCtx context.Context) (map[string]string, error) {
	return listPodmanDebuggableContainers(connCtx)
}

func listPodmanDebugContainersWithConfig(
	connCtx context.Context,
	targetContainer string,
	onlyActive bool) (map[string]*DebugContainerInfo, error) {
	//todo: pass the podman client config params instead of the existing client
	return listPodmanDebugContainers(connCtx, targetContainer, onlyActive)
}

// https://github.com/containers/podman/blob/main/libpod/define/containerstate.go#L42
// PCS - Podman Container State
const (
	PCSUnknown    = "unknown"
	PCSConfigured = "created"
	PCSCreated    = "initialized"
	PCSRunning    = "running"
	PCSStopped    = "stopped"
	PCSPaused     = "paused"
	PCSExited     = "exited"
	PCSRemoving   = "removing"
	PCSStopping   = "stopping"
	//also referenced in the APIs/docs:
	PCSRestarting = "restarting"
	PCSDead       = "dead"
)

func listPodmanDebugContainers(
	connCtx context.Context,
	targetContainer string,
	onlyActive bool) (map[string]*DebugContainerInfo, error) {
	const op = "debug.listPodmanDebugContainers"
	clist, err := containers.List(connCtx, nil)
	if err != nil {
		return nil, err
	}

	result := map[string]*DebugContainerInfo{}
	for _, container := range clist {
		names := strings.Join(container.Names, ",")
		name := names
		if len(container.Names) > 0 {
			name = container.Names[0]
		}

		if !strings.HasPrefix(name, containerNamePrefix) {
			log.WithFields(log.Fields{
				"op":        op,
				"container": name,
			}).Trace("ignoring.nondebug.container")
			continue
		}

		//todo: filter by targetContainer (when info.TargetContainerName is populated)
		info := &DebugContainerInfo{
			//TargetContainerName: info.TargetContainerName,
			Name:        name,
			SpecImage:   container.Image,
			ContainerID: container.ID,
			//Command:             container.Command,
			//Args:                container.Args,
			//WorkingDir:          container.WorkingDir,
			//TTY:                 container.TTY,
			StartTime: fmt.Sprintf("%v", container.Created),
		}

		switch container.State {
		case PCSConfigured, PCSCreated, PCSPaused, PCSRestarting:
			info.State = CSWaiting
		case PCSRunning:
			info.State = CSRunning
		case PCSExited, PCSRemoving, PCSStopping, PCSStopped, PCSDead:
			info.State = CSTerminated
		default:
			info.State = CSOther
		}

		if onlyActive {
			if info.State == CSRunning {
				result[info.Name] = info
			}
		} else {
			result[info.Name] = info
		}
	}

	return result, nil
}

func dumpPodmanContainerLogs(
	logger *log.Entry,
	xc *app.ExecutionContext,
	connCtx context.Context,
	containerID string) error {
	logger.Tracef("dumpPodmanContainerLogs(%s)", containerID)

	stdoutChan := make(chan string, 100)
	stderrChan := make(chan string, 100)
	options := &containers.LogOptions{
		Follow: &trueVal,
		//Details: &trueVal,
	}
	err := containers.Logs(connCtx, containerID, options, stdoutChan, stderrChan)
	if err != nil {
		logger.WithError(err).Error("error reading container logs")
		return err
	}

	xc.Out.Info("container.logs.start")
	for {
		select {
		case outData := <-stdoutChan:
			xc.Out.LogDump("debug.container.logs.stdout", string(outData))
		case errData := <-stderrChan:
			xc.Out.LogDump("debug.container.logs.stderr", string(errData))
		}
	}
	xc.Out.Info("container.logs.end")
	return nil
}
