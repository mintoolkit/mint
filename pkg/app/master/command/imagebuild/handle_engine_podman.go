package imagebuild

import (
	"context"

	log "github.com/sirupsen/logrus"

	"github.com/mintoolkit/mint/pkg/app"
	"github.com/mintoolkit/mint/pkg/app/master/command"
	//"github.com/mintoolkit/mint/pkg/util/jsonutil"
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
}
