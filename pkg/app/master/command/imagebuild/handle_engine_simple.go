package imagebuild

import (
	log "github.com/sirupsen/logrus"

	"github.com/mintoolkit/mint/pkg/app"
	"github.com/mintoolkit/mint/pkg/app/master/command"
	//"github.com/mintoolkit/mint/pkg/util/jsonutil"
)

// HandleSimpleEngine implements support for the simple built-in container build engine
func HandleSimpleEngine(
	logger *log.Entry,
	xc *app.ExecutionContext,
	gparams *command.GenericParams,
	cparams *CommandParams) {
	logger.Trace("HandleSimpleEngine.call")
	defer logger.Trace("HandleSimpleEngine.exit")
}
