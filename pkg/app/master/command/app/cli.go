package app

import (
	"fmt"

	"github.com/urfave/cli/v2"

	a "github.com/mintoolkit/mint/pkg/app"
	"github.com/mintoolkit/mint/pkg/app/master/command"
)

const (
	Name  = "app"
	Usage = "Execute app management, maintenance, debugging and query operations"
	Alias = "a"

	BomCmdName      = "bom"
	BomCmdNameUsage = "Show application BOM"

	VersionCmdName      = "version"
	VersionCmdNameUsage = "Shows mint and container runtime version information"

	RSVCmdName      = "remove-sensor-volumes"
	RSVCmdNameUsage = "Remove all available sensor volumes"

	UpdateCmdName      = "update"
	UpdateCmdNameUsage = "Update app to the latest version"

	InstallCmdName      = "install"
	InstallCmdNameUsage = "Copy Mint binaries from current location to the standard user app bin directory (/usr/local/bin)"

	InstallDCPCmdName      = "install-docker-cli-plugin"
	InstallDCPCmdNameUsage = "Install Docker CLI plugin for Mint"
)

type ovars = a.OutVars

func fullCmdName(subCmdName string) string {
	return fmt.Sprintf("%s.%s", Name, subCmdName)
}

type CommonCommandParams struct {
}

func CommonCommandFlagValues(ctx *cli.Context) (*CommonCommandParams, error) {
	values := &CommonCommandParams{}

	return values, nil
}

type InstallCommandParams struct {
	*CommonCommandParams
}

func InstallCommandFlagValues(ctx *cli.Context) (*InstallCommandParams, error) {
	common, err := CommonCommandFlagValues(ctx)
	if err != nil {
		return nil, err
	}

	values := &InstallCommandParams{
		CommonCommandParams: common,
	}

	return values, nil
}

type UpdateCommandParams struct {
	*CommonCommandParams
	ShowProgress bool
}

func UpdateCommandFlagValues(ctx *cli.Context) (*UpdateCommandParams, error) {
	common, err := CommonCommandFlagValues(ctx)
	if err != nil {
		return nil, err
	}

	values := &UpdateCommandParams{
		CommonCommandParams: common,
		ShowProgress:        ctx.Bool(command.FlagShowProgress),
	}

	return values, nil
}

var CLI = &cli.Command{
	Name:    Name,
	Aliases: []string{Alias},
	Usage:   Usage,
	Flags:   []cli.Flag{},
	Subcommands: []*cli.Command{
		{
			Name:  BomCmdName,
			Usage: BomCmdNameUsage,
			Flags: []cli.Flag{},
			Action: func(ctx *cli.Context) error {
				gcvalues, ok := command.CLIContextGet(ctx.Context, command.GlobalParams).(*command.GenericParams)
				if !ok || gcvalues == nil {
					return command.ErrNoGlobalParams
				}

				xc := a.NewExecutionContext(
					fullCmdName(BomCmdName),
					gcvalues.QuietCLIMode,
					gcvalues.OutputFormat)

				OnBomCommand(xc, gcvalues)
				return nil
			},
		},
		{
			Name:  VersionCmdName,
			Usage: VersionCmdNameUsage,
			Flags: []cli.Flag{},
			Action: func(ctx *cli.Context) error {
				gcvalues, ok := command.CLIContextGet(ctx.Context, command.GlobalParams).(*command.GenericParams)
				if !ok || gcvalues == nil {
					return command.ErrNoGlobalParams
				}

				xc := a.NewExecutionContext(
					fullCmdName(VersionCmdName),
					gcvalues.QuietCLIMode,
					gcvalues.OutputFormat)

				OnVersionCommand(xc, gcvalues)
				return nil
			},
		},
		{
			Name:  RSVCmdName,
			Usage: RSVCmdNameUsage,
			Flags: []cli.Flag{},
			Action: func(ctx *cli.Context) error {
				gcvalues, ok := command.CLIContextGet(ctx.Context, command.GlobalParams).(*command.GenericParams)
				if !ok || gcvalues == nil {
					return command.ErrNoGlobalParams
				}

				xc := a.NewExecutionContext(
					fullCmdName(RSVCmdName),
					gcvalues.QuietCLIMode,
					gcvalues.OutputFormat)

				OnRsvCommand(xc, gcvalues)
				return nil
			},
		},
		{
			Name:  UpdateCmdName,
			Usage: UpdateCmdNameUsage,
			Flags: []cli.Flag{
				initFlagShowProgress(),
			},
			Action: func(ctx *cli.Context) error {
				gcvalues, ok := command.CLIContextGet(ctx.Context, command.GlobalParams).(*command.GenericParams)
				if !ok || gcvalues == nil {
					return command.ErrNoGlobalParams
				}

				xc := a.NewExecutionContext(
					fullCmdName(UpdateCmdName),
					gcvalues.QuietCLIMode,
					gcvalues.OutputFormat)

				cparams, err := UpdateCommandFlagValues(ctx)
				xc.FailOn(err)

				OnUpdateCommand(xc, gcvalues, cparams)
				return nil
			},
		},
		{
			Name:  InstallCmdName,
			Usage: InstallCmdNameUsage,
			Flags: []cli.Flag{},
			Action: func(ctx *cli.Context) error {
				gcvalues, ok := command.CLIContextGet(ctx.Context, command.GlobalParams).(*command.GenericParams)
				if !ok || gcvalues == nil {
					return command.ErrNoGlobalParams
				}

				xc := a.NewExecutionContext(
					fullCmdName(InstallCmdName),
					gcvalues.QuietCLIMode,
					gcvalues.OutputFormat)

				cparams, err := InstallCommandFlagValues(ctx)
				xc.FailOn(err)

				OnInstallCommand(xc, gcvalues, cparams)
				return nil
			},
		},
		{
			Name:  InstallDCPCmdName,
			Usage: InstallDCPCmdNameUsage,
			Flags: []cli.Flag{},
			Action: func(ctx *cli.Context) error {
				gcvalues, ok := command.CLIContextGet(ctx.Context, command.GlobalParams).(*command.GenericParams)
				if !ok || gcvalues == nil {
					return command.ErrNoGlobalParams
				}

				xc := a.NewExecutionContext(
					fullCmdName(InstallDCPCmdName),
					gcvalues.QuietCLIMode,
					gcvalues.OutputFormat)

				OnInstallDCPCommand(xc, gcvalues)
				return nil
			},
		},
	},
}
