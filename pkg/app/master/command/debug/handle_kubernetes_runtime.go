package debug

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	dockerapi "github.com/fsouza/go-dockerclient"
	log "github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	restclient "k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/remotecommand"

	"github.com/mintoolkit/mint/pkg/app"
	"github.com/mintoolkit/mint/pkg/app/master/command"
	"github.com/mintoolkit/mint/pkg/app/master/inspectors/image"
	"github.com/mintoolkit/mint/pkg/crt/docker/dockercrtclient"
	"github.com/mintoolkit/mint/pkg/util/jsonutil"
)

// HandleKubernetesRuntime implements support for the k8s runtime
func HandleKubernetesRuntime(
	logger *log.Entry,
	xc *app.ExecutionContext,
	gparams *command.GenericParams,
	commandParams *CommandParams,
	dockerClient *dockerapi.Client,
	sid string,
	debugContainerName string) {
	logger = logger.WithFields(
		log.Fields{
			"op": "debug.HandleKubernetesRuntime",
		})
	xc.Out.State("kubernetes.runtime.handler.started")

	cpJson, _ := json.Marshal(commandParams)
	logger.WithField("cparams", string(cpJson)).Trace("call")
	defer logger.Trace("exit")

	ctx := context.Background()
	crtClient := dockercrtclient.New(dockerClient)

	api, restConfig, err := apiClientFromConfig(commandParams.Kubeconfig)
	if err != nil {
		logger.WithError(err).Error("apiClientFromConfig")
		xc.FailOn(err)
	}

	if commandParams.ActionListNamespaces {
		xc.Out.State("action.list_namespaces")
		names, err := listNamespaces(ctx, api)
		if err != nil {
			logger.WithError(err).Error("listNamespaces")
			xc.FailOn(err)
		}

		for _, name := range names {
			xc.Out.Info("namespace", ovars{"name": name})
		}

		return
	}

	nsName, err := ensureNamespace(ctx, api, commandParams.TargetNamespace)
	if err != nil {
		logger.WithError(err).Error("ensureNamespace")
		xc.FailOn(err)
	}

	if commandParams.ActionListPods {
		xc.Out.State("action.list_pods", ovars{"namespace": nsName})
		names, err := listActivePods(ctx, api, nsName)
		if err != nil {
			logger.WithError(err).Error("listActivePods")
			xc.FailOn(err)
		}

		for _, name := range names {
			xc.Out.Info("pod", ovars{"name": name})
		}

		return
	}

	pod, podName, err := ensurePod(ctx, api, nsName, commandParams.TargetPod)
	if apierrors.IsNotFound(err) {
		logger.WithError(err).
			WithFields(log.Fields{
				"ns":  nsName,
				"pod": podName,
			}).Error("ensurePod - not found")
		xc.FailOn(err)
	} else if statusError, isStatus := err.(*apierrors.StatusError); isStatus {
		logger.WithError(err).
			WithFields(log.Fields{
				"ns":     nsName,
				"pod":    podName,
				"status": statusError.ErrStatus.Message,
			}).Error("ensurePod - status error")
		xc.FailOn(err)
	} else if err != nil {
		logger.WithError(err).
			WithFields(log.Fields{
				"ns":     nsName,
				"pod":    podName,
				"status": statusError.ErrStatus.Message,
			}).Error("ensurePod - other error")
		xc.FailOn(err)
	}

	logger.WithField("phase", pod.Status.Phase).Debug("target pod status")

	if pod.Status.Phase != corev1.PodRunning {
		logger.Error("target pod is not running")
		xc.FailOn(fmt.Errorf("target pod is not running"))
	}

	logger.WithFields(
		log.Fields{
			"ns":       nsName,
			"pod":      podName,
			"ec.count": len(pod.Spec.EphemeralContainers),
		}).Debug("target pod info")

	if commandParams.ActionListDebuggableContainers {
		xc.Out.State("action.list_debuggable_containers",
			ovars{"namespace": nsName, "pod": podName})
		result, err := listK8sDebuggableContainers(ctx, api, nsName, podName)
		if err != nil {
			logger.WithError(err).Error("listK8sDebuggableContainers")
			xc.FailOn(err)
		}

		for cname, iname := range result {
			xc.Out.Info("debuggable.container", ovars{"name": cname, "image": iname})
		}

		return
	}

	//todo: need to check that if targetRef is not empty it is valid

	if commandParams.ActionListSessions {
		//list sessions before we pick a target container,
		//so we can list all debug session for the selected pod
		xc.Out.State("action.list_sessions",
			ovars{
				"namespace": nsName,
				"pod":       podName,
				"target":    commandParams.TargetRef})

		//later will track/show additional debug session info
		result, err := listK8sDebugContainers(ctx, api, nsName, podName, commandParams.TargetRef, false)
		if err != nil {
			logger.WithError(err).Error("listK8sDebugContainers")
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

			xc.Out.Info("debug.session", outParams)
		}

		return
	}

	if commandParams.TargetRef == "" {
		logger.Debug("no explicit target container... pick one")
		//TODO: improve this logic (to also check for the default container)
		if len(pod.Spec.Containers) > 0 {
			commandParams.TargetRef = pod.Spec.Containers[0].Name
		} else {
			xc.FailOn(fmt.Errorf("no containers"))
		}
	}

	if commandParams.ActionShowSessionLogs {
		//list sessions before we pick a target container,
		//so we can list all debug session for the selected pod
		xc.Out.State("action.show_session_logs",
			ovars{
				"namespace": nsName,
				"pod":       podName,
				"target":    commandParams.TargetRef,
				"session":   commandParams.Session})

		if commandParams.Session == "" {
			result, err := listK8sDebugContainers(ctx, api, nsName, podName, commandParams.TargetRef, false)
			if err != nil {
				logger.WithError(err).Error("listK8sDebugContainers")
				xc.FailOn(err)
			}

			if len(result) < 1 {
				xc.Out.Info("no.debug.session")
				return
			}

			//todo: need to pick the last session
			for _, info := range result {
				commandParams.Session = info.Name
				break
			}
		}

		if err := dumpK8sContainerLogs(logger, xc, ctx, api, nsName, podName, commandParams.Session); err != nil {
			logger.WithError(err).Error("dumpK8sContainerLogs")
		}

		return
	}

	if commandParams.ActionConnectSession {
		xc.Out.State("action.connect_session",
			ovars{
				"namespace": nsName,
				"pod":       podName,
				"target":    commandParams.TargetRef,
				"session":   commandParams.Session})

		if commandParams.Session == "" {
			result, err := listK8sDebugContainers(ctx, api, nsName, podName, commandParams.TargetRef, true)
			if err != nil {
				logger.WithError(err).Error("listK8sDebugContainers")
				xc.FailOn(err)
			}

			if len(result) < 1 {
				xc.Out.Info("no.debug.session")
				return
			}

			//todo: need to pick the last session
			for _, info := range result {
				commandParams.Session = info.Name
				break
			}
		}

		//todo: need to validate that the debug session container exists and it's running

		//note: tty should be controlled by the 'terminal' flag
		//and connecting would not be interactive if it's not true
		doTTY := true

		req := api.CoreV1().RESTClient().Post().
			Resource("pods").
			Name(podName).
			Namespace(nsName).
			SubResource("attach").
			VersionedParams(&corev1.PodAttachOptions{
				Container: commandParams.Session,
				Stdin:     true,
				Stdout:    true,
				Stderr:    true,
				TTY:       doTTY,
			}, scheme.ParameterCodec)

		logger.Tracef("(connect to session) pod attach request URL: %s", req.URL())

		attach, err := remotecommand.NewSPDYExecutor(restConfig, http.MethodPost, req.URL())
		if err != nil {
			logger.WithError(err).Error("remotecommand.NewSPDYExecutor")
			xc.FailOn(err)
		}

		xc.Out.Info("terminal.start",
			ovars{
				"mode": "connecting to existing debug session",
				"note": "press enter if you dont see any output",
			})

		logger.Trace("starting stream...")
		//TODO:
		//use commandParams.DoTerminal to conditionally enable the interactive terminal
		//if false configure stream to do a one off command execution
		//and dump the container logs
		//similar to how it's done with the docker runtime

		fmt.Printf("\n")
		//note: blocks until done streaming or failure...
		err = attach.StreamWithContext(
			ctx,
			remotecommand.StreamOptions{
				Stdin:  os.Stdin,
				Stdout: os.Stdout,
				Stderr: os.Stderr,
				Tty:    doTTY,
			})

		if err != nil {
			if apierrors.IsNotFound(err) {
				logger.WithError(err).
					Error("attach.StreamWithContext - not found")
			} else if statusError, isStatus := err.(*apierrors.StatusError); isStatus {
				logger.WithError(err).
					WithFields(log.Fields{
						"status": statusError.ErrStatus.Message,
					}).Error("attach.StreamWithContext - status error")
			} else {
				logger.WithError(err).
					Error("attach.StreamWithContext - other error")
			}

			xc.FailOn(err)
		}

		return
	}

	logger.WithField("target", commandParams.TargetRef).Debug("locating container")

	targetContainerIndex := -1
	targetContainerIsRunning := false
	var targetContainer *corev1.Container
	for i, c := range pod.Spec.Containers {
		if c.Name == commandParams.TargetRef {
			targetContainerIndex = i
			targetContainer = &c

			logger.WithFields(
				log.Fields{
					"index":  targetContainerIndex,
					"ns":     nsName,
					"pod":    podName,
					"target": commandParams.TargetRef,
				}).Trace("found container")
			break
		}
	}

	if targetContainer != nil {
		//doTTY = targetContainer.TTY
		logger.WithField("data", fmt.Sprintf("%#v", targetContainer)).Trace("target container info")
	}

	containerFound := false
	for _, containerStatus := range pod.Status.ContainerStatuses {
		if containerStatus.Name == commandParams.TargetRef {
			containerFound = true
			if containerStatus.State.Running != nil {
				targetContainerIsRunning = true
				logger.Trace("target container is running")
			}
			break
		}
	}

	if !containerFound {
		logger.Errorf("Container %s not found in pod %s", commandParams.TargetRef, podName)
		xc.FailOn(fmt.Errorf("target container not found"))
	}

	if !targetContainerIsRunning {
		xc.Out.Info("wait.for.target.container",
			ovars{
				"name":      commandParams.TargetRef,
				"pod":       podName,
				"namespace": nsName,
			})

		err = waitForContainer(logger, xc, ctx, api, nsName, podName, commandParams.TargetRef, ctStandard)
		if err != nil {
			logger.WithError(err).Error("waitForContainer")
			xc.FailOn(err)
		}
	}

	//'tty' config needs to be the same when creating & attaching
	doTTY := true
	isEcPrivileged := commandParams.DoRunPrivileged //true

	roRootFilesystem := func() bool {
		sc := targetContainer.SecurityContext
		if sc != nil && sc.ReadOnlyRootFilesystem != nil {
			return *sc.ReadOnlyRootFilesystem
		}

		return false
	}

	podRunAsNonRoot := func() bool {
		psc := pod.Spec.SecurityContext
		if psc != nil &&
			psc.RunAsNonRoot != nil &&
			*psc.RunAsNonRoot {
			return true
		}

		return false
	}

	runAsNonRoot := func() bool {
		sc := targetContainer.SecurityContext
		if sc != nil &&
			sc.RunAsNonRoot != nil &&
			*sc.RunAsNonRoot {
			return true
		}

		return podRunAsNonRoot()
	}

	runAsUser := func(lookup bool) *int64 {
		if !lookup {
			return nil
		}

		sc := targetContainer.SecurityContext
		if sc != nil && sc.RunAsUser != nil {
			return sc.RunAsUser
		}

		psc := pod.Spec.SecurityContext
		if psc != nil && psc.RunAsUser != nil {
			return psc.RunAsUser
		}

		return nil
	}

	runAsGroup := func(lookup bool) *int64 {
		if !lookup {
			return nil
		}

		sc := targetContainer.SecurityContext
		if sc != nil && sc.RunAsGroup != nil {
			return sc.RunAsGroup
		}

		psc := pod.Spec.SecurityContext
		if psc != nil && psc.RunAsGroup != nil {
			return psc.RunAsGroup
		}

		return nil
	}

	targetIsNonRoot := runAsNonRoot()

	runAsGroupVal := runAsGroup(targetIsNonRoot)
	if targetIsNonRoot && runAsGroupVal == nil && commandParams.UID < 0 {
		//don't use the default group if UID is set
		runAsGroupVal = &defaultNonRootGroup
	}

	runAsUserVal := runAsUser(targetIsNonRoot)
	if targetIsNonRoot && runAsUserVal == nil {
		//first, try getting the user identity from the target's container image
		var userFromImage string
		if targetContainer.Image != "" && dockerClient != nil {
			//TODO: improve
			//v1 version is very hacky:
			//* it expects the Docker container runtime locally
			//* it expects the target container image to be available locally
			//* it expects the target container images to be pullable (with no auth)
			imageInspector, err := image.NewInspector(crtClient, targetContainer.Image)
			if err == nil {
				noImage, err := imageInspector.NoImage()
				if err == nil {
					var foundImage bool
					if noImage {
						if err := imageInspector.Pull(true, "", "", ""); err != nil {
							logger.WithError(err).Trace("imageInspector.Pull")
						}

						imageInspector, err = image.NewInspector(crtClient, targetContainer.Image)
						if err == nil {
							noImage, err = imageInspector.NoImage()
							if err == nil {
								if !noImage {
									foundImage = true
								}
							} else {
								logger.WithError(err).Trace("imageInspector.NoImage")
							}
						} else {
							logger.WithError(err).Trace("image.NewInspector")
						}
					} else {
						foundImage = true
					}

					if foundImage {
						if err := imageInspector.Inspect(); err == nil {
							userFromImage = imageInspector.ImageInfo.Config.User
						} else {
							logger.WithError(err).Trace("imageInspector.Inspect")
						}
					}

				} else {
					logger.WithError(err).Trace("imageInspector.NoImage")
				}
			} else {
				logger.WithError(err).Trace("image.NewInspector")
			}
		}

		uid := int64(-1)
		gid := int64(-1)
		if userFromImage != "" {
			var uidStr string
			var gidStr string
			if strings.Contains(userFromImage, ":") {
				parts := strings.SplitN(userFromImage, ":", 2)
				uidStr = parts[0]
				gidStr = parts[1]
			} else {
				uidStr = userFromImage
			}

			uid, err = strconv.ParseInt(uidStr, 10, 64)
			if err != nil {
				logger.WithError(err).Tracef("strconv.ParseUint(uidStr=%s)", uidStr)
				uid = -1
			}

			if gidStr != "" {
				gid, err = strconv.ParseInt(gidStr, 10, 64)
				if err != nil {
					logger.WithError(err).Tracef("strconv.ParseUint(gidStr=%s)", gidStr)
					gid = -1
				}
			}

			logger.WithFields(
				log.Fields{
					"data":    userFromImage,
					"image":   targetContainer.Image,
					"uid.str": uidStr,
					"gid.str": gidStr,
					"uid":     uid,
					"gid":     gid,
				}).Trace("user.from.target.image")
		}

		if uid > -1 {
			runAsUserVal = &uid
			logger.Debugf("using.target.image.user=%v", uid)
		} else {
			runAsUserVal = &defaultNonRootUser
		}

		if gid == -1 {
			gid = uid
		}

		if runAsGroupVal == &defaultNonRootGroup && gid > -1 {
			runAsGroupVal = &gid
			logger.Debugf("using.target.image.user.group=%v", gid)
		}
	}

	var doRunAsNonRoot bool
	if targetIsNonRoot && commandParams.DoFallbackToTargetUser {
		doRunAsNonRoot = true
	}

	var disableRunAsTargetShellReason string
	if roRootFilesystem() && commandParams.DoRunAsTargetShell {
		disableRunAsTargetShellReason = "readonly target filesystem"
	}

	if targetIsNonRoot && commandParams.DoRunAsTargetShell {
		disableRunAsTargetShellReason = "runAsNonRoot target"
	}

	if disableRunAsTargetShellReason != "" {
		commandParams.DoRunAsTargetShell = false
		xc.Out.Info("cmd.input.param.disable",
			ovars{
				"name":   FlagRunAsTargetShell,
				"reason": disableRunAsTargetShellReason,
			})
	}

	if commandParams.DoRunAsTargetShell {
		logger.Trace("doRunAsTargetShell")
		commandParams.Entrypoint = ShellCommandPrefix(commandParams.DebugContainerImage)
		shellConfig := configShell(sid, true)
		if CgrCustomDebugImage == commandParams.DebugContainerImage {
			shellConfig = configShellAlt(sid, true)
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

	var targetEnvVars []corev1.EnvVar
	if commandParams.DoLoadTargetEnvVars {
		logger.Trace("doLoadTargetEnvVars")
		targetEnvVars = targetContainer.Env
	}

	var targetVolumes []corev1.VolumeMount
	if commandParams.DoMountTargetVolumes ||
		commandParams.UID > 0 ||
		(targetIsNonRoot && commandParams.DoFallbackToTargetUser) {
		logger.Trace("doMountTargetVolumes")
		for _, record := range targetContainer.VolumeMounts {
			if record.SubPath == "" {
				targetVolumes = append(targetVolumes, record)
			}
		}
	}

	var securityCtx *corev1.SecurityContext
	if commandParams.UseSecurityContextFromTarget {
		securityCtx = targetContainer.SecurityContext
	}

	logger.WithFields(
		log.Fields{
			"work.dir":         commandParams.Workdir,
			"params":           fmt.Sprintf("%#v", commandParams),
			"run.as.nonroot":   doRunAsNonRoot,
			"is.ec.privileged": isEcPrivileged,
		}).Trace("newEphemeralContainerInfo")

	//TODO: pass commandParams.DoTerminal
	ecInfo := newEphemeralContainerInfo(
		commandParams.TargetRef,
		debugContainerName,
		commandParams.DebugContainerImage,
		commandParams.Entrypoint,
		commandParams.Cmd,
		commandParams.Workdir,
		targetEnvVars,
		commandParams.EnvVars,
		targetVolumes,
		commandParams.Volumes,
		isEcPrivileged,
		commandParams.UID,
		commandParams.GID,
		securityCtx,
		doRunAsNonRoot,
		runAsUserVal,
		runAsGroupVal,
		doTTY)

	logger.Tracef("Debugger sidecar spec: %s", jsonutil.ToString(ecInfo))
	pod.Spec.EphemeralContainers = append(pod.Spec.EphemeralContainers, ecInfo)

	_, err = api.CoreV1().
		Pods(pod.Namespace).
		UpdateEphemeralContainers(ctx, pod.Name, pod, metav1.UpdateOptions{})

	if err != nil {
		logger.WithError(err).Error("error adding the ephemeral container to target pod")
		xc.FailOn(err)
	}

	updatedPod, err := api.CoreV1().
		Pods(pod.Namespace).
		Get(ctx, pod.Name, metav1.GetOptions{})
	if err != nil {
		logger.WithError(err).Error("error getting the ephemeral container from target pod")
		xc.FailOn(err)
	}

	logger.WithFields(
		log.Fields{
			"ns":     nsName,
			"pod":    podName,
			"target": commandParams.TargetRef,
			"image":  commandParams.DebugContainerImage,
		}).Debug("attached ephemeral container")

	ec := ephemeralContainerFromPod(updatedPod, commandParams.TargetRef, debugContainerName)
	if ec == nil {
		logger.Errorf("ephemeral container not found in pod")
		xc.FailOn(fmt.Errorf("ephemeral container not found"))
	}

	ecData, _ := json.Marshal(ec)
	logger.WithField("data", string(ecData)).Trace("ephemeral container")

	var ecContainerIsRunning bool
	for _, ecStatus := range updatedPod.Status.EphemeralContainerStatuses {
		if ecStatus.Name == debugContainerName {
			if ecStatus.State.Running != nil {
				ecContainerIsRunning = true
			}
			break
		}
	}

	if !ecContainerIsRunning {
		xc.Out.Info("wait.for.debug.container",
			ovars{
				"name":      debugContainerName,
				"pod":       podName,
				"namespace": nsName,
			})

		err = waitForContainer(logger, xc, ctx, api, nsName, podName, debugContainerName, ctEphemeral)
		if err != nil {
			logger.WithError(err).Error("waitForContainer")

			if err == ErrContainerTerminated {
				xc.Out.Error("debug.container.error", "terminated")

				if err := dumpK8sContainerLogs(logger, xc, ctx, api, nsName, podName, debugContainerName); err != nil {
					logger.WithError(err).Error("dumpK8sContainerLogs")
				}

				xc.Out.State("debug.container.error",
					ovars{
						"exit.code": -1,
					})
				xc.Exit(-1)
			} else {
				xc.FailOn(err)
			}
		}
	}

	xc.Out.State("debug.container.running")

	req := api.CoreV1().RESTClient().Post().
		Resource("pods").
		Name(podName).
		Namespace(nsName).
		SubResource("attach").
		VersionedParams(&corev1.PodAttachOptions{
			Container: debugContainerName,
			Stdin:     true,
			Stdout:    true,
			Stderr:    true,
			TTY:       doTTY,
		}, scheme.ParameterCodec)

	logger.Tracef("pod attach request URL: %s", req.URL())

	attach, err := remotecommand.NewSPDYExecutor(restConfig, http.MethodPost, req.URL())
	if err != nil {
		logger.WithError(err).Error("remotecommand.NewSPDYExecutor")
		xc.FailOn(err)
	}

	xc.Out.Info("terminal.start",
		ovars{
			"note": "press enter if you dont see any output",
		})

	logger.Trace("starting stream...")
	//TODO:
	//use commandParams.DoTerminal to conditionally enable the interactive terminal
	//if false configure stream to do a one off command execution
	//and dump the container logs
	//similar to how it's done with the docker runtime

	fmt.Printf("\n")
	//note: blocks until done streaming or failure...
	if commandParams.TUI {
		// TODO - move RuntimeCommunicator off of command params
		reader := &TUIReader{inputChan: commandParams.RuntimeCommunicator.InputChan}
		err = attach.StreamWithContext(
			ctx,
			remotecommand.StreamOptions{
				Stdin:  reader,
				Stdout: os.Stdout,
				Stderr: os.Stderr,
				Tty:    true, // Later on we may parse this in TUI mode.
			})
	} else {
		err = attach.StreamWithContext(
			ctx,
			remotecommand.StreamOptions{
				Stdin:  os.Stdin,
				Stdout: os.Stdout,
				Stderr: os.Stderr,
				Tty:    doTTY,
			})
	}

	if err != nil {
		if apierrors.IsNotFound(err) {
			logger.WithError(err).
				Error("attach.StreamWithContext - not found")
		} else if statusError, isStatus := err.(*apierrors.StatusError); isStatus {
			logger.WithError(err).
				WithFields(log.Fields{
					"status": statusError.ErrStatus.Message,
				}).Error("attach.StreamWithContext - status error")
		} else {
			logger.WithError(err).
				Error("attach.StreamWithContext - other error")
		}

		xc.FailOn(err)
	}
}

// NOTE -> this input channel reader will be genericized
// as per the comment in `debug/tui.go`.
// An InputReader usable by Docker, Podman, Kubernetes, and Containerd
// will be added to this directory.
type TUIReader struct {
	inputChan chan InputKey
}

func (tuiReader *TUIReader) Read(p []byte) (n int, err error) {
	inputKey, ok := <-tuiReader.inputChan
	if !ok {
		return 0, io.EOF
	}
	log.Debugf("TUIReader received inputKey %v", inputKey)
	switch inputKey.Special {
	case NotSpecial:
		p[0] = byte(inputKey.Rune)
		return 1, nil
	case Enter:
		p[0] = '\n'
		return 1, nil
	case Backspace:
		p[0] = 127
		return 1, nil
	case Up:
		copy(p, []byte{27, 91, 65}) // ESC [ A
		return 3, nil
	case Down:
		copy(p, []byte{27, 91, 66}) // ESC [ B
		return 3, nil
	case Left:
		copy(p, []byte{27, 91, 68}) // ESC [ D
		return 3, nil
	case Right:
		copy(p, []byte{27, 91, 67}) // ESC [ C
		return 3, nil
	default:
		log.Debugf("Unhandled inputKey %v", inputKey)
		// Handle other special keys or return an error
		return 0, fmt.Errorf("unsupported special key")
	}
}

func listNamespaces(ctx context.Context, api *kubernetes.Clientset) ([]string, error) {
	namespaces, err := api.CoreV1().Namespaces().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	if len(namespaces.Items) == 0 {
		return []string{}, nil
	}

	var names []string
	for _, nsInfo := range namespaces.Items {
		names = append(names, nsInfo.Name)
	}

	return names, nil
}

func listNamespacesWithConfig(kubeconfig string) ([]string, error) {
	ctx := context.Background()

	api, _, err := apiClientFromConfig(kubeconfig)
	if err != nil {
		log.WithError(err).Error("apiClientFromConfig")
		return nil, err
	}

	names, err := listNamespaces(ctx, api)
	if err != nil {
		log.WithError(err).Error("listNamespaces")
		return nil, err
	}

	return names, nil
}

func ensureNamespace(ctx context.Context, api *kubernetes.Clientset, name string) (string, error) {
	if name == "" {
		namespaces, err := api.CoreV1().Namespaces().List(ctx, metav1.ListOptions{})
		if err != nil {
			return "", err
		}

		if len(namespaces.Items) == 0 {
			return "", fmt.Errorf("no namespaces")
		}

		return namespaces.Items[0].Name, nil
	}

	_, err := api.CoreV1().Namespaces().Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			log.Debugf("ensureNamespace: %s namespace is not found", name)
		}

		return "", err
	}

	return name, nil
}

func listActivePods(ctx context.Context, api *kubernetes.Clientset, nsName string) ([]string, error) {
	pods, err := api.CoreV1().Pods(nsName).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	if len(pods.Items) == 0 {
		return []string{}, nil
	}

	var names []string
	for _, podInfo := range pods.Items {
		switch podInfo.Status.Phase {
		case corev1.PodRunning, corev1.PodPending:
			names = append(names, podInfo.Name)
		}
	}

	return names, nil
}

func listActivePodsWithConfig(kubeconfig string, nsName string) ([]string, error) {
	ctx := context.Background()

	api, _, err := apiClientFromConfig(kubeconfig)
	if err != nil {
		log.WithError(err).Error("apiClientFromConfig")
		return nil, err
	}

	names, err := listActivePods(ctx, api, nsName)
	if err != nil {
		log.WithError(err).Error("listActivePods")
		return nil, err
	}

	return names, nil
}

func listAllActiveContainers(
	ctx context.Context,
	api *kubernetes.Clientset,
	nsName string,
	podName string) ([]string, error) {

	pod, err := api.CoreV1().Pods(nsName).Get(ctx, podName, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	if pod.Status.Phase != corev1.PodRunning {
		return nil, ErrPodNotRunning
	}

	var names []string
	cnl := getActiveContainerNames(pod.Status.ContainerStatuses)
	names = append(names, cnl...)
	icnl := getActiveContainerNames(pod.Status.InitContainerStatuses)
	names = append(names, icnl...)
	ecnl := getActiveContainerNames(pod.Status.EphemeralContainerStatuses)
	names = append(names, ecnl...)

	return names, nil
}

func listK8sDebuggableContainers(
	ctx context.Context,
	api *kubernetes.Clientset,
	nsName string,
	podName string) (map[string]string, error) {

	pod, err := api.CoreV1().Pods(nsName).Get(ctx, podName, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	if pod.Status.Phase != corev1.PodRunning {
		return nil, ErrPodNotRunning
	}

	activeNames := getActiveContainerNames(pod.Status.ContainerStatuses)
	activeContainers := map[string]string{}
	for _, name := range activeNames {
		activeContainers[name] = ""
	}

	for _, c := range pod.Spec.Containers {
		_, found := activeContainers[c.Name]
		if found {
			activeContainers[c.Name] = c.Image
		}
	}

	return activeContainers, nil
}

func listDebuggableK8sContainersWithConfig(
	kubeconfig string,
	nsName string,
	podName string) (map[string]string, error) {
	ctx := context.Background()

	api, _, err := apiClientFromConfig(kubeconfig)
	if err != nil {
		log.WithError(err).Error("apiClientFromConfig")
		return nil, err
	}

	_, podName, err = ensurePod(ctx, api, nsName, podName)
	if err != nil {
		if apierrors.IsNotFound(err) {
			log.WithError(err).
				WithFields(log.Fields{
					"ns":  nsName,
					"pod": podName,
				}).Error("ensurePod - not found")
		} else if statusError, isStatus := err.(*apierrors.StatusError); isStatus {
			log.WithError(err).
				WithFields(log.Fields{
					"ns":     nsName,
					"pod":    podName,
					"status": statusError.ErrStatus.Message,
				}).Error("ensurePod - status error")
		} else if err != nil {
			log.WithError(err).
				WithFields(log.Fields{
					"ns":  nsName,
					"pod": podName,
				}).Error("ensurePod - other error")
		}
		return nil, err
	}

	result, err := listK8sDebuggableContainers(ctx, api, nsName, podName)
	if err != nil {
		log.WithError(err).Error("listK8sDebuggableContainers")
		return nil, err
	}

	return result, nil
}

func listK8sDebugContainers(
	ctx context.Context,
	api *kubernetes.Clientset,
	nsName string,
	podName string,
	targetContainer string,
	onlyActive bool) (map[string]*DebugContainerInfo, error) {

	pod, err := api.CoreV1().Pods(nsName).Get(ctx, podName, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	if pod.Status.Phase != corev1.PodRunning {
		return nil, ErrPodNotRunning
	}

	all := map[string]*DebugContainerInfo{}
	for _, ec := range pod.Spec.EphemeralContainers {
		if !strings.HasPrefix(ec.Name, containerNamePrefix) {
			log.WithFields(log.Fields{
				"op":        "listK8sDebugContainers",
				"ns":        nsName,
				"pod":       podName,
				"container": ec.Name,
			}).Trace("ignoring.other.ec")
			continue
		}

		if targetContainer != "" && ec.TargetContainerName != targetContainer {
			log.WithFields(log.Fields{
				"op":              "listK8sDebugContainers",
				"ns":              nsName,
				"pod":             podName,
				"container":       ec.Name,
				"target.selected": targetContainer,
				"target":          ec.TargetContainerName,
			}).Trace("ignoring.ec")
			continue
		}

		info := &DebugContainerInfo{
			TargetContainerName: ec.TargetContainerName,
			Name:                ec.Name,
			SpecImage:           ec.Image,
			Command:             ec.Command,
			Args:                ec.Args,
			WorkingDir:          ec.WorkingDir,
			TTY:                 ec.TTY,
		}

		all[info.Name] = info
	}

	result := map[string]*DebugContainerInfo{}
	for _, status := range pod.Status.EphemeralContainerStatuses {
		info, found := all[status.Name]
		if !found {
			continue
		}

		info.ContainerID = status.ContainerID
		info.RunningImage = status.Image
		info.RunningImageID = status.ImageID

		if status.State.Waiting != nil {
			info.State = CSWaiting
			info.WaitReason = status.State.Waiting.Reason
			info.WaitMessage = status.State.Waiting.Message
		}

		if status.State.Running != nil {
			info.State = CSRunning
			info.StartTime = fmt.Sprintf("%v", status.State.Running.StartedAt)
		}

		if status.State.Terminated != nil {
			info.State = CSTerminated
			info.ExitCode = status.State.Terminated.ExitCode
			info.ExitReason = status.State.Terminated.Reason
			info.ExitMessage = status.State.Terminated.Message
			info.StartTime = fmt.Sprintf("%v", status.State.Terminated.StartedAt)
			info.FinishTime = fmt.Sprintf("%v", status.State.Terminated.FinishedAt)
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

func listK8sDebugContainersWithConfig(
	kubeconfig string,
	nsName string,
	podName string,
	targetContainer string,
	onlyActive bool) (map[string]*DebugContainerInfo, error) {
	ctx := context.Background()

	api, _, err := apiClientFromConfig(kubeconfig)
	if err != nil {
		log.WithError(err).Error("apiClientFromConfig")
		return nil, err
	}

	_, podName, err = ensurePod(ctx, api, nsName, podName)
	if err != nil {
		if apierrors.IsNotFound(err) {
			log.WithError(err).
				WithFields(log.Fields{
					"ns":  nsName,
					"pod": podName,
				}).Error("ensurePod - not found")
		} else if statusError, isStatus := err.(*apierrors.StatusError); isStatus {
			log.WithError(err).
				WithFields(log.Fields{
					"ns":     nsName,
					"pod":    podName,
					"status": statusError.ErrStatus.Message,
				}).Error("ensurePod - status error")
		} else if err != nil {
			log.WithError(err).
				WithFields(log.Fields{
					"ns":     nsName,
					"pod":    podName,
					"status": statusError.ErrStatus.Message,
				}).Error("ensurePod - other error")
		}
		return nil, err
	}

	result, err := listK8sDebugContainers(ctx, api, nsName, podName, targetContainer, onlyActive)
	if err != nil {
		log.WithError(err).Error("listK8sDebugContainers")
		return nil, err
	}

	return result, nil
}

func getActiveContainerNames(input []corev1.ContainerStatus) []string {
	var list []string
	for _, status := range input {
		if status.State.Running != nil || status.State.Waiting != nil {
			list = append(list, status.Name)
		}
	}

	return list
}

func ensurePod(ctx context.Context, api *kubernetes.Clientset, nsName string, podName string) (*corev1.Pod, string, error) {
	if podName == "" {
		pods, err := api.CoreV1().Pods(nsName).List(ctx, metav1.ListOptions{})
		if err != nil {
			return nil, "", err
		}

		if len(pods.Items) == 0 {
			return nil, "", fmt.Errorf("no pods")
		}

		podName = pods.Items[0].Name
	}

	var outputPod *corev1.Pod
	isPodRunning := func() (bool, error) {
		pod, err := api.CoreV1().Pods(nsName).Get(ctx, podName, metav1.GetOptions{})
		if err != nil {
			return false, err
		}

		switch pod.Status.Phase {
		case corev1.PodRunning:
			outputPod = pod
			return true, nil
		case corev1.PodFailed, corev1.PodSucceeded:
			return false, fmt.Errorf("pod is done")
		}
		return false, nil
	}

	err := wait.PollImmediate(2*time.Second, 2*time.Minute, isPodRunning)
	if err != nil {
		return nil, "", err
	}

	return outputPod, podName, nil
}

const (
	ctInit      = "init"
	ctStandard  = "standard"
	ctEphemeral = "ephemeral"
)

var (
	ErrPodTerminated       = errors.New("Pod terminated")
	ErrPodNotRunning       = errors.New("Pod not running")
	ErrContainerTerminated = errors.New("Container terminated")
)

type CreateContainerError struct {
	Reason  string
	Message string
}

func (e *CreateContainerError) Error() string {
	return fmt.Sprintf("Error: reason='%s' message='%s'", e.Reason, e.Message)
}

func waitForContainer(
	logger *log.Entry,
	xc *app.ExecutionContext,
	ctx context.Context,
	api *kubernetes.Clientset,
	nsName string,
	podName string,
	containerName string,
	containerType string) error {
	logger.Tracef("waitForContainer(%s,%s,%s,%s)", nsName, podName, containerName, containerType)

	isContainerRunning := func() (bool, error) {
		pod, err := api.CoreV1().Pods(nsName).Get(ctx, podName, metav1.GetOptions{})
		if err != nil {
			return false, err
		}

		switch pod.Status.Phase {
		case corev1.PodRunning:
			var statuses []corev1.ContainerStatus
			switch containerType {
			case ctInit:
				statuses = pod.Status.InitContainerStatuses
			case ctStandard:
				statuses = pod.Status.ContainerStatuses
			case ctEphemeral:
				statuses = pod.Status.EphemeralContainerStatuses
			default:
				return false, fmt.Errorf("unknown container type")
			}

			logger.Tracef("waitForContainer: statuses (%d)", len(statuses))
			for _, status := range statuses {
				if status.Name == containerName {
					if status.State.Running != nil {
						logger.Tracef("waitForContainer: RUNNING - %s/%s/%s[%s]", nsName, podName, containerName, containerType)

						if xc != nil {
							xc.Out.Info("wait.for.container.done",
								ovars{
									"state":      "RUNNING",
									"name":       containerName,
									"pod":        podName,
									"namespace":  nsName,
									"type":       containerType,
									"start_time": fmt.Sprintf("%v", status.State.Running.StartedAt),
									"id":         status.ContainerID,
								})
						}

						return true, nil
					} else {
						logger.Trace("waitForContainer: target is not running yet...")

						if xc != nil {
							paramVars := ovars{
								"name":      containerName,
								"pod":       podName,
								"namespace": nsName,
								"type":      containerType,
								"id":        status.ContainerID,
							}

							if status.Started != nil && *status.Started {
								paramVars["is_started"] = true
							}

							if status.State.Waiting != nil {
								paramVars["state"] = "WAITING"

								if status.State.Waiting.Reason != "" {
									paramVars["reason"] = status.State.Waiting.Reason
								}

								if status.State.Waiting.Message != "" {
									paramVars["message"] = status.State.Waiting.Message
								}
							}

							if status.State.Terminated != nil {
								paramVars["state"] = "TERMINATED"
								paramVars["exit_code"] = status.State.Terminated.ExitCode

								if status.State.Terminated.Reason != "" {
									paramVars["reason"] = status.State.Terminated.Reason
								}

								if status.State.Terminated.Message != "" {
									paramVars["message"] = status.State.Terminated.Message
								}
							}

							xc.Out.Info("wait.for.container", paramVars)
						}

						if status.State.Waiting != nil &&
							status.State.Waiting.Reason == "CreateContainerConfigError" {
							cce := &CreateContainerError{
								Reason:  status.State.Waiting.Reason,
								Message: status.State.Waiting.Message,
							}

							return false, cce
						}

						if status.State.Terminated != nil {
							return false, ErrContainerTerminated
						}
					}
				}
			}

			//don't fail right away, let it time out...
			return false, nil
		case corev1.PodFailed, corev1.PodSucceeded:
			return false, ErrPodTerminated
		}

		return false, nil
	}

	return wait.PollImmediate(2*time.Second, 4*time.Minute, isContainerRunning)
}

func dumpK8sContainerLogs(
	logger *log.Entry,
	xc *app.ExecutionContext,
	ctx context.Context,
	api *kubernetes.Clientset,
	nsName string,
	podName string,
	containerName string) error {
	logger.Tracef("dumpK8sContainerLogs(%s,%s,%s)", nsName, podName, containerName)

	options := &corev1.PodLogOptions{
		Container: containerName,
	}

	req := api.CoreV1().
		Pods(nsName).
		GetLogs(podName, options)

	containerLogs, err := req.Stream(ctx)
	if err != nil {
		logger.WithError(err).Error("error streaming container logs")
		return err
	}
	defer containerLogs.Close()

	/*
		var outData bytes.Buffer
		_, err = io.Copy(&outData, containerLogs)
		if err != nil {
			logger.WithError(err).Error("error copying container logs")
			return err
		}

		fmt.Printf("%s\n", outData.String())
		//_, _ = outData.WriteTo(os.Stdout)
	*/

	outData, err := ioutil.ReadAll(containerLogs)
	if err != nil {
		logger.WithError(err).Error("error reading container logs")
		return err
	}

	xc.Out.Info("container.logs.start")
	xc.Out.LogDump("debug.container.logs", string(outData))
	xc.Out.Info("container.logs.end")
	return nil
}

func ephemeralContainerFromPod(
	pod *corev1.Pod,
	target string,
	name string) *corev1.EphemeralContainer {
	for _, ec := range pod.Spec.EphemeralContainers {
		if ec.TargetContainerName == target &&
			ec.Name == name {
			return &ec
		}
	}

	return nil
}

var isTrue bool = true
var defaultNonRootUser int64 = 1000
var defaultNonRootGroup int64 = 1000

func newEphemeralContainerInfo(
	target string, // target container in the pod
	name string, // name to use for the ephemeral container (must be unique)
	image string, // image to use for the ephemeral container
	command []string, // custom ENTRYPOINT to use for the ephemeral container (yes, it's not CMD :-))
	args []string, // custom CMD to use
	workingDir string,
	targetEnvVars []corev1.EnvVar,
	envVars []NVPair,
	targetVolumes []corev1.VolumeMount,
	volumes []Volume,
	isPrivileged bool, // true if it should be a privileged container
	uid int64,
	gid int64,
	securityCtx *corev1.SecurityContext,
	doRunAsNonRoot bool,
	runAsUserVal *int64,
	runAsGroupVal *int64,
	doTTY bool,
) corev1.EphemeralContainer {
	out := corev1.EphemeralContainer{
		TargetContainerName: target,
		EphemeralContainerCommon: corev1.EphemeralContainerCommon{
			TTY:          doTTY,
			Stdin:        true,
			Name:         name,
			Image:        image,
			Command:      command,
			Args:         args,
			WorkingDir:   workingDir,
			Env:          targetEnvVars,
			VolumeMounts: targetVolumes,
			//TODO: add support for more params
		},
	}

	var sc corev1.SecurityContext
	if securityCtx != nil {
		sc = *securityCtx
	}

	out.EphemeralContainerCommon.SecurityContext = &sc

	for _, val := range envVars {
		if val.Name == "" {
			continue
		}

		record := corev1.EnvVar{Name: val.Name, Value: val.Value}
		out.Env = append(out.Env, record)
	}

	for _, val := range volumes {
		if val.Name == "" {
			continue
		}

		record := corev1.VolumeMount{
			Name:      val.Name,
			MountPath: val.Path,
			ReadOnly:  val.ReadOnly,
		}
		out.VolumeMounts = append(out.VolumeMounts, record)
	}

	if isPrivileged {
		out.EphemeralContainerCommon.SecurityContext.Privileged = &isTrue
	} else if securityCtx != nil {
		out.EphemeralContainerCommon.SecurityContext.Privileged = &isPrivileged
	}

	if uid > -1 {
		out.EphemeralContainerCommon.SecurityContext.RunAsUser = &uid
	}

	if gid > -1 {
		out.EphemeralContainerCommon.SecurityContext.RunAsGroup = &gid
	} else if uid > -1 {
		out.EphemeralContainerCommon.SecurityContext.RunAsGroup = &uid
	}

	if doRunAsNonRoot {
		out.EphemeralContainerCommon.SecurityContext.RunAsNonRoot = &isTrue
		if out.EphemeralContainerCommon.SecurityContext.RunAsUser == nil &&
			runAsUserVal != nil {
			out.EphemeralContainerCommon.SecurityContext.RunAsUser = runAsUserVal
		}

		if out.EphemeralContainerCommon.SecurityContext.RunAsGroup == nil &&
			runAsGroupVal != nil {
			out.EphemeralContainerCommon.SecurityContext.RunAsGroup = runAsGroupVal
		}
	}

	return out
}

func apiClientFromConfig(kubeconfig string) (*kubernetes.Clientset, *restclient.Config, error) {
	kubeconfig = os.ExpandEnv(kubeconfig)

	config, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
	if err != nil {
		return nil, nil, err
	}
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, nil, err
	}

	return clientset, config, nil
}
