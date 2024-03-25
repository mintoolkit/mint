package debug

import (
	log "github.com/sirupsen/logrus"

	"github.com/mintoolkit/mint/pkg/app"
	"github.com/mintoolkit/mint/pkg/app/master/command"
	//"github.com/mintoolkit/mint/pkg/app/master/container"
	//"github.com/mintoolkit/mint/pkg/app/master/inspectors/image"
	"github.com/mintoolkit/mint/pkg/app/master/version"
	cmd "github.com/mintoolkit/mint/pkg/command"
	"github.com/mintoolkit/mint/pkg/docker/dockerclient"
	"github.com/mintoolkit/mint/pkg/report"
	//"github.com/mintoolkit/mint/pkg/util/errutil"
	"github.com/mintoolkit/mint/pkg/util/fsutil"
	v "github.com/mintoolkit/mint/pkg/version"
)

const appName = command.AppName

type ovars = app.OutVars

// OnCommand implements the 'debug' command
func OnCommand(
	xc *app.ExecutionContext,
	gparams *command.GenericParams,
	commandParams *CommandParams) {
	logger := log.WithFields(log.Fields{"app": appName, "cmd": Name})

	viChan := version.CheckAsync(gparams.CheckVersion, gparams.InContainer, gparams.IsDSImage)

	cmdReport := report.NewDebugCommand(gparams.ReportLocation, gparams.InContainer)
	cmdReport.State = cmd.StateStarted

	cmdReportOnExit := func() {
		cmdReport.State = cmd.StateError
		if cmdReport.Save() {
			xc.Out.Info("report",
				ovars{
					"file": cmdReport.ReportLocation(),
				})
		}
	}
	xc.AddCleanupHandler(cmdReportOnExit)

	xc.Out.State("started")
	paramVars := ovars{
		"runtime":             commandParams.Runtime,
		"target":              commandParams.TargetRef,
		"debug-image":         commandParams.DebugContainerImage,
		"entrypoint":          commandParams.Entrypoint,
		"cmd":                 commandParams.Cmd,
		"terminal":            commandParams.DoTerminal,
		"run-as-target-shell": commandParams.DoRunAsTargetShell,
	}

	if commandParams.Runtime == KubernetesRuntime {
		paramVars["namespace"] = commandParams.TargetNamespace
		paramVars["pod"] = commandParams.TargetPod
	}

	xc.Out.Info("params", paramVars)

	sid := generateSessionID()
	debugContainerName := generateContainerName(sid)
	logger = logger.WithFields(
		log.Fields{
			"sid":                  sid,
			"debug.container.name": debugContainerName,
		})

	switch commandParams.Runtime {
	case DockerRuntime:
		client, err := dockerclient.New(gparams.ClientConfig)
		if err == dockerclient.ErrNoDockerInfo {
			exitMsg := "missing Docker connection info"
			if gparams.InContainer && gparams.IsDSImage {
				exitMsg = "make sure to pass the Docker connect parameters to the slim app container"
			}

			xc.Out.Info("docker.connect.error",
				ovars{
					"message": exitMsg,
				})

			exitCode := command.ECTCommon | command.ECCNoDockerConnectInfo
			xc.Out.State("exited",
				ovars{
					"exit.code": exitCode,
					"version":   v.Current(),
					"location":  fsutil.ExeDir(),
				})
			xc.Exit(exitCode)
		}
		xc.FailOn(err)

		if gparams.Debug {
			version.Print(xc, Name, logger, client, false, gparams.InContainer, gparams.IsDSImage)
		}

		HandleDockerRuntime(logger, xc, gparams, commandParams, client, sid, debugContainerName)
	case KubernetesRuntime:
		if gparams.Debug {
			version.Print(xc, Name, logger, nil, false, gparams.InContainer, gparams.IsDSImage)
		}

		HandleKubernetesRuntime(logger, xc, gparams, commandParams, sid, debugContainerName)
	case ContainerdRuntime:
		if gparams.Debug {
			version.Print(xc, Name, logger, nil, false, gparams.InContainer, gparams.IsDSImage)
		}

		HandleContainerdRuntime(logger, xc, gparams, commandParams, sid, debugContainerName)
	default:
		xc.Out.Error("runtime", "unsupported runtime")
		xc.Out.State("exited",
			ovars{
				"exit.code": -1,
				"version":   v.Current(),
				"location":  fsutil.ExeDir(),
			})
		xc.Exit(-1)
	}

	xc.Out.State("completed")
	cmdReport.State = cmd.StateCompleted
	xc.Out.State("done")

	vinfo := <-viChan
	version.PrintCheckVersion(xc, "", vinfo)

	cmdReport.State = cmd.StateDone
	if cmdReport.Save() {
		xc.Out.Info("report",
			ovars{
				"file": cmdReport.ReportLocation(),
			})
	}
}
