package version

import (
	"github.com/urfave/cli/v2"

	"github.com/mintoolkit/mint/pkg/app"
	"github.com/mintoolkit/mint/pkg/app/master/command"
)

const (
	Name  = "version"
	Usage = "Shows mint and docker version information"
	Alias = "v"
)

var CLI = &cli.Command{
	Name:    Name,
	Aliases: []string{Alias},
	Usage:   Usage,
	Action: func(ctx *cli.Context) error {
		gcvalues := command.GlobalFlagValues(ctx)
		xc := app.NewExecutionContext(
			Name,
			gcvalues.QuietCLIMode,
			gcvalues.OutputFormat)

		OnCommand(xc,
			gcvalues.Debug,
			gcvalues.InContainer,
			gcvalues.IsDSImage,
			gcvalues.ClientConfig)

		return nil
	},
}
