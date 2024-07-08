package app

import (
	"os"
	"path/filepath"

	log "github.com/sirupsen/logrus"

	a "github.com/mintoolkit/mint/pkg/app"
	"github.com/mintoolkit/mint/pkg/app/master/command"
	"github.com/mintoolkit/mint/pkg/util/errutil"
	"github.com/mintoolkit/mint/pkg/util/fsutil"
	vinfo "github.com/mintoolkit/mint/pkg/version"
)

// OnInstallDCPCommand implements the 'app install-docker-cli-plugin' command
func OnInstallDCPCommand(
	xc *a.ExecutionContext,
	gparams *command.GenericParams) {
	cmdName := fullCmdName(InstallCmdName)
	logger := log.WithFields(log.Fields{
		"app": command.AppName,
		"cmd": cmdName,
		"sub": InstallDCPCmdName,
	})

	logger.Trace("call")
	defer logger.Trace("exit")

	appPath, err := os.Executable()
	errutil.FailOn(err)
	appDirPath := filepath.Dir(appPath)

	err = installDockerCLIPlugin(logger, gparams.StatePath, gparams.InContainer, gparams.IsDSImage, appDirPath)
	if err != nil {
		xc.Out.Info("status",
			ovars{
				"message": "error installing Docker CLI plugin",
			})

		exitCode := -998
		xc.Out.State("exited",
			ovars{
				"exit.code": exitCode,
				"version":   vinfo.Current(),
			})
		xc.Exit(exitCode)
	}

	xc.Out.State("app.install.docker.cli.plugin.done")
}

func installDockerCLIPlugin(logger *log.Entry, statePath string, inContainer, isDSImage bool, appDirPath string) error {
	hd, _ := os.UserHomeDir()
	dockerCLIPluginDir := filepath.Join(hd, dockerCLIPluginDirSuffx)

	if !fsutil.Exists(dockerCLIPluginDir) {
		var dirMode os.FileMode = 0755
		err := os.MkdirAll(dockerCLIPluginDir, dirMode)
		if err != nil {
			return err
		}
	}

	if err := symlinkBinaries(logger, appDirPath, dockerCLIPluginDir); err != nil {
		logger.Debugf("installDockerCLIPlugin error: %v", err)
		return err
	}

	return nil
}

func symlinkBinaries(logger *log.Entry, appRootPath, symlinkRootPath string) error {
	symlinkMasterAppPath := filepath.Join(symlinkRootPath, masterAppName)
	symlinkSensorAppPath := filepath.Join(symlinkRootPath, sensorAppName)
	targetSensorAppPath := filepath.Join(appRootPath, sensorAppName)
	targetMasterAppPath := filepath.Join(appRootPath, masterAppName)

	//todo:
	//should not symlink the sensor because Docker CLI will treat it as an invalid plugin
	//need to improve sensor bin discovery from master app symlink
	err := os.Symlink(targetSensorAppPath, symlinkSensorAppPath)
	if err != nil {
		return err
	}

	err = os.Symlink(targetMasterAppPath, symlinkMasterAppPath)
	if err != nil {
		return err
	}

	return nil
}
