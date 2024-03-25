package debug

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/user"
	"strings"
	"sync"

	//"time"
	"context"
	"os/signal"
	"regexp"
	"runtime"
	"syscall"

	"github.com/containerd/console"
	"github.com/containerd/containerd"
	tasksv1 "github.com/containerd/containerd/api/services/tasks/v1"
	"github.com/containerd/containerd/api/types/task"
	"github.com/containerd/containerd/cio"
	"github.com/containerd/containerd/cmd/ctr/commands"
	"github.com/containerd/containerd/cmd/ctr/commands/tasks"
	"github.com/containerd/containerd/namespaces"
	"github.com/containerd/containerd/oci"
	"github.com/moby/term"
	"github.com/opencontainers/runtime-spec/specs-go"
	log "github.com/sirupsen/logrus"

	"github.com/mintoolkit/mint/pkg/app"
	"github.com/mintoolkit/mint/pkg/app/master/command"
	"github.com/mintoolkit/mint/pkg/util/fsutil"
	"github.com/mintoolkit/mint/pkg/util/jsonutil"
	v "github.com/mintoolkit/mint/pkg/version"
)

const cdSocket = "/run/containerd/containerd.sock"

func hasContainerDSocket() bool {
	return hasSocket(cdSocket)
}

// HandleContainerdRuntime implements support for the ContainerD runtime
func HandleContainerdRuntime(
	logger *log.Entry,
	xc *app.ExecutionContext,
	gparams *command.GenericParams,
	commandParams *CommandParams,
	sid string,
	debugContainerName string) {
	logger = logger.WithFields(
		log.Fields{
			"op": "debug.HandleContainerdRuntime",
		})

	logger.WithField("cparams", jsonutil.ToString(commandParams)).Trace("call")
	defer logger.Trace("exit")

	if runtime.GOOS != "linux" {
		xc.Out.Info("unsupported.runtime.os",
			ovars{
				"os":      runtime.GOOS,
				"runtime": "containerd",
			})

		xc.Out.State("exited",
			ovars{
				"exit.code": 0,
				"version":   v.Current(),
				"location":  fsutil.ExeDir(),
			})
		xc.Exit(0)
	}

	if !hasContainerDSocket() {
		xc.Out.Error("runtime.containerd", "socket.not.found")
		xc.Out.State("exited",
			ovars{
				"exit.code": -1,
				"version":   v.Current(),
				"location":  fsutil.ExeDir(),
			})
		xc.Exit(-1)
	}

	ctx := context.Background()
	api, err := containerd.New(cdSocket)
	if err != nil {
		if errors.Is(err, os.ErrPermission) {
			username := "unknown"
			usr, uerr := user.Current()
			if uerr == nil {
				username = usr.Username
			}

			xc.Out.Error("runtime.containerd", "socket.permission.error")
			xc.Out.State("exited",
				ovars{
					"exit.code": -1,
					"version":   v.Current(),
					"location":  fsutil.ExeDir(),
					"uid":       os.Geteuid(),
					"user":      username,
					"message":   "make sure to run as user with access to the containerd unix socket",
				})
			xc.Exit(-1)
		}

		log.WithError(err).Error("containerd.Newx")
		xc.FailOn(err)
	}
	defer api.Close()

	if commandParams.ActionListNamespaces {
		xc.Out.State("action.list_namespaces")
		names, err := cdListNamespacesWithParams(ctx, api)
		if err != nil {
			logger.WithError(err).Error("listNamespaces")
			xc.FailOn(err)
		}

		for _, name := range names {
			xc.Out.Info("namespace", ovars{"name": name})
		}

		return
	}

	nsName, err := cdEnsureNamespaceWithParams(ctx, api, commandParams.TargetNamespace)
	if err != nil {
		logger.WithError(err).Error("ensureNamespace")
		xc.FailOn(err)
	}

	ctx = namespaces.WithNamespace(ctx, nsName)

	if commandParams.ActionListDebuggableContainers {
		xc.Out.State("action.list_debuggable_containers", ovars{"namespace": nsName})

		containers, err := cdListDebuggableContainers(ctx, api)
		if err != nil {
			logger.WithError(err).Error("listDebuggableContainers")
			xc.FailOn(err)
		}

		xc.Out.Info("debuggable.containers", ovars{"count": len(containers)})
		for _, c := range containers {
			xc.Out.Info("debuggable.container", ovars{"name": c.Name, "image": c.Image})
		}

		return
	}

	//ALSO TODO: support for all session related actions

	logger.WithField("target", commandParams.TargetRef).Debug("locating container")

	filters := []string{
		fmt.Sprintf("labels.%q==%s", "name", commandParams.TargetRef),
		fmt.Sprintf("labels.%q==%s", "nerdctl/name", commandParams.TargetRef),
		fmt.Sprintf("id~=^%s.*$", regexp.QuoteMeta(commandParams.TargetRef)),
	}

	clist, err := api.Containers(ctx, filters...)
	if err != nil {
		logger.WithError(err).Error("api.Containers")
		xc.FailOn(err)
	}

	containerFound := false
	targetContainerIndex := -1
	targetContainerIsRunning := false
	var targetContainer containerd.Container
	for idx, c := range clist {
		cname := cdContainerName(ctx, c)
		if cname != commandParams.TargetRef {
			continue
		}

		task, err := c.Task(ctx, nil)
		if err != nil {
			logger.WithError(err).Error("c.Task")
			xc.FailOn(err)
		}

		status, err := task.Status(ctx)
		if err != nil {
			logger.WithError(err).Error("task.Status")
			xc.FailOn(err)
		}

		if status.Status == containerd.Running {
			targetContainerIndex = idx
			targetContainer = c
			containerFound = true
			targetContainerIsRunning = true

			logger.WithFields(
				log.Fields{
					"index":  targetContainerIndex,
					"ns":     nsName,
					"target": commandParams.TargetRef,
				}).Trace("found container (running)")
		} else {
			logger.WithFields(
				log.Fields{
					"index":  targetContainerIndex,
					"ns":     nsName,
					"target": commandParams.TargetRef,
					"id":     c.ID(),
				}).Trace("found container (not running)")
		}

		break
	}

	if targetContainer != nil {
		logger.WithField("data", fmt.Sprintf("id=%s", targetContainer.ID())).Trace("target container info")
	}

	if !containerFound {
		logger.Errorf("Container %s not found in namespace %s", commandParams.TargetRef, nsName)
		xc.FailOn(fmt.Errorf("target container not found"))
	}

	if !targetContainerIsRunning {
		xc.Out.Info("wait.for.target.container",
			ovars{
				"name":      commandParams.TargetRef,
				"namespace": nsName,
			})

		//TODO: support waiting for pending/starting containers
	}

	doTTY := commandParams.DoTerminal && term.IsTerminal(os.Stdin.Fd())
	logger.WithFields(
		log.Fields{
			"do.tty":        doTTY,
			"do.term":       commandParams.DoTerminal,
			"stdin.is.term": term.IsTerminal(os.Stdin.Fd()),
		}).Trace("doTTY")

	if commandParams.DoRunAsTargetShell {
		logger.Trace("doRunAsTargetShell")
		commandParams.Entrypoint = ShellCommandPrefix(commandParams.DebugContainerImage)
		shellConfig := configShell(sid, true)
		if CgrCustomDebugImage == commandParams.DebugContainerImage {
			shellConfig = configShellAlt(sid, false)
		}

		commandParams.Cmd = []string{shellConfig}
	} else {
		commandParams.Entrypoint = ShellCommandPrefix(commandParams.DebugContainerImage)
		if len(commandParams.Cmd) == 0 {
			commandParams.Cmd = []string{defaultShellName}
			if CgrCustomDebugImage == commandParams.DebugContainerImage {
				commandParams.Cmd = []string{bashShellName}
			}
		}
	}

	logger.WithFields(
		log.Fields{
			"work.dir": commandParams.Workdir,
			"params":   jsonutil.ToString(commandParams),
		}).Trace("newDebuggingContainerInfo")

	//TODO: expand the image path for short docker image paths
	if !strings.Contains(commandParams.DebugContainerImage, "/") {
		//a hacky way to ensure full paths for containerd :)
		commandParams.DebugContainerImage = fmt.Sprintf("docker.io/library/%s", commandParams.DebugContainerImage)
	}

	//TODO: pull even if local images exists if the 'pull' flag is specified explicitly
	var debugImage containerd.Image
	imgInfo, err := api.ImageService().Get(ctx, commandParams.DebugContainerImage)
	if err != nil {
		logger.WithError(err).Trace("api.ImageService.Get")
		logger.Debugf("api.Pull(%s)", commandParams.DebugContainerImage)
		debugImage, err = api.Pull(ctx, commandParams.DebugContainerImage, containerd.WithPullUnpack)
		if err != nil {
			logger.WithError(err).Error("api.Pull")
			xc.FailOn(err)
		}
	} else {
		debugImage = containerd.NewImage(api, imgInfo)
	}

	targetTask, err := targetContainer.Task(ctx, nil)
	if err != nil {
		logger.WithError(err).Error("targetContainer.Task")
		xc.FailOn(err)
	}

	targetTaskPID := int(targetTask.Pid())
	logger.Debugf("Target task PID: %d", targetTaskPID)

	targetSpec, err := targetContainer.Spec(ctx)
	if err != nil {
		logger.WithError(err).Error("targetContainer.Spec")
		xc.FailOn(err)
	}

	var pidNS string
	var netNS string
	var ipcNS string
	var utsNS string
	for _, ns := range targetSpec.Linux.Namespaces {
		switch ns.Type {
		case specs.PIDNamespace:
			pidNS = fmt.Sprintf("/proc/%d/ns/pid", targetTaskPID)
		case specs.NetworkNamespace:
			netNS = fmt.Sprintf("/proc/%d/ns/net", targetTaskPID)
		case specs.IPCNamespace:
			ipcNS = fmt.Sprintf("/proc/%d/ns/ipc", targetTaskPID)
		case specs.UTSNamespace:
			utsNS = fmt.Sprintf("/proc/%d/ns/uts", targetTaskPID)
		}
	}

	logger.Debugf("pidNS=%s netNS=%s ipcNS=%s utsNS=%s", pidNS, netNS, ipcNS, utsNS)

	specOpts := []oci.SpecOpts{
		oci.WithImageConfig(debugImage),
		oci.WithProcessArgs(
			append(commandParams.Entrypoint, commandParams.Cmd...)...,
		),
	}

	if commandParams.Workdir != "" {
		specOpts = append(specOpts, oci.WithProcessCwd(commandParams.Workdir))
	}

	if len(commandParams.EnvVars) > 0 {
		var evList []string
		for _, pair := range commandParams.EnvVars {
			evList = append(evList, fmt.Sprintf("%s=%s", pair.Name, pair.Value))
		}

		specOpts = append(specOpts, oci.WithEnv(evList))
	}

	if doTTY {
		specOpts = append(specOpts,
			func() oci.SpecOpts {
				return oci.WithTTY
			}())
	}

	if pidNS != "" {
		specOpts = append(specOpts,
			oci.WithLinuxNamespace(specs.LinuxNamespace{
				Type: specs.PIDNamespace,
				Path: pidNS,
			}))
	}

	if netNS != "" {
		specOpts = append(specOpts,
			oci.WithLinuxNamespace(specs.LinuxNamespace{
				Type: specs.NetworkNamespace,
				Path: netNS,
			}))
	}

	if ipcNS != "" {
		specOpts = append(specOpts,
			oci.WithLinuxNamespace(specs.LinuxNamespace{
				Type: specs.IPCNamespace,
				Path: ipcNS,
			}))
	}

	if utsNS != "" {
		specOpts = append(specOpts,
			oci.WithLinuxNamespace(specs.LinuxNamespace{
				Type: specs.UTSNamespace,
				Path: utsNS,
			}))
	}

	debugContainer, err := api.NewContainer(
		ctx,
		debugContainerName,
		containerd.WithNewSnapshot(
			fmt.Sprintf("%s-snapshot", debugContainerName), debugImage),
		containerd.WithNewSpec(oci.Compose(specOpts...)),
	)

	if err != nil {
		logger.WithError(err).Error("api.NewContainer")
		xc.FailOn(err)
	}

	defer debugContainer.Delete(ctx, containerd.WithSnapshotCleanup)

	xc.AddCleanupHandler(func() {
		logger.Trace("xc.cleanup")
		if err := debugContainer.Delete(ctx, containerd.WithSnapshotCleanup); err != nil {
			logger.Debugf("failed to delete container: %v", err)
		}
	})

	ioc, con, err := prepareTaskIO(ctx, logger, doTTY, true, debugContainer)
	if err != nil {
		logger.WithError(err).Error("prepareTaskIO")
		xc.FailOn(err)
	}
	if con != nil {
		defer con.Reset()

		xc.AddCleanupHandler(func() {
			logger.Trace("xc.cleanup")
			con.Reset()
		})
	}

	task, err := debugContainer.NewTask(ctx, ioc)
	if err != nil {
		logger.WithError(err).Error("debugContainer.NewTask")
		xc.FailOn(err)
	}
	defer task.Delete(ctx)

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT)

	taskExitStatusCh, err := task.Wait(ctx)
	if err != nil {
		logger.WithError(err).Error("task.Wait")
		xc.FailOn(err)
	}

	if err := task.Start(ctx); err != nil {
		logger.WithError(err).Error("task.Start")
		xc.FailOn(err)
	}

	logger.Debugf("Debugger is attached to target - %s", commandParams.TargetRef)

	xc.Out.State("debug.container.running")
	xc.Out.Info("terminal.start",
		ovars{
			"note": "press enter if you don't see any output",
		})

	fmt.Printf("\n")

	if doTTY {
		if err := tasks.HandleConsoleResize(ctx, task, con); err != nil {
			log.WithError(err).Error("console resize")
		}
	} else {
		sigc := commands.ForwardAllSignals(ctx, task)
		defer commands.StopCatch(sigc)
	}

	var exitStatus containerd.ExitStatus
	select {
	case <-sigCh:
		logger.Debugf("Exiting session [signal]: Control+C (killing debugging task)...")
		if err := task.Kill(ctx, syscall.SIGKILL); err != nil {
			logger.Debugf("failed to kill task: %v", err)
		}

		logger.Debug("Getting task exit status...")
		exitStatus = <-taskExitStatusCh
		logger.Debug("Got task exit status...")
		//Container exited with status: 137 (137)
	case exitStatus = <-taskExitStatusCh:
		logger.Debug("Exiting session [task exited]...")
		//When contrainer is stopped by an external tool: Container exited with status: 143 (143)
		//When 'exit' cli command: Container exited with status: 0 (0)
	case <-ctx.Done():
		logger.Debug("Exiting session [ctx done]...")
	}

	if exitStatus.Error() != nil {
		logger.Debugf("waiting debugger container failed: %v", err)
	} else {
		code, _, err := exitStatus.Result()
		if err != nil {
			logger.Debugf("error getting exit status code: %v", err)
		}

		logger.Debugf("Container exited with status: %d (%d)",
			exitStatus.ExitCode(), code)
	}

	logger.Debug("Cleaning up resources...")

	if _, err := task.Delete(ctx); err != nil {
		logger.Debugf("failed to delete task: %v", err)
	}

	if err := debugContainer.Delete(ctx, containerd.WithSnapshotCleanup); err != nil {
		logger.Debugf("failed to delete container: %v", err)
	}
}

func cdListNamespaces() ([]string, error) {
	api, err := containerd.New(cdSocket)
	if err != nil {
		log.WithError(err).Error("containerd.New")
		return nil, err
	}
	defer api.Close()

	ctx := context.Background()
	names, err := cdListNamespacesWithParams(ctx, api)
	if err != nil {
		log.WithError(err).Error("cdListNamespacesWithParams")
		return nil, err
	}
	return names, nil
}

func cdListNamespacesWithParams(ctx context.Context, client *containerd.Client) ([]string, error) {
	names, err := client.NamespaceService().List(ctx)
	if err != nil {
		return nil, err
	}

	return names, nil
}

func cdEnsureNamespaceWithParams(ctx context.Context, client *containerd.Client, name string) (string, error) {
	nsList, err := cdListNamespacesWithParams(ctx, client)
	if err != nil {
		return "", err
	}

	for _, val := range nsList {
		if name == "" {
			return val, nil
		}

		if val == name {
			return name, nil
		}
	}

	return "", fmt.Errorf("no namespaces")
}

type cdContainerInfo struct {
	Name  string
	Image string
}

func cdListDebuggableContainers(ctx context.Context, api *containerd.Client) ([]cdContainerInfo, error) {
	var err error
	if api == nil {
		api, err = containerd.New(cdSocket)
		if err != nil {
			return nil, fmt.Errorf("failed to create containerd client: %w", err)
		}
		defer api.Close()
	}

	if _, exists := namespaces.Namespace(ctx); !exists {
		ctx = namespaces.NamespaceFromEnv(ctx)
	}

	allTasks, err := api.TaskService().List(ctx, &tasksv1.ListTasksRequest{})
	if err != nil {
		return nil, err
	}
	runningTasks := map[string]*task.Process{}
	for _, t := range allTasks.Tasks {
		if t.GetStatus() == task.Status_RUNNING {
			runningTasks[t.ID] = t // t.ContainerID seems to be always empty but t.ID is usually the container ID
		}
	}

	allContainers, err := api.Containers(ctx)
	if err != nil {
		return nil, err
	}

	var debuggableContainers []cdContainerInfo
	for _, c := range allContainers {
		if strings.HasPrefix(c.ID(), containerNamePrefix) {
			continue
		}

		if _, found := runningTasks[c.ID()]; !found {
			continue
		}

		debuggableContainers = append(debuggableContainers, cdContainerInfo{
			Name:  cdContainerName(ctx, c),
			Image: cdContainerImage(ctx, c),
		})
	}

	return debuggableContainers, nil
}

func cdContainerName(ctx context.Context, cont containerd.Container) string {
	labels, err := cont.Labels(ctx)
	if err != nil {
		log.WithError(err).Warnf("container=%s: error getting labels", cont.ID())
	}

	cname, found := labels["io.containerd.container.name"]
	if !found {
		cname, found = labels["nerdctl/name"]
	}
	if !found {
		cname, found = labels["name"]
	}

	if cname == "" {
		cname = cont.ID()
	}

	return cname
}

func cdContainerImage(ctx context.Context, cont containerd.Container) string {
	img, err := cont.Image(ctx)
	if err != nil {
		log.WithError(err).Warnf("container=%s: error getting image", cont.ID())
		return "<unknown>"
	}

	return img.Name()
}

func prepareTaskIO(
	ctx context.Context,
	logger *log.Entry,
	tty bool,
	stdin bool,
	cont containerd.Container,
) (cio.Creator, console.Console, error) {
	if tty {
		logger.Trace("prepareTaskIO.with.tty")
		con := console.Current()
		if err := con.SetRaw(); err != nil {
			return nil, nil, err
		}

		var in io.Reader
		if stdin {
			if con == nil {
				return nil, nil, errors.New("input must be a terminal")
			}
			in = con
		}

		return cio.NewCreator(cio.WithStreams(in, con, nil), cio.WithTerminal), con, nil
	}

	logger.Trace("prepareTaskIO.no.tty")
	var in io.Reader
	if stdin {
		logger.Trace("prepareTaskIO.no.tty.with.stdin")
		in = &inCloser{
			inputStream: os.Stdin,
			close: func() {
				if task, err := cont.Task(ctx, nil); err != nil {
					logger.Debugf("Failed to get task for stdinCloser: %s", err)
				} else {
					task.CloseIO(ctx, containerd.WithStdinCloser)
				}
			},
		}
	}

	return cio.NewCreator(cio.WithStreams(
		in,
		os.Stdout,
		os.Stderr,
	)), nil, nil
}

type inCloser struct {
	inputStream io.Reader
	close       func()

	mu     sync.Mutex
	closed bool
}

func (s *inCloser) Read(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return 0, syscall.EBADF
	}

	n, err := s.inputStream.Read(p)
	if err != nil {
		if s.close != nil {
			s.close()
			s.closed = true
		}
	}

	return n, err
}

func (s *inCloser) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return nil
	}

	if s.close != nil {
		s.close()
	}
	s.closed = true
	return nil
}
