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
	Runtime   string `json:"runtime,omitempty"`
	Filter    string `json:"filter,omitempty"`
	TUI       bool   `json:"tui,omitempty"`
	GlobalTUI bool   `json:"globalTui,omitempty"`
}

var ImagesFlags = []cli.Flag{
	command.Cflag(command.FlagRuntime),
	cflag(FlagFilter),
	cflag(FlagTUI),
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

		tuiMode := ctx.Bool(FlagTUI)
		cparams := &CommandParams{
			Runtime: ctx.String(command.FlagRuntime),
			Filter:  ctx.String(FlagFilter),
			TUI:     tuiMode,
		}

		quietLogs := tuiMode || gcvalues.QuietCLIMode

		xc := app.NewExecutionContext(
			Name,
			quietLogs,
			gcvalues.OutputFormat)

		OnCommand(xc, gcvalues, cparams)
		return nil
	},
}
