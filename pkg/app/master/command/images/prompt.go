package images

import (
	"github.com/c-bata/go-prompt"

	"github.com/mintoolkit/mint/pkg/app/master/command"
)

var CommandSuggestion = prompt.Suggest{
	Text:        Name,
	Description: Usage,
}

var CommandFlagSuggestions = &command.FlagSuggestions{
	Names: []prompt.Suggest{
		{Text: command.FullFlagName(command.FlagRuntime), Description: command.FlagRuntimeUsage},
		{Text: command.FullFlagName(FlagFilter), Description: FlagFilterUsage},
	},
	Values: map[string]command.CompleteValue{
		command.FullFlagName(command.FlagRuntime): command.CompleteRuntime,
	},
}
