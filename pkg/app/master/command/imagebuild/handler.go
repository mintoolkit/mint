package imagebuild

import (
	"context"

	docker "github.com/fsouza/go-dockerclient"
	log "github.com/sirupsen/logrus"

	"github.com/mintoolkit/mint/pkg/app"
	"github.com/mintoolkit/mint/pkg/app/master/command"
	"github.com/mintoolkit/mint/pkg/app/master/version"
	cmd "github.com/mintoolkit/mint/pkg/command"

	"github.com/mintoolkit/mint/pkg/crt"
	"github.com/mintoolkit/mint/pkg/crt/docker/dockerclient"

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

	switch cparams.Engine {
	case DockerBuildEngine:
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

	//images, err := crtClient.ListImages(cparams.Filter)
	//xc.FailOn(err)

	/*
		if xc.Out.Quiet {
			if xc.Out.OutputFormat == command.OutputFormatJSON {
				fmt.Printf("%s\n", jsonutil.ToPretty(images))
				return
			}

			printImagesTable(images)
			return
		} else {
			xc.Out.Info("image.list", ovars{"count": len(images)})
			for name, info := range images {
				fields := ovars{
					"name":    name,
					"id":      info.ID,
					"size":    humanize.Bytes(uint64(info.Size)),
					"created": time.Unix(info.Created, 0).Format(time.RFC3339),
				}

				xc.Out.Info("image", fields)
			}
		}
	*/

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
