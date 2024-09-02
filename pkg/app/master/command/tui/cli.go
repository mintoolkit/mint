package tui

import (
	tui "github.com/mintoolkit/mint/pkg/app/master/tui"
	"github.com/mintoolkit/mint/pkg/app/master/tui/models"
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
		m := models.InitialModel()
		tui.RunTUI(m)
		return nil
	},
}
