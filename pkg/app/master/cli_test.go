package app

import (
	"github.com/mintoolkit/mint/pkg/app/master/signals"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestCLI(t *testing.T) {
	signals.InitHandlers()
	cli := newCLI()

	runArgs := [][]string{
		{"mint", "--version"},
		{"mint", "-v"},
		{"mint", "help"},
		{"mint", "-h"},
	}
	for _, args := range runArgs {
		require.NoError(t, cli.Run(args))
	}
}
