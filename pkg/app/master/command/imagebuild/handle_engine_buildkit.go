package imagebuild

import (
	"context"
	"encoding/base64"
	"fmt"
	"golang.org/x/sync/errgroup"
	"io"
	"os"
	"path/filepath"
	"strings"

	log "github.com/sirupsen/logrus"

	"github.com/mintoolkit/mint/pkg/app"
	"github.com/mintoolkit/mint/pkg/app/master/command"

	"github.com/moby/buildkit/client"
	//dockerfile "github.com/moby/buildkit/frontend/dockerfile/builder"
	"github.com/moby/buildkit/util/progress/progressui"
)

// HandleBuildkitEngine implements support for the Buildkit container build engine
func HandleBuildkitEngine(
	logger *log.Entry,
	xc *app.ExecutionContext,
	gparams *command.GenericParams,
	cparams *CommandParams) {
	logger.Trace("HandleBuildkitEngine.call")
	defer logger.Trace("HandleBuildkitEngine.exit")
	ctx := context.Background()

	logger.Trace("buildkit.client.New")
	bclient, berr := client.New(ctx, cparams.EngineEndpoint)
	xc.FailOn(berr)

	berr = buildkitBuildImage(logger, xc, cparams, ctx, bclient)
	xc.FailOn(berr)
}

func buildkitBuildImage(
	logger *log.Entry,
	xc *app.ExecutionContext,
	cparams *CommandParams,
	ctx context.Context,
	bclient *client.Client) error {
	logger.Trace("buildkitBuildImage.call")
	defer logger.Trace("buildkitBuildImage.exit")

	ch := make(chan *client.SolveStatus)
	eg, ctx := errgroup.WithContext(ctx)

	outputFile, err := os.Create(cparams.ImageArchiveFile)
	if err != nil {
		return fmt.Errorf("failed to create output file: %v", err)
	}
	defer outputFile.Close()

	output := func(map[string]string) (io.WriteCloser, error) {
		return outputFile, nil
	}

	if cparams.Dockerfile == "" {
		cparams.Dockerfile = DefaultDockerfilePath
	}

	dockerfileDir := filepath.Dir(cparams.Dockerfile)
	dockerfileName := filepath.Base(cparams.Dockerfile)
	if cparams.ContextDir == "" {
		cparams.ContextDir = dockerfileDir
	}

	opts := client.SolveOpt{
		Frontend: "dockerfile.v0",
		FrontendAttrs: map[string]string{
			"platform": fmt.Sprintf("linux/%s", cparams.Architecture),
			"filename": dockerfileName,
		},
		LocalDirs: map[string]string{
			"context":    cparams.ContextDir,
			"dockerfile": dockerfileDir,
		},
		Internal: true,
		Exports: []client.ExportEntry{
			{
				Type:   client.ExporterDocker,
				Output: output,
				Attrs: map[string]string{
					"name": cparams.ImageName,
				},
			},
		},
	}

	for _, kvStr := range cparams.BuildArgs {
		kv := strings.SplitN(kvStr, "=", 2)
		if len(kv) != 2 {
			logger.Debugf("malformed build arg: %s", kvStr)
			continue
		}

		opts.FrontendAttrs["build-arg:"+kv[0]] = kv[1]
	}

	var res *client.SolveResponse
	eg.Go(func() error {
		//res, err = bclient.Build(ctx, opts, "", dockerfile.Build, ch)
		res, err = bclient.Solve(ctx, nil, opts, ch)
		return err
	})

	eg.Go(func() error {
		display, err := progressui.NewDisplay(os.Stdout, progressui.AutoMode) //progressui.TtyMode
		if err != nil {
			display, err = progressui.NewDisplay(os.Stdout, progressui.PlainMode)
			if err != nil {
				return err
			}
		}

		_, err = display.UpdateFrom(context.TODO(), ch)
		return err
	})

	if err := eg.Wait(); err != nil {
		return err
	}

	xc.Out.Info("imagebuild.results",
		ovars{
			"engine":                              cparams.Engine,
			"image.name":                          cparams.ImageName,
			"image.archive.file":                  cparams.ImageArchiveFile,
			"architecture":                        cparams.Architecture,
			"result.image.name":                   res.ExporterResponse["image.name"],
			"result.containerimage.config.digest": res.ExporterResponse["containerimage.config.digest"],
			"result.containerimage.digest":        res.ExporterResponse["containerimage.digest"],
		})

	//response fields:
	//"image.name"
	//"containerimage.descriptor" - base64
	//"containerimage.config.digest"
	//"containerimage.digest"
	for k, v := range res.ExporterResponse {
		//some fields are base64 encoded, but not all
		decoded, err := base64.StdEncoding.DecodeString(v)
		if err == nil {
			v = string(decoded)
		}

		logger.Debugf("Exporter response: %s='%s'\n", k, string(v))
	}

	return nil
}
