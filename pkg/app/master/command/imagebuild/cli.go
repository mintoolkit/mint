package imagebuild

import (
	"fmt"
	"os"
	"strings"

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
	Engine           string                 `json:"engine,omitempty"`
	EngineEndpoint   string                 `json:"engine_endpoint,omitempty"`
	EngineToken      string                 `json:"engine_token,omitempty"`
	EngineNamespace  string                 `json:"engine_namespace,omitempty"`
	ImageName        string                 `json:"image_name,omitempty"`
	ImageArchiveFile string                 `json:"image_archive_file,omitempty"`
	Runtime          string                 `json:"runtime,omitempty"` //runtime where to load the created image
	Dockerfile       string                 `json:"dockerfile,omitempty"`
	ContextDir       string                 `json:"context_dir,omitempty"`
	BuildArgs        []imagebuilder.NVParam `json:"build_args,omitempty"`
	Labels           map[string]string      `json:"labels,omitempty"`
	Architecture     string                 `json:"architecture,omitempty"`
}

var ImageBuildFlags = useAllFlags()

var CLI = &cli.Command{
	Name:    Name,
	Aliases: []string{Alias},
	Usage:   Usage,
	Flags:   ImageBuildFlags,
	Action: func(ctx *cli.Context) error {
		gcvalues, ok := command.CLIContextGet(ctx.Context, command.GlobalParams).(*command.GenericParams)
		if !ok || gcvalues == nil {
			return command.ErrNoGlobalParams
		}

		xc := app.NewExecutionContext(
			Name,
			gcvalues.QuietCLIMode,
			gcvalues.OutputFormat)

		cparams := &CommandParams{
			Engine:           ctx.String(FlagEngine),
			EngineEndpoint:   ctx.String(FlagEngineEndpoint),
			EngineToken:      ctx.String(FlagEngineToken),
			EngineNamespace:  ctx.String(FlagEngineNamespace),
			ImageName:        ctx.String(FlagImageName),
			ImageArchiveFile: ctx.String(FlagImageArchiveFile),
			Dockerfile:       ctx.String(FlagDockerfile),
			ContextDir:       ctx.String(FlagContextDir),
			Runtime:          ctx.String(FlagRuntimeLoad),
			Architecture:     ctx.String(FlagArchitecture),
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
			return command.ErrBadParamValue
		}

		if cparams.Dockerfile == "" {
			cparams.Dockerfile = DefaultDockerfilePath
		}

		if !fsutil.Exists(cparams.Dockerfile) {
			return command.ErrBadParamValue
		}

		if !fsutil.DirExists(cparams.ContextDir) {
			return command.ErrBadParamValue
		}

		if cparams.Architecture == "" {
			cparams.Architecture = GetDefaultBuildArch()
		}

		if !IsArchValue(cparams.Architecture) {
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
