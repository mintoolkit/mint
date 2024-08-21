package images

import (
	"github.com/urfave/cli/v2"

	"github.com/mintoolkit/mint/pkg/app"
	"github.com/mintoolkit/mint/pkg/app/master/command"
	cmd "github.com/mintoolkit/mint/pkg/command"
)

const (
	Name  = string(cmd.Images)
	Usage = "Get information about container images"
	Alias = "i"
)

type CommandParams struct {
	Runtime string `json:"runtime,omitempty"`
	Filter  string `json:"filter,omitempty"`
}

var ImagesFlags = []cli.Flag{
	command.Cflag(command.FlagRuntime),
	cflag(FlagFilter),
}

//todo soon: add a lot of useful filtering flags
// (to show new images from last hour, to show images in use, by size, with details, etc)

var CLI = &cli.Command{
	Name:    Name,
	Aliases: []string{Alias},
	Usage:   Usage,
	Flags:   ImagesFlags,
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
			Runtime: ctx.String(command.FlagRuntime),
			Filter:  ctx.String(FlagFilter),
		}

		OnCommand(xc, gcvalues, cparams)
		return nil
	},
}
