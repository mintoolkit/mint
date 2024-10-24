package imagebuild

import (
	"github.com/c-bata/go-prompt"

	"github.com/mintoolkit/mint/pkg/app/master/command"
)

var CommandSuggestion = prompt.Suggest{
	Text:        Name,
	Description: Usage,
}

var buildEngineValues = []prompt.Suggest{
	{Text: DockerBuildEngine, Description: DockerBuildEngineInfo},
	{Text: BuildkitBuildEngine, Description: BuildkitBuildEngineInfo},
	{Text: DepotBuildEngine, Description: DepotBuildEngineInfo},
	{Text: PodmanBuildEngine, Description: PodmanBuildEngineInfo},
	{Text: SimpleBuildEngine, Description: SimpleBuildEngineInfo},
}

func completeBuildEngine(ia *command.InteractiveApp, token string, params prompt.Document) []prompt.Suggest {
	return prompt.FilterHasPrefix(buildEngineValues, token, true)
}

var runtimeLoadValues = []prompt.Suggest{
	{Text: NoneRuntimeLoad, Description: "Do not load image in any container runtime"},
	{Text: DockerRuntimeLoad, Description: "Load image into Docker container runtime"},
	{Text: PodmanRuntimeLoad, Description: "Load image into Podman container runtime"},
}

func completeRuntimeLoad(ia *command.InteractiveApp, token string, params prompt.Document) []prompt.Suggest {
	return prompt.FilterHasPrefix(runtimeLoadValues, token, true)
}

var architectureValues = []prompt.Suggest{
	{Text: Amd64Arch},
	{Text: Arm64Arch},
}

func completeArchitecture(ia *command.InteractiveApp, token string, params prompt.Document) []prompt.Suggest {
	return prompt.FilterHasPrefix(architectureValues, token, true)
}

var CommandFlagSuggestions = &command.FlagSuggestions{
	Names: []prompt.Suggest{
		{Text: command.FullFlagName(FlagEngine), Description: FlagEngineUsage},
		{Text: command.FullFlagName(FlagRuntimeLoad), Description: FlagRuntimeLoadUsage},
		{Text: command.FullFlagName(FlagEngineEndpoint), Description: FlagEngineEndpointUsage},
		{Text: command.FullFlagName(FlagEngineToken), Description: FlagEngineTokenUsage},
		{Text: command.FullFlagName(FlagEngineNamespace), Description: FlagEngineNamespaceUsage},
		{Text: command.FullFlagName(FlagImageName), Description: FlagImageNameUsage},
		{Text: command.FullFlagName(FlagImageArchiveFile), Description: FlagImageArchiveFileUsage},
		{Text: command.FullFlagName(FlagDockerfile), Description: FlagDockerfileUsage},
		{Text: command.FullFlagName(FlagContextDir), Description: FlagContextDirUsage},
		{Text: command.FullFlagName(FlagBuildArg), Description: FlagBuildArgUsage},
		{Text: command.FullFlagName(FlagLabel), Description: FlagLabelUsage},
		{Text: command.FullFlagName(FlagArchitecture), Description: FlagArchitectureUsage},
		{Text: command.FullFlagName(FlagBase), Description: FlagBaseUsage},
		{Text: command.FullFlagName(FlagBaseTar), Description: FlagBaseTarUsage},
		{Text: command.FullFlagName(FlagBaseWithCerts), Description: FlagBaseWithCertsUsage},
		{Text: command.FullFlagName(FlagExePath), Description: FlagExePathUsage},
	},
	Values: map[string]command.CompleteValue{
		command.FullFlagName(FlagEngine):        completeBuildEngine,
		command.FullFlagName(FlagRuntimeLoad):   completeRuntimeLoad,
		command.FullFlagName(FlagArchitecture):  completeArchitecture,
		command.FullFlagName(FlagContextDir):    command.CompleteDir,
		command.FullFlagName(FlagBaseTar):       command.CompleteFile,
		command.FullFlagName(FlagBaseWithCerts): command.CompleteBool,
		command.FullFlagName(FlagExePath):       command.CompleteFile,
	},
}
