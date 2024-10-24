package imagebuild

import (
	"path"
	"path/filepath"
	"strings"

	dockerapi "github.com/fsouza/go-dockerclient"
	log "github.com/sirupsen/logrus"

	"github.com/mintoolkit/mint/pkg/app"
	"github.com/mintoolkit/mint/pkg/app/master/command"
	"github.com/mintoolkit/mint/pkg/imagebuilder"
	"github.com/mintoolkit/mint/pkg/imagebuilder/simplebuilder"
	"github.com/mintoolkit/mint/pkg/util/fsutil"
	v "github.com/mintoolkit/mint/pkg/version"
)

// HandleSimpleEngine implements support for the simple built-in container build engine
func HandleSimpleEngine(
	logger *log.Entry,
	xc *app.ExecutionContext,
	gparams *command.GenericParams,
	cparams *CommandParams,
	client *dockerapi.Client) {
	//passing docker client for now (later pass a CRT client), need docker client to get to the base image...
	logger.Trace("HandleSimpleEngine.call")
	defer logger.Trace("HandleSimpleEngine.exit")

	var localExePath string
	var targetExePath string
	if strings.Contains(cparams.ExePath, ":") {
		parts := strings.SplitN(cparams.ExePath, ":", 2)
		localExePath = parts[0]
		targetExePath = parts[1]
	} else {
		localExePath = cparams.ExePath
		targetExePath = path.Join(simplebuilder.DefaultAppDir, filepath.Base(localExePath))
	}

	if !fsutil.Exists(localExePath) || !fsutil.IsRegularFile(localExePath) {
		xc.Out.Info("build.error",
			ovars{
				"status":         "docker.engine.image.build.error",
				"value":          "bad exe path",
				"exe.path":       cparams.ExePath,
				"local.exe.path": localExePath,
			})

		exitCode := 111
		xc.Out.State("exited",
			ovars{
				"exit.code": exitCode,
				"version":   v.Current(),
				"location":  fsutil.ExeDir(),
			})

		xc.Exit(exitCode)
	}

	doShowBuildLogs := true
	builder, err := simplebuilder.New(doShowBuildLogs, false, false)
	options := imagebuilder.SimpleBuildOptions{
		OutputImageTar: cparams.ImageArchiveFile,
		From:           cparams.BaseImage,
		FromTar:        cparams.BaseImageTar,
		Tags:           []string{cparams.ImageName},
		Layers: []imagebuilder.LayerDataInfo{
			{
				Type:   imagebuilder.FileSource,
				Source: cparams.ExePath,
				Params: &imagebuilder.DataParams{
					TargetPath: targetExePath,
				},
				EntrypointLayer: true,
			},
		},
		ImageConfig: &imagebuilder.ImageConfig{
			Architecture: cparams.Architecture,
			Config: imagebuilder.RunConfig{
				Entrypoint: []string{targetExePath},
			},
		},
	}

	if cparams.BaseImageWithCerts {
		options.From = simplebuilder.BaseImageWithCerts
	}

	bresult, err := builder.Build(options)
	if err != nil {
		xc.Out.Info("build.error",
			ovars{
				"status": "docker.engine.image.build.error",
				"value":  err,
			})

		exitCode := 721
		xc.Out.State("exited",
			ovars{
				"exit.code": exitCode,
				"version":   v.Current(),
				"location":  fsutil.ExeDir(),
			})

		xc.Exit(exitCode)
	}

	xc.Out.State("docker.engine.image.build.completed")
	xc.Out.Info("output.image",
		ovars{
			"name":   bresult.Name,
			"id":     bresult.ID,
			"digest": bresult.Digest,
		})
}
