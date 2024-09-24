package imagebuild

import (
	"context"
	"fmt"
	"os"

	log "github.com/sirupsen/logrus"

	"github.com/mintoolkit/mint/pkg/app"
	"github.com/mintoolkit/mint/pkg/app/master/command"
	"github.com/mintoolkit/mint/pkg/crt/podman/podmancrtclient"
	"github.com/mintoolkit/mint/pkg/imagebuilder"
	"github.com/mintoolkit/mint/pkg/imagebuilder/standardbuilder"
	"github.com/mintoolkit/mint/pkg/util/fsutil"
	v "github.com/mintoolkit/mint/pkg/version"
)

// HandlePodmanEngine implements support for the Podman container build engine
func HandlePodmanEngine(
	logger *log.Entry,
	xc *app.ExecutionContext,
	gparams *command.GenericParams,
	cparams *CommandParams,
	client context.Context) {
	logger.Trace("HandlePodmanEngine.call")
	defer logger.Trace("HandlePodmanEngine.exit")
	xc.Out.State("podman.engine.image.build.started")

	doShowBuildLogs := true
	crtClient := podmancrtclient.NewBuilder(client, doShowBuildLogs)
	builder, err := standardbuilder.New(crtClient)

	//note: need to also "save" the created image if it needs to be loaded in a different runtime
	options := imagebuilder.DockerfileBuildOptions{
		OutputStream: os.Stdout, //doShowBuildLogs
		Dockerfile:   cparams.Dockerfile,
		BuildContext: cparams.ContextDir,
		ImagePath:    cparams.ImageName,
		BuildArgs:    cparams.BuildArgs,
		Labels:       cparams.Labels,
	}

	if cparams.Architecture != "" {
		options.Platforms = []string{
			fmt.Sprintf("linux/%s", cparams.Architecture),
		}
	}

	err = builder.Build(options)
	if err != nil {
		xc.Out.Info("build.error",
			ovars{
				"status": "podman.engine.image.build.error",
				"value":  err,
			})

		exitCode := 721
		xc.Out.State("exited",
			ovars{
				"exit.code": exitCode,
				"version":   v.Current(),
				"location":  fsutil.ExeDir(),
			})

		xc.Exit(exitCode)
	}

	xc.Out.State("podman.engine.image.build.completed")
}
