package app

import (
	"os"
	"path/filepath"

	"github.com/mintoolkit/go-update"
	log "github.com/sirupsen/logrus"

	a "github.com/mintoolkit/mint/pkg/app"
	"github.com/mintoolkit/mint/pkg/app/master/command"
	"github.com/mintoolkit/mint/pkg/util/errutil"
	"github.com/mintoolkit/mint/pkg/util/fsutil"
	vinfo "github.com/mintoolkit/mint/pkg/version"
)

const (
	dockerCLIPluginDirSuffx = "/.docker/cli-plugins"
	masterAppName           = "mint"
	sensorAppName           = "mint-sensor"
	binDirName              = "/usr/local/bin"
)

// OnInstallCommand implements the 'app install' command
func OnInstallCommand(
	xc *a.ExecutionContext,
	gparams *command.GenericParams,
	cparams *InstallCommandParams) {
	cmdName := fullCmdName(InstallCmdName)
	logger := log.WithFields(log.Fields{
		"app": command.AppName,
		"cmd": cmdName,
		"sub": InstallCmdName,
	})

	logger.Trace("call")
	defer logger.Trace("exit")

	appPath, err := os.Executable()
	errutil.FailOn(err)
	appDirPath := filepath.Dir(appPath)

	err = installToBinDir(logger, gparams.StatePath, gparams.InContainer, gparams.IsDSImage, appDirPath)
	if err != nil {
		xc.Out.Info("status",
			ovars{
				"message": "error installing to bin dir",
			})

		exitCode := -999
		xc.Out.State("exited",
			ovars{
				"exit.code": exitCode,
				"version":   vinfo.Current(),
			})
		xc.Exit(exitCode)
	}

	xc.Out.State("app.install.done")
}

func installToBinDir(logger *log.Entry, statePath string, inContainer, isDSImage bool, appDirPath string) error {
	if err := installRelease(logger, appDirPath, statePath, binDirName); err != nil {
		logger.Debugf("installToBinDir error: %v", err)
		return err
	}

	return nil
}

func installRelease(logger *log.Entry, appRootPath, statePath, targetRootPath string) error {
	targetMasterAppPath := filepath.Join(targetRootPath, masterAppName)
	targetSensorAppPath := filepath.Join(targetRootPath, sensorAppName)
	srcSensorAppPath := filepath.Join(appRootPath, sensorAppName)
	srcMasterAppPath := filepath.Join(appRootPath, masterAppName)

	err := updateFile(logger, srcSensorAppPath, targetSensorAppPath)
	if err != nil {
		return err
	}

	//will copy the sensor to the state dir if DS is installed in a bad non-shared location on Macs
	fsutil.PreparePostUpdateStateDir(statePath)

	err = updateFile(logger, srcMasterAppPath, targetMasterAppPath)
	if err != nil {
		return err
	}

	return nil
}

// copied from updater
func updateFile(logger *log.Entry, sourcePath, targetPath string) error {
	file, err := os.Open(sourcePath)
	if err != nil {
		return err
	}
	defer file.Close()

	options := update.Options{}
	if targetPath != "" {
		options.TargetPath = targetPath
	}

	err = update.Apply(file, options)
	if err != nil {
		if rerr := update.RollbackError(err); rerr != nil {
			logger.Debugf("install.updateFile(%s,%s): Failed to rollback from bad update: %v",
				sourcePath, targetPath, rerr)
		}
	}
	return err
}
