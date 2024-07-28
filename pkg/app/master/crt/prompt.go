package crt

import (
	"github.com/c-bata/go-prompt"

	"github.com/mintoolkit/mint/pkg/app/master/command"
)

var runtimeValues = []prompt.Suggest{
	{Text: DockerRuntime, Description: DockerRuntimeDesc},
	{Text: KubernetesRuntime, Description: KubernetesRuntimeDesc},
	{Text: ContainerdRuntime, Description: ContainerdRuntimeDesc},
	{Text: PodmanRuntime, Description: PodmanRuntimeDesc},
	{Text: AutoRuntime, Description: AutoRuntimeDesc},
}

func CompleteRuntime(ia *command.InteractiveApp, token string, params prompt.Document) []prompt.Suggest {
	return prompt.FilterHasPrefix(runtimeValues, token, true)
}

func ResolveAutoRuntime(val string) string {
	if val != AutoRuntime {
		return val
	}

	return AutoSelectRuntime()
}
