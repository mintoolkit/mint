package app

import (
	"fmt"
	"os"
	"strings"

	log "github.com/sirupsen/logrus"
	"github.com/urfave/cli/v2"

	"github.com/mintoolkit/mint/pkg/app"
	"github.com/mintoolkit/mint/pkg/app/master/command"
	"github.com/mintoolkit/mint/pkg/app/master/command/build"
	//"github.com/mintoolkit/mint/pkg/app/master/command/containerize"
	//"github.com/mintoolkit/mint/pkg/app/master/command/convert"
	"github.com/mintoolkit/mint/pkg/app/master/command/debug"
	"github.com/mintoolkit/mint/pkg/app/master/command/dockerclipm"
	//"github.com/mintoolkit/mint/pkg/app/master/command/edit"
	"github.com/mintoolkit/mint/pkg/app/master/command/help"
	"github.com/mintoolkit/mint/pkg/app/master/command/images"
	"github.com/mintoolkit/mint/pkg/app/master/command/lint"
	"github.com/mintoolkit/mint/pkg/app/master/command/merge"
	"github.com/mintoolkit/mint/pkg/app/master/command/probe"
	"github.com/mintoolkit/mint/pkg/app/master/command/profile"
	"github.com/mintoolkit/mint/pkg/app/master/command/registry"
	"github.com/mintoolkit/mint/pkg/app/master/command/run"
	//"github.com/mintoolkit/mint/pkg/app/master/command/server"
	appcmd "github.com/mintoolkit/mint/pkg/app/master/command/app"
	"github.com/mintoolkit/mint/pkg/app/master/command/vulnerability"
	"github.com/mintoolkit/mint/pkg/app/master/command/xray"
	"github.com/mintoolkit/mint/pkg/app/master/config"
	"github.com/mintoolkit/mint/pkg/system"
	"github.com/mintoolkit/mint/pkg/util/fsutil"
	v "github.com/mintoolkit/mint/pkg/version"
)

// Main/driver app CLI constants
const (
	AppName  = "mint"
	AppUsage = "inspect, minify, customize, secure and debug your containers!"
)

func registerCommands() {
	//registering commands explicitly instead of relying on init()
	//also get to control the order of the commands in the interactive prompt

	debug.RegisterCommand()
	build.RegisterCommand()
	xray.RegisterCommand()
	lint.RegisterCommand()
	merge.RegisterCommand()
	images.RegisterCommand()
	registry.RegisterCommand()
	vulnerability.RegisterCommand()
	profile.RegisterCommand()
	help.RegisterCommand()
	//edit.RegisterCommand() - not doing anything yet
	probe.RegisterCommand()
	//convert.RegisterCommand() - not doing anything yet
	run.RegisterCommand()
	//server.RegisterCommand() - not doing anything yet
	//containerize.RegisterCommand() - not doing anything yet
	dockerclipm.RegisterCommand()
	appcmd.RegisterCommand()
}

func newCLI() *cli.App {
	registerCommands()

	doShowCommunityInfo := true
	cliApp := cli.NewApp()
	cliApp.Version = v.Current()
	cliApp.Name = AppName
	cliApp.Usage = AppUsage
	cliApp.CommandNotFound = func(ctx *cli.Context, command string) {
		fmt.Printf("unknown command - %v \n\n", command)
		cli.ShowAppHelp(ctx)
	}

	cliApp.Flags = command.GlobalFlags()

	cliApp.Before = func(ctx *cli.Context) error {
		gparams := command.GlobalFlagValues(ctx)

		appParams, err := config.NewAppOptionsFromFile(fsutil.ResolveImageStateBasePath(gparams.StatePath))
		if err != nil {
			log.Errorf("config.NewAppOptionsFromFile error - %v", err)
			return err
		}

		gparams = command.UpdateGlobalFlagValues(appParams, gparams)

		ctx.Context = command.CLIContextSave(ctx.Context, command.GlobalParams, gparams)
		ctx.Context = command.CLIContextSave(ctx.Context, command.AppParams, appParams)

		if gparams.NoColor {
			app.NoColor()
		}

		if gparams.Debug {
			log.SetLevel(log.DebugLevel)
		} else {
			if gparams.Verbose {
				log.SetLevel(log.InfoLevel)
			} else {
				logLevel := log.WarnLevel
				switch gparams.LogLevel {
				case "trace":
					logLevel = log.TraceLevel
				case "debug":
					logLevel = log.DebugLevel
				case "info":
					logLevel = log.InfoLevel
				case "warn":
					logLevel = log.WarnLevel
				case "error":
					logLevel = log.ErrorLevel
				case "fatal":
					logLevel = log.FatalLevel
				case "panic":
					logLevel = log.PanicLevel
				default:
					log.Fatalf("unknown log-level %q", gparams.LogLevel)
				}

				log.SetLevel(logLevel)
			}
		}

		if gparams.Log != "" {
			f, err := os.Create(gparams.Log)
			if err != nil {
				return err
			}
			log.SetOutput(f)
		}

		switch gparams.LogFormat {
		case "text":
			log.SetFormatter(&log.TextFormatter{DisableColors: true})
		case "json":
			log.SetFormatter(new(log.JSONFormatter))
		default:
			log.Fatalf("unknown log-format %q", gparams.LogFormat)
		}

		log.Debugf("sysinfo => %#v", system.GetSystemInfo())

		//NOTE: not displaying the community info here to reduce noise
		//tmp hack
		//if !strings.Contains(strings.Join(os.Args, " "), " docker-cli-plugin-metadata") {
		//   app.ShowCommunityInfo(gparams.OutputFormat)
		//}
		return nil
	}

	cliApp.After = func(ctx *cli.Context) error {
		//todo: get already fetched gcvalues from ctx.Context
		gcvalues := command.GlobalFlagValues(ctx)

		if gcvalues.QuietCLIMode {
			return nil
		}

		//tmp hack
		if !strings.Contains(strings.Join(os.Args, " "), " docker-cli-plugin-metadata") {
			if doShowCommunityInfo {
				app.ShowCommunityInfo(ctx.String(command.FlagOutputFormat))
			}
		}
		return nil
	}

	cliApp.Action = func(ctx *cli.Context) error {
		//todo: get already fetched gcvalues from ctx.Context
		gcvalues := command.GlobalFlagValues(ctx)

		//disable community info in interactive mode (too noisy)
		doShowCommunityInfo = false
		ia := command.NewInteractiveApp(cliApp, gcvalues)
		ia.Run()
		return nil
	}

	cliApp.Commands = command.GetCommands()
	return cliApp
}
