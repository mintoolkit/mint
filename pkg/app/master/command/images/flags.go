package images

import (
	log "github.com/sirupsen/logrus"
	"github.com/urfave/cli/v2"
)

// Images command flag names and usage descriptions
const (
	FlagFilter      = "filter"
	FlagFilterUsage = "container image filter pattern"
	// TODO - replace with reference to `master/command.FlagTUI`
	FlagTUI      = "tui"
	FlagTUIUsage = "terminal user interface"
)

var Flags = map[string]cli.Flag{
	FlagFilter: &cli.StringFlag{
		Name:    FlagFilter,
		Value:   "",
		Usage:   FlagFilterUsage,
		EnvVars: []string{"DSLIM_IMAGES_FILTER"},
	},
	FlagTUI: &cli.BoolFlag{
		Name:  FlagTUI,
		Usage: FlagTUIUsage,
	},
}

func cflag(name string) cli.Flag {
	cf, ok := Flags[name]
	if !ok {
		log.Fatalf("unknown flag='%s'", name)
	}

	return cf
}
