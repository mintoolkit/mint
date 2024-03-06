package dockerclipm

import (
	"github.com/c-bata/go-prompt"

	"github.com/mintoolkit/mint/pkg/app/master/command"
)

func RegisterCommand() {
	command.AddCLICommand(
		Name,
		CLI,
		prompt.Suggest{},
		nil)
}
