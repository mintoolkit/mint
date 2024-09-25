package standardbuilder

import (
	"github.com/mintoolkit/mint/pkg/crt"
	"github.com/mintoolkit/mint/pkg/imagebuilder"
)

const (
	Name = "standard.container.build.engine"
)

// Engine is the standard container build engine
type Engine struct {
	//pclient       *docker.Client
	pclient crt.ImageBuilderAPIClient
	//showBuildLogs bool
	//buildLog      bytes.Buffer

	//pushToDaemon   bool
	//pushToRegistry bool
}

// New creates new Engine instances
func New(
	client crt.ImageBuilderAPIClient,
	//providerClient *docker.Client,
	//showBuildLogs bool,
	//pushToDaemon bool,
	//pushToRegistry bool,
) (*Engine, error) {

	engine := &Engine{
		pclient: client,
		//showBuildLogs: showBuildLogs,
		//pushToDaemon:   pushToDaemon,
		//pushToRegistry: pushToRegistry,
	}

	return engine, nil
}

func (ref *Engine) Name() string {
	return Name
}

func (ref *Engine) BuildLog() string {
	return ref.pclient.BuildOutputLog()
}

func (ref *Engine) Build(options imagebuilder.DockerfileBuildOptions) error {
	return ref.pclient.BuildImage(options)
}
