package images

import (
	log "github.com/sirupsen/logrus"
	"github.com/urfave/cli/v2"
)

// Images command flag names and usage descriptions
const (
	FlagFilter      = "filter"
	FlagFilterUsage = "container image filter pattern"
)

var Flags = map[string]cli.Flag{
	FlagFilter: &cli.StringFlag{
		Name:    FlagFilter,
		Value:   "",
		Usage:   FlagFilterUsage,
		EnvVars: []string{"DSLIM_IMAGES_FILTER"},
	},
}

func cflag(name string) cli.Flag {
	cf, ok := Flags[name]
	if !ok {
		log.Fatalf("unknown flag='%s'", name)
	}

	return cf
}
