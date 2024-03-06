package appbom

import (
	"github.com/mintoolkit/mint/pkg/app"
	"github.com/mintoolkit/mint/pkg/app/master/command"

	"github.com/urfave/cli/v2"
)

const (
	Name  = "appbom"
	Usage = "Show application BOM"
	Alias = "a"
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

		OnCommand(
			xc,
			gcvalues)

		return nil
	},
}
