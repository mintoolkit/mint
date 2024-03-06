package merge

import (
	"github.com/mintoolkit/mint/pkg/app/master/command"
)

func RegisterCommand() {
	command.AddCLICommand(
		Name,
		CLI,
		CommandSuggestion,
		CommandFlagSuggestions)
}
