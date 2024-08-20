package app

import (
	log "github.com/sirupsen/logrus"

	a "github.com/mintoolkit/mint/pkg/app"
	"github.com/mintoolkit/mint/pkg/app/master/command"
	"github.com/mintoolkit/mint/pkg/app/master/version"
	"github.com/mintoolkit/mint/pkg/crt/docker/dockerclient"
	"github.com/mintoolkit/mint/pkg/util/errutil"
	"github.com/mintoolkit/mint/pkg/util/fsutil"
	v "github.com/mintoolkit/mint/pkg/version"
)

// OnVersionCommand implements the 'app version' command
func OnVersionCommand(
	xc *a.ExecutionContext,
	gparams *command.GenericParams) {
	cmdName := fullCmdName(VersionCmdName)
	logger := log.WithFields(log.Fields{
		"app": command.AppName,
		"cmd": cmdName,
		"sub": VersionCmdName,
	})

	logger.Trace("call")
	defer logger.Trace("exit")

	client, err := dockerclient.New(gparams.ClientConfig)
	if err == dockerclient.ErrNoDockerInfo {
		exitMsg := "missing Docker connection info"
		if gparams.InContainer && gparams.IsDSImage {
			exitMsg = "make sure to pass the Docker connect parameters to the Mint app container"
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

	version.Print(xc, cmdName, logger, client, true, gparams.InContainer, gparams.IsDSImage)
}
