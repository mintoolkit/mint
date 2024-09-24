package standardbuilder

import (
	"bytes"
	"fmt"
	"path/filepath"
	"strings"

	docker "github.com/fsouza/go-dockerclient"

	"github.com/mintoolkit/mint/pkg/imagebuilder"
	"github.com/mintoolkit/mint/pkg/util/fsutil"
)

const (
	Name = "standard.container.build.engine"
)

// Engine is the standard container build engine
type Engine struct {
	pclient       *docker.Client
	showBuildLogs bool
	buildLog      bytes.Buffer
	//pushToDaemon   bool
	//pushToRegistry bool
}

// New creates new Engine instances
func New(
	providerClient *docker.Client,
	showBuildLogs bool,
	//pushToDaemon bool,
	//pushToRegistry bool,
) (*Engine, error) {

	engine := &Engine{
		pclient:       providerClient,
		showBuildLogs: showBuildLogs,
		//pushToDaemon:   pushToDaemon,
		//pushToRegistry: pushToRegistry,
	}

	return engine, nil
}

func (ref *Engine) Name() string {
	return Name
}

func (ref *Engine) BuildLog() string {
	return ref.buildLog.String()
}

func (ref *Engine) Build(options imagebuilder.DockerfileBuildOptions) error {
	if len(options.Labels) > 0 {
		//Docker has a limit on how long the labels can be
		labels := map[string]string{}
		for k, v := range options.Labels {
			lineLen := len(k) + len(v) + 7
			if lineLen > 65535 {
				//TODO: improve JSON data splitting
				valueLen := len(v)
				parts := valueLen / 50000
				parts++
				offset := 0
				for i := 0; i < parts && offset < valueLen; i++ {
					chunkSize := 50000
					if (offset + chunkSize) > valueLen {
						chunkSize = valueLen - offset
					}
					value := v[offset:(offset + chunkSize)]
					offset += chunkSize
					key := fmt.Sprintf("%s.%d", k, i)
					labels[key] = value
				}
			} else {
				labels[k] = v
			}
		}
		options.Labels = labels
	}

	//not using options.CacheTo in this image builder...
	buildOptions := docker.BuildImageOptions{
		Dockerfile: options.Dockerfile,
		Target:     options.Target,
		Name:       options.ImagePath,

		NetworkMode:    options.NetworkMode,
		ExtraHosts:     options.ExtraHosts,
		CacheFrom:      options.CacheFrom,
		Labels:         options.Labels,
		RmTmpContainer: true,
	}

	if len(options.Platforms) > 0 {
		buildOptions.Platform = strings.Join(options.Platforms, ",")
	}

	for _, nv := range options.BuildArgs {
		buildOptions.BuildArgs = append(buildOptions.BuildArgs, docker.BuildArg{Name: nv.Name, Value: nv.Value})
	}

	if strings.HasPrefix(options.BuildContext, "http://") ||
		strings.HasPrefix(options.BuildContext, "https://") {
		buildOptions.Remote = options.BuildContext
	} else {
		if exists := fsutil.DirExists(options.BuildContext); exists {
			buildOptions.ContextDir = options.BuildContext
		} else {
			return imagebuilder.ErrInvalidContextDir
		}
	}

	if !fsutil.Exists(buildOptions.Dockerfile) || !fsutil.IsRegularFile(buildOptions.Dockerfile) {
		//a slightly hacky behavior using the build context directory if the dockerfile flag doesn't include a usable path
		fullDockerfileName := filepath.Join(buildOptions.ContextDir, buildOptions.Dockerfile)
		if !fsutil.Exists(fullDockerfileName) || !fsutil.IsRegularFile(fullDockerfileName) {
			return fmt.Errorf("invalid dockerfile reference - %s", fullDockerfileName)
		}

		buildOptions.Dockerfile = fullDockerfileName
	}

	if options.OutputStream != nil {
		buildOptions.OutputStream = options.OutputStream
	} else if ref.showBuildLogs {
		ref.buildLog.Reset()
		buildOptions.OutputStream = &ref.buildLog
	}

	return ref.pclient.BuildImage(buildOptions)
}
