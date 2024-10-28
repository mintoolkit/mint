package images

import (
	"context"
	"fmt"
	"time"

	"github.com/dustin/go-humanize"
	docker "github.com/fsouza/go-dockerclient"
	"github.com/jedib0t/go-pretty/v6/table"
	log "github.com/sirupsen/logrus"

	"github.com/mintoolkit/mint/pkg/app"
	"github.com/mintoolkit/mint/pkg/app/master/command"
	"github.com/mintoolkit/mint/pkg/app/master/tui"
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

// OnCommand implements the 'images' command
func OnCommand(
	xc *app.ExecutionContext,
	gparams *command.GenericParams,
	cparams *CommandParams,
) {
	const cmdName = Name

	logger := log.WithFields(log.Fields{"app": appName, "cmd": cmdName})
	viChan := version.CheckAsync(gparams.CheckVersion, gparams.InContainer, gparams.IsDSImage)

	cmdReport := report.NewImagesCommand(gparams.ReportLocation, gparams.InContainer)

	cmdReport.State = cmd.StateStarted

	// We will want to communicate this completed state to the TUI.
	xc.Out.State(cmd.StateStarted)
	rr := command.ResolveAutoRuntime(cparams.Runtime)
	if rr != cparams.Runtime {
		rr = fmt.Sprintf("%s/%s", cparams.Runtime, rr)
	}

	xc.Out.Info("cmd.input.params",
		ovars{
			"runtime": rr,
			"filter":  cparams.Filter,
		})

	resolved := command.ResolveAutoRuntime(cparams.Runtime)
	logger.Tracef("runtime.handler: rt=%s resolved=%s", cparams.Runtime, resolved)

	var err error
	var dclient *docker.Client
	var pclient context.Context
	var crtClient crt.APIClient

	switch resolved {
	case crt.DockerRuntime:
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
		crtClient = dockercrtclient.New(dclient)
	case crt.PodmanRuntime:
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
		crtClient = podmancrtclient.New(pclient)
	default:
		xc.Out.Error("runtime", "unsupported runtime")
		xc.Out.State("exited",
			ovars{
				"exit.code":        -1,
				"version":          v.Current(),
				"location":         fsutil.ExeDir(),
				"runtime":          cparams.Runtime,
				"runtime.resolved": resolved,
			})
		xc.Exit(-1)
	}

	if gparams.Debug {
		version.Print(xc, cmdName, logger, dclient, false, gparams.InContainer, gparams.IsDSImage)
	}

	images, err := crtClient.ListImages(cparams.Filter)
	xc.FailOn(err)

	if cparams.TUI { // `images --tui`
		initialTUI := InitialTUI(images, true)
		tui.RunTUI(initialTUI, true)
	} else if cparams.GlobalTUI { // `tui` -> `i`
		// TODO - create a central store for the lookup key.
		// As this key needs to be the same on the sender and the receiver.
		xc.Out.Data("images", images)
	} else if xc.Out.Quiet {
		if xc.Out.OutputFormat == command.OutputFormatJSON {
			fmt.Printf("%s\n", jsonutil.ToPretty(images))
		}

		printImagesTable(images)
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

	// We will want to communicate this completed state to the TUI.
	xc.Out.State("completed")
	// Upon hitting "completed", this is when we could pass the 'complete' images data
	// to the TUI.
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

func printImagesTable(images map[string]crt.BasicImageInfo) {
	tw := table.NewWriter()
	tw.AppendHeader(table.Row{"Name", "ID", "Size", "Created"})

	for name, info := range images {
		tw.AppendRow(table.Row{
			name,
			info.ID,
			humanize.Bytes(uint64(info.Size)),
			time.Unix(info.Created, 0).Format(time.RFC3339),
		})
	}

	tw.SetStyle(table.StyleLight)
	tw.Style().Options.DrawBorder = false
	fmt.Printf("%s\n", tw.Render())
}
