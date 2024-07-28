package crt

import (
	log "github.com/sirupsen/logrus"
	"github.com/urfave/cli/v2"
)

// Container runtime command flag names and usage descriptions
const (
	FlagRuntime      = "runtime"
	FlagRuntimeUsage = "Runtime environment type"
)

var Flags = map[string]cli.Flag{
	FlagRuntime: &cli.StringFlag{
		Name:    FlagRuntime,
		Value:   AutoRuntime,
		Usage:   FlagRuntimeUsage,
		EnvVars: []string{"DSLIM_CRT_NAME"},
	},
}

func Cflag(name string) cli.Flag {
	cf, ok := Flags[name]
	if !ok {
		log.Fatalf("unknown flag='%s'", name)
	}

	return cf
}
