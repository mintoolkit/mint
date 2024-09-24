package imagebuild

import (
	"context"
	"os"

	docker "github.com/fsouza/go-dockerclient"
	log "github.com/sirupsen/logrus"

	"github.com/mintoolkit/mint/pkg/app"
	"github.com/mintoolkit/mint/pkg/app/master/command"
	"github.com/mintoolkit/mint/pkg/app/master/version"
	cmd "github.com/mintoolkit/mint/pkg/command"

	"github.com/mintoolkit/mint/pkg/crt"
	"github.com/mintoolkit/mint/pkg/crt/docker/dockerclient"
	"github.com/mintoolkit/mint/pkg/crt/docker/dockercrtclient"
	"github.com/mintoolkit/mint/pkg/crt/podman/podmancrtclient"

	"github.com/mintoolkit/mint/pkg/report"
	"github.com/mintoolkit/mint/pkg/util/fsutil"
	"github.com/mintoolkit/mint/pkg/util/jsonutil"
	v "github.com/mintoolkit/mint/pkg/version"
)

const appName = command.AppName

type ovars = app.OutVars

// OnCommand implements the 'imagebuild' command
func OnCommand(
	xc *app.ExecutionContext,
	gparams *command.GenericParams,
	cparams *CommandParams) {
	const cmdName = Name
	logger := log.WithFields(log.Fields{"app": appName, "cmd": cmdName})

	viChan := version.CheckAsync(gparams.CheckVersion, gparams.InContainer, gparams.IsDSImage)

	cmdReport := report.NewImageBuildCommand(gparams.ReportLocation, gparams.InContainer)
	cmdReport.State = cmd.StateStarted

	xc.Out.State(cmd.StateStarted)
	//todo: runtime to load param also needs to auto-resolve...
	xc.Out.Info("cmd.input.params",
		ovars{
			"cparams": jsonutil.ToString(cparams),
		})

	var err error
	var dclient *docker.Client
	var pclient context.Context

	initDockerClient := func() {
		dclient, err = dockerclient.New(gparams.ClientConfig)
		if err == dockerclient.ErrNoDockerInfo {
			exitMsg := "missing Docker connection info"
			if gparams.InContainer && gparams.IsDSImage {
				exitMsg = "make sure to pass the Docker connect parameters to the mint container"
			}

			xc.Out.Error("docker.connect.error", exitMsg)

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
	}

	initPodmanClient := func() {
		if gparams.CRTConnection != "" {
			pclient = crt.GetPodmanConnContextWithConn(gparams.CRTConnection)
		} else {
			pclient = crt.GetPodmanConnContext()
		}

		if pclient == nil {
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
	}

	var isSame bool
	switch cparams.Engine {
	case DockerBuildEngine:
		initDockerClient()
		if cparams.Runtime == DockerRuntimeLoad {
			isSame = true
		}

		if gparams.Debug {
			version.Print(xc, cmdName, logger, dclient, false, gparams.InContainer, gparams.IsDSImage)
		}

		HandleDockerEngine(logger, xc, gparams, cparams, dclient)
	case DepotBuildEngine:
		if gparams.Debug {
			version.Print(xc, cmdName, logger, nil, false, gparams.InContainer, gparams.IsDSImage)
		}

		HandleDepotEngine(logger, xc, gparams, cparams)
	case BuildkitBuildEngine:
		if gparams.Debug {
			version.Print(xc, cmdName, logger, nil, false, gparams.InContainer, gparams.IsDSImage)
		}

		HandleBuildkitEngine(logger, xc, gparams, cparams)
	case SimpleBuildEngine:
		if gparams.Debug {
			version.Print(xc, cmdName, logger, nil, false, gparams.InContainer, gparams.IsDSImage)
		}

		HandleSimpleEngine(logger, xc, gparams, cparams)
	case PodmanBuildEngine:
		initPodmanClient()
		if cparams.Runtime == PodmanRuntimeLoad {
			isSame = true
		}

		if gparams.Debug {
			version.Print(xc, Name, logger, nil, false, gparams.InContainer, gparams.IsDSImage)
		}

		HandlePodmanEngine(logger, xc, gparams, cparams, pclient)
	default:
		xc.Out.Error("engine", "unsupported engine")
		xc.Out.State("exited",
			ovars{
				"exit.code": -1,
				"version":   v.Current(),
				"location":  fsutil.ExeDir(),
				"runtime":   cparams.Engine,
			})
		xc.Exit(-1)
	}

	var crtLoaderClient crt.ImageLoaderAPIClient
	switch cparams.Runtime {
	case DockerRuntimeLoad:
		if dclient == nil {
			initDockerClient()
		}

		crtLoaderClient = dockercrtclient.New(dclient)
	case PodmanRuntimeLoad:
		if pclient == nil {
			initPodmanClient()
		}

		crtLoaderClient = podmancrtclient.New(pclient)
	}

	if crtLoaderClient != nil {
		xc.Out.Info("runtime.load.image", ovars{
			"runtime":            cparams.Runtime,
			"image.archive.file": cparams.ImageArchiveFile,
		})

		if !isSame {
			err = crtLoaderClient.LoadImage(cparams.ImageArchiveFile, os.Stdout)
			xc.FailOn(err)
		} else {
			xc.Out.Info("same.image.engine.runtime")
		}
	} else {
		xc.Out.Info("runtime.load.image.none")
	}

	xc.Out.State(cmd.StateCompleted)
	cmdReport.State = cmd.StateCompleted

	vinfo := <-viChan
	version.PrintCheckVersion(xc, "", vinfo)

	xc.Out.State(cmd.StateDone)
	cmdReport.State = cmd.StateDone
	if cmdReport.Save() {
		xc.Out.Info("report",
			ovars{
				"file": cmdReport.ReportLocation(),
			})
	}
}
