package version

import (
	log "github.com/sirupsen/logrus"

	"github.com/mintoolkit/mint/pkg/app"
	//"github.com/mintoolkit/mint/pkg/app/master/commands"
	"github.com/mintoolkit/mint/pkg/app/master/config"
	"github.com/mintoolkit/mint/pkg/app/master/version"
	cmd "github.com/mintoolkit/mint/pkg/command"
	"github.com/mintoolkit/mint/pkg/docker/dockerclient"
	"github.com/mintoolkit/mint/pkg/util/errutil"
	"github.com/mintoolkit/mint/pkg/util/fsutil"
	v "github.com/mintoolkit/mint/pkg/version"
)

type ovars = app.OutVars

// OnCommand implements the 'version' command
func OnCommand(
	xc *app.ExecutionContext,
	doDebug, inContainer, isDSImage bool,
	clientConfig *config.DockerClient) {
	logger := log.WithFields(log.Fields{"app": "mint", "cmd": cmd.Version})

	client, err := dockerclient.New(clientConfig)
	if err == dockerclient.ErrNoDockerInfo {
		exitMsg := "missing Docker connection info"
		if inContainer && isDSImage {
			exitMsg = "make sure to pass the Docker connect parameters to the slim app container"
		}

		xc.Out.Info("docker.connect.error",
			ovars{
				"message": exitMsg,
			})

		exitCode := -777
		xc.Out.State("exited",
			ovars{
				"exit.code": exitCode,
				"version":   v.Current(),
				"location":  fsutil.ExeDir(),
			})
		xc.Exit(exitCode)
	}
	errutil.FailOn(err)

	version.Print(xc, Name, logger, client, true, inContainer, isDSImage)
}
