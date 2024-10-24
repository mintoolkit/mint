package imagebuild

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	log "github.com/sirupsen/logrus"
	"github.com/urfave/cli/v2"

	"github.com/mintoolkit/mint/pkg/app"
	"github.com/mintoolkit/mint/pkg/app/master/command"
	cmd "github.com/mintoolkit/mint/pkg/command"
	"github.com/mintoolkit/mint/pkg/imagebuilder"
	"github.com/mintoolkit/mint/pkg/util/fsutil"
)

const (
	Name  = string(cmd.ImageBuild)
	Usage = "Build container image using selected build engine"
	Alias = "bi"
)

type CommandParams struct {
	Engine             string                 `json:"engine,omitempty"`
	EngineEndpoint     string                 `json:"engine_endpoint,omitempty"`
	EngineToken        string                 `json:"engine_token,omitempty"`
	EngineNamespace    string                 `json:"engine_namespace,omitempty"`
	ImageName          string                 `json:"image_name,omitempty"`
	ImageArchiveFile   string                 `json:"image_archive_file,omitempty"`
	Runtime            string                 `json:"runtime,omitempty"` //runtime where to load the created image
	Dockerfile         string                 `json:"dockerfile,omitempty"`
	ContextDir         string                 `json:"context_dir,omitempty"`
	BuildArgs          []imagebuilder.NVParam `json:"build_args,omitempty"`
	Labels             map[string]string      `json:"labels,omitempty"`
	Architecture       string                 `json:"architecture,omitempty"`
	BaseImage          string                 `json:"base_image,omitempty"`
	BaseImageTar       string                 `json:"base_image_tar,omitempty"`
	BaseImageWithCerts bool                   `json:"base_image_with_certs,omitempty"`
	ExePath            string                 `json:"exe_path,omitempty"`
}

var ImageBuildFlags = useAllFlags()

var CLI = &cli.Command{
	Name:    Name,
	Aliases: []string{Alias},
	Usage:   Usage,
	Flags:   ImageBuildFlags,
	Action: func(ctx *cli.Context) error {
		logger := log.WithFields(log.Fields{"app": command.AppName, "cmd": Name, "op": "cli.Action"})

		gcvalues, ok := command.CLIContextGet(ctx.Context, command.GlobalParams).(*command.GenericParams)
		if !ok || gcvalues == nil {
			logger.Error("no gcvalues")
			return command.ErrNoGlobalParams
		}

		xc := app.NewExecutionContext(
			Name,
			gcvalues.QuietCLIMode,
			gcvalues.OutputFormat)

		cparams := &CommandParams{
			Engine:             ctx.String(FlagEngine),
			EngineEndpoint:     ctx.String(FlagEngineEndpoint),
			EngineToken:        ctx.String(FlagEngineToken),
			EngineNamespace:    ctx.String(FlagEngineNamespace),
			ImageName:          ctx.String(FlagImageName),
			ImageArchiveFile:   ctx.String(FlagImageArchiveFile),
			Dockerfile:         ctx.String(FlagDockerfile),
			ContextDir:         ctx.String(FlagContextDir),
			Runtime:            ctx.String(FlagRuntimeLoad),
			Architecture:       ctx.String(FlagArchitecture),
			BaseImage:          ctx.String(FlagBase),
			BaseImageTar:       ctx.String(FlagBaseTar),
			BaseImageWithCerts: ctx.Bool(FlagBaseWithCerts),
			ExePath:            ctx.String(FlagExePath),
			Labels:             map[string]string{},
		}

		cboBuildArgs := command.ParseKVParams(ctx.StringSlice(FlagBuildArg))
		for _, val := range cboBuildArgs {
			cparams.BuildArgs = append(cparams.BuildArgs,
				imagebuilder.NVParam{Name: val.Name, Value: val.Value})
		}

		kvLabels := command.ParseKVParams(ctx.StringSlice(FlagLabel))
		for _, kv := range kvLabels {
			cparams.Labels[kv.Name] = kv.Value
		}

		engineProps, found := BuildEngines[cparams.Engine]
		if !found {
			logger.Errorf("engine not found - %s", cparams.Engine)
			return command.ErrBadParamValue
		}

		if cparams.Dockerfile == "" {
			cparams.Dockerfile = DefaultDockerfilePath
		}

		if !fsutil.DirExists(cparams.ContextDir) {
			logger.Errorf("context dir not found - %s", cparams.ContextDir)
			return command.ErrBadParamValue
		}

		switch cparams.Engine {
		case BuildkitBuildEngine, DepotBuildEngine:
			if !fsutil.Exists(cparams.Dockerfile) {
				logger.Errorf("Dockerfile not found - '%s'", cparams.Dockerfile)
				return command.ErrBadParamValue
			}
		case SimpleBuildEngine:
			if cparams.ExePath == "" && !fsutil.Exists(cparams.Dockerfile) {
				logger.Errorf("no exe-path and no Dockerfile - '%s'", cparams.Dockerfile)
				return command.ErrBadParamValue
			}
		default:
			fullDockerfilePath := filepath.Join(cparams.ContextDir, cparams.Dockerfile)
			if !fsutil.Exists(fullDockerfilePath) {
				logger.Errorf("Dockerfile not found - '%s' ('%s')", cparams.Dockerfile, fullDockerfilePath)
				return command.ErrBadParamValue
			}
		}

		if cparams.Architecture == "" {
			cparams.Architecture = GetDefaultBuildArch()
		}

		if !IsArchValue(cparams.Architecture) {
			logger.Errorf("architecture not supported - %s", cparams.Architecture)
			return command.ErrBadParamValue
		}

		if engineProps.TokenRequired &&
			cparams.EngineToken == "" &&
			engineProps.NativeTokenEnvVar != "" {
			cparams.EngineToken = os.Getenv(engineProps.NativeTokenEnvVar)
		}

		if engineProps.NamespaceRequired &&
			cparams.EngineNamespace == "" &&
			engineProps.NativeNamespaceEnvVar != "" {
			cparams.EngineNamespace = os.Getenv(engineProps.NativeNamespaceEnvVar)
		}

		if engineProps.TokenRequired && cparams.EngineToken == "" {
			return fmt.Errorf("missing flag value - %s", FlagEngineToken)
		}

		if engineProps.NamespaceRequired && cparams.EngineNamespace == "" {
			return fmt.Errorf("missing flag value - %s", FlagEngineNamespace)
		}

		if engineProps.EndpointRequired && cparams.EngineEndpoint == "" {
			return fmt.Errorf("missing flag value - %s", FlagEngineEndpoint)
		}

		if cparams.ImageName == "" {
			return fmt.Errorf("missing flag value - %s", FlagImageName)
		}

		if cparams.ImageArchiveFile == "" {
			return fmt.Errorf("missing flag value - %s", FlagImageArchiveFile)
		}

		if !strings.Contains(cparams.ImageName, ":") {
			cparams.ImageName = fmt.Sprintf("%s:latest", cparams.ImageName)
		}

		OnCommand(xc, gcvalues, cparams)
		return nil
	},
}
