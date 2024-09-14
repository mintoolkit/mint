package tui

import (
	"github.com/mintoolkit/mint/pkg/app/master/command"
	tui "github.com/mintoolkit/mint/pkg/app/master/tui"
	"github.com/mintoolkit/mint/pkg/app/master/tui/home"
	cmd "github.com/mintoolkit/mint/pkg/command"
	"github.com/urfave/cli/v2"
)

const (
	Name  = string(cmd.Tui)
	Usage = "Open a terminal user interface"
	Alias = "t"
)

var CLI = &cli.Command{
	Name:    Name,
	Aliases: []string{Alias},
	Usage:   Usage,
	Action: func(ctx *cli.Context) error {
		gcvalues, ok := command.CLIContextGet(ctx.Context, command.GlobalParams).(*command.GenericParams)
		if !ok || gcvalues == nil {
			return command.ErrNoGlobalParams
		}

		m, _ := home.InitialModel(gcvalues)
		tui.RunTUI(m, false)
		return nil
	},
}
