package registry

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
		{Text: command.FullFlagName(FlagUseDockerCreds), Description: FlagUseDockerCredsUsage},
		{Text: command.FullFlagName(FlagCredsAccount), Description: FlagCredsAccountUsage},
		{Text: command.FullFlagName(FlagCredsSecret), Description: FlagCredsSecretUsage},
		//including sub-commands here too
		{Text: PullCmdName, Description: PullCmdNameUsage},
		{Text: PushCmdName, Description: PushCmdNameUsage},
		{Text: ImageIndexCreateCmdName, Description: ImageIndexCreateCmdNameUsage},
		{Text: ServerCmdName, Description: ServerCmdNameUsage},
	},
	Values: map[string]command.CompleteValue{
		command.FullFlagName(FlagUseDockerCreds): command.CompleteTBool,
	},
}
