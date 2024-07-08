package app

import (
	log "github.com/sirupsen/logrus"

	a "github.com/mintoolkit/mint/pkg/app"
	"github.com/mintoolkit/mint/pkg/app/master/command"

	"github.com/mintoolkit/mint/pkg/app/master/update"
)

// OnUpdateCommand implements the 'app update' command
func OnUpdateCommand(
	xc *a.ExecutionContext,
	gparams *command.GenericParams,
	cparams *UpdateCommandParams) {
	cmdName := fullCmdName(InstallCmdName)
	logger := log.WithFields(log.Fields{
		"app": command.AppName,
		"cmd": cmdName,
		"sub": UpdateCmdName,
	})

	logger.Trace("call")
	defer logger.Trace("exit")

	update.Run(gparams.Debug, gparams.StatePath, gparams.InContainer, gparams.IsDSImage, cparams.ShowProgress)
}
