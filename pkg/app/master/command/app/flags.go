package app

import (
	"fmt"
	"runtime"

	log "github.com/sirupsen/logrus"
	"github.com/urfave/cli/v2"

	"github.com/mintoolkit/mint/pkg/app/master/command"
)

// App command flags
const (
	FlagX      = "x"
	FlagXUsage = "usage"
)

var Flags = map[string]cli.Flag{}

func cflag(name string) cli.Flag {
	cf, ok := Flags[name]
	if !ok {
		log.Fatalf("unknown flag='%s'", name)
	}

	return cf
}

func initFlagShowProgress() cli.Flag {
	//enable 'show-progress' by default only on Mac OS X
	var doShowProgressFlag cli.Flag
	switch runtime.GOOS {
	case "darwin":
		doShowProgressFlag = &cli.BoolFlag{
			Name:    command.FlagShowProgress,
			Value:   true,
			Usage:   fmt.Sprintf("%s (default: true)", command.FlagShowProgressUsage),
			EnvVars: []string{"DSLIM_UPDATE_SHOW_PROGRESS"},
		}
	default:
		doShowProgressFlag = &cli.BoolFlag{
			Name:    command.FlagShowProgress,
			Usage:   fmt.Sprintf("%s (default: false)", command.FlagShowProgressUsage),
			EnvVars: []string{"DSLIM_UPDATE_SHOW_PROGRESS"},
		}
	}

	return doShowProgressFlag
}
