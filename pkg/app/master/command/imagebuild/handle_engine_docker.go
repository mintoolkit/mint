package imagebuild

import (
	dockerapi "github.com/fsouza/go-dockerclient"
	log "github.com/sirupsen/logrus"

	"github.com/mintoolkit/mint/pkg/app"
	"github.com/mintoolkit/mint/pkg/app/master/command"
	//"github.com/mintoolkit/mint/pkg/util/jsonutil"
)

// HandleDockerEngine implements support for the Docker container build engine
func HandleDockerEngine(
	logger *log.Entry,
	xc *app.ExecutionContext,
	gparams *command.GenericParams,
	cparams *CommandParams,
	client *dockerapi.Client) {
	logger.Trace("HandleDockerEngine.call")
	defer logger.Trace("HandleDockerEngine.exit")
}
