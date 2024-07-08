package app

import (
	log "github.com/sirupsen/logrus"

	a "github.com/mintoolkit/mint/pkg/app"
	"github.com/mintoolkit/mint/pkg/app/master/command"
	"github.com/mintoolkit/mint/pkg/app/master/version"
	"github.com/mintoolkit/mint/pkg/docker/dockerclient"
	"github.com/mintoolkit/mint/pkg/docker/dockerutil"
	"github.com/mintoolkit/mint/pkg/util/errutil"
	"github.com/mintoolkit/mint/pkg/util/fsutil"
	v "github.com/mintoolkit/mint/pkg/version"
)

const (
	sensorVolumePrefix = "mint-sensor."
)

// OnRsvCommand implements the 'app remove-sensor-volumes' command
func OnRsvCommand(
	xc *a.ExecutionContext,
	gparams *command.GenericParams) {
	cmdName := fullCmdName(RSVCmdName)
	logger := log.WithFields(log.Fields{
		"app": command.AppName,
		"cmd": cmdName,
		"sub": RSVCmdName,
	})

	logger.Trace("call")
	defer logger.Trace("exit")

	dclient, err := dockerclient.New(gparams.ClientConfig)
	if err == dockerclient.ErrNoDockerInfo {
		exitMsg := "missing Docker connection info"
		if gparams.InContainer && gparams.IsDSImage {
			exitMsg = "make sure to pass the Docker connect parameters to the mint container"
		}

		xc.Out.Error("docker.connect.error", exitMsg)
		exitCode := -888
		xc.Out.State("exited",
			ovars{
				"exit.code": exitCode,
				"version":   v.Current(),
				"location":  fsutil.ExeDir(),
			})
		xc.Exit(exitCode)
	}
	errutil.FailOn(err)

	if gparams.Debug {
		version.Print(xc, cmdName, logger, dclient, false, gparams.InContainer, gparams.IsDSImage)
	}

	names, err := dockerutil.ListVolumes(dclient, sensorVolumePrefix)
	if err != nil {
		logger.Errorf("dockerutil.ListVolumes: error - %v", err)
		return
	}

	xc.Out.Info("volumes", ovars{"names": names})

	for _, name := range names {
		err = dockerutil.DeleteVolume(dclient, name)
		if err != nil {
			logger.Errorf("dockerutil.DeleteVolume(%s): error - %v", name, err)
		} else {
			xc.Out.State("volume.deleted", ovars{"name": name})
		}
	}
}
