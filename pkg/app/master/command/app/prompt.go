package app

import (
	"github.com/c-bata/go-prompt"

	"github.com/mintoolkit/mint/pkg/app/master/command"
)

var CommandSuggestion = prompt.Suggest{
	Text:        Name,
	Description: Usage,
}

// TODO: need to improve flag prompting to work with sub-commands properly
var CommandFlagSuggestions = &command.FlagSuggestions{
	Names: []prompt.Suggest{
		{Text: command.FullFlagName(command.FlagShowProgress), Description: command.FlagShowProgressUsage},
	},
	Values: map[string]command.CompleteValue{
		command.FullFlagName(command.FlagShowProgress): command.CompleteProgress,
	},
}
