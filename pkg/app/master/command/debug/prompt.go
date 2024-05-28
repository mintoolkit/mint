package debug

import (
	"context"
	"fmt"

	"github.com/c-bata/go-prompt"

	"github.com/mintoolkit/mint/pkg/app/master/command"
)

var CommandSuggestion = prompt.Suggest{
	Text:        Name,
	Description: Usage,
}

var CommandFlagSuggestions = &command.FlagSuggestions{
	Names: []prompt.Suggest{
		{Text: command.FullFlagName(FlagRuntime), Description: FlagRuntimeUsage},
		{Text: command.FullFlagName(FlagTarget), Description: FlagTargetUsage},
		{Text: command.FullFlagName(FlagNamespace), Description: FlagNamespaceUsage},
		{Text: command.FullFlagName(FlagPod), Description: FlagPodUsage},
		{Text: command.FullFlagName(FlagDebugImage), Description: FlagDebugImageUsage},
		{Text: command.FullFlagName(FlagEntrypoint), Description: FlagEntrypointUsage},
		{Text: command.FullFlagName(FlagCmd), Description: FlagCmdUsage},
		{Text: command.FullFlagName(FlagShellCmd), Description: FlagShellCmdUsage},
		{Text: command.FullFlagName(FlagWorkdir), Description: FlagWorkdirUsage},
		{Text: command.FullFlagName(FlagEnv), Description: FlagEnvUsage},
		{Text: command.FullFlagName(FlagMount), Description: FlagMountUsage},
		{Text: command.FullFlagName(FlagMountTargetVolumes), Description: FlagMountTargetVolumesUsage},
		{Text: command.FullFlagName(FlagLoadTargetEnvVars), Description: FlagLoadTargetEnvVarsUsage},
		{Text: command.FullFlagName(FlagUID), Description: FlagUIDUsage},
		{Text: command.FullFlagName(FlagGID), Description: FlagGIDUsage},
		{Text: command.FullFlagName(FlagRunPrivileged), Description: FlagRunPrivilegedUsage},
		{Text: command.FullFlagName(FlagSecurityContextFromTarget), Description: FlagSecurityContextFromTargetUsage},
		{Text: command.FullFlagName(FlagFallbackToTargetUser), Description: FlagFallbackToTargetUserUsage},
		{Text: command.FullFlagName(FlagTerminal), Description: FlagTerminalUsage},
		{Text: command.FullFlagName(FlagRunAsTargetShell), Description: FlagRunAsTargetShellUsage},
		{Text: command.FullFlagName(FlagListSessions), Description: FlagListSessionsUsage},
		{Text: command.FullFlagName(FlagShowSessionLogs), Description: FlagShowSessionLogsUsage},
		{Text: command.FullFlagName(FlagConnectSession), Description: FlagConnectSessionUsage},
		{Text: command.FullFlagName(FlagSession), Description: FlagSessionUsage},
		{Text: command.FullFlagName(FlagListNamespaces), Description: FlagListNamespacesUsage},
		{Text: command.FullFlagName(FlagListPods), Description: FlagListPodsUsage},
		{Text: command.FullFlagName(FlagListDebuggableContainers), Description: FlagListDebuggableContainersUsage},
		{Text: command.FullFlagName(FlagListDebugImage), Description: FlagListDebugImageUsage},
		{Text: command.FullFlagName(FlagKubeconfig), Description: FlagKubeconfigUsage},
	},
	Values: map[string]command.CompleteValue{
		command.FullFlagName(FlagRuntime):                   completeRuntime,
		command.FullFlagName(FlagTarget):                    completeTarget,
		command.FullFlagName(FlagDebugImage):                completeDebugImage,
		command.FullFlagName(FlagTerminal):                  command.CompleteTBool,
		command.FullFlagName(FlagRunAsTargetShell):          command.CompleteTBool,
		command.FullFlagName(FlagListSessions):              command.CompleteBool,
		command.FullFlagName(FlagShowSessionLogs):           command.CompleteBool,
		command.FullFlagName(FlagConnectSession):            command.CompleteBool,
		command.FullFlagName(FlagSession):                   completeSession,
		command.FullFlagName(FlagListNamespaces):            command.CompleteBool,
		command.FullFlagName(FlagListPods):                  command.CompleteBool,
		command.FullFlagName(FlagListDebuggableContainers):  command.CompleteBool,
		command.FullFlagName(FlagListDebugImage):            command.CompleteBool,
		command.FullFlagName(FlagNamespace):                 completeNamespace,
		command.FullFlagName(FlagPod):                       completePod,
		command.FullFlagName(FlagMountTargetVolumes):        command.CompleteBool,
		command.FullFlagName(FlagLoadTargetEnvVars):         command.CompleteTBool,
		command.FullFlagName(FlagRunPrivileged):             command.CompleteTBool,
		command.FullFlagName(FlagSecurityContextFromTarget): command.CompleteBool,
		command.FullFlagName(FlagFallbackToTargetUser):      command.CompleteTBool,
	},
}

func getDebugImageValues() []prompt.Suggest {
	var values []prompt.Suggest
	for k, v := range debugImages {
		value := prompt.Suggest{Text: k, Description: v}
		values = append(values, value)
	}

	return values
}

func completeDebugImage(ia *command.InteractiveApp, token string, params prompt.Document) []prompt.Suggest {
	return prompt.FilterHasPrefix(getDebugImageValues(), token, true)
}

var runtimeValues = []prompt.Suggest{
	{Text: DockerRuntime, Description: DockerRuntimeDesc},
	{Text: KubernetesRuntime, Description: KubernetesRuntimeDesc},
	{Text: ContainerdRuntime, Description: ContainerdRuntimeDesc},
	{Text: PodmanRuntime, Description: PodmanRuntimeDesc},
	{Text: AutoRuntime, Description: AutoRuntimeDesc},
}

func completeRuntime(ia *command.InteractiveApp, token string, params prompt.Document) []prompt.Suggest {
	return prompt.FilterHasPrefix(runtimeValues, token, true)
}

func resolveAutoRuntime(val string) string {
	if val != AutoRuntime {
		return val
	}

	return AutoSelectRuntime()
}

func completeNamespace(ia *command.InteractiveApp, token string, params prompt.Document) []prompt.Suggest {
	var values []prompt.Suggest
	ccs := command.GetCurrentCommandState()
	if ccs != nil && ccs.Command == Name {
		runtimeFlag := command.FullFlagName(FlagRuntime)
		if rtFlagVals, found := ccs.CommandFlags[runtimeFlag]; found {
			if len(rtFlagVals) > 0 {
				runtime := resolveAutoRuntime(rtFlagVals[0])

				var names []string
				switch runtime {
				case KubernetesRuntime:
					kubeconfig := KubeconfigDefault
					kubeconfigFlag := command.FullFlagName(FlagKubeconfig)
					kcFlagVals, found := ccs.CommandFlags[kubeconfigFlag]
					if found && len(kcFlagVals) > 0 {
						kubeconfig = kcFlagVals[0]
					}

					names, _ = listNamespacesWithConfig(kubeconfig)
				case ContainerdRuntime:
					names, _ = cdListNamespaces()
				}

				for _, name := range names {
					value := prompt.Suggest{Text: name}
					values = append(values, value)
				}
			}
		}
	}

	return prompt.FilterHasPrefix(values, token, true)
}

func completePod(ia *command.InteractiveApp, token string, params prompt.Document) []prompt.Suggest {
	var values []prompt.Suggest
	ccs := command.GetCurrentCommandState()
	if ccs != nil && ccs.Command == Name {
		runtimeFlag := command.FullFlagName(FlagRuntime)
		if rtFlagVals, found := ccs.CommandFlags[runtimeFlag]; found {
			if len(rtFlagVals) > 0 && resolveAutoRuntime(rtFlagVals[0]) == KubernetesRuntime {
				kubeconfig := KubeconfigDefault
				kubeconfigFlag := command.FullFlagName(FlagKubeconfig)
				kcFlagVals, found := ccs.CommandFlags[kubeconfigFlag]
				if found && len(kcFlagVals) > 0 {
					kubeconfig = kcFlagVals[0]
				}

				namespace := NamespaceDefault
				namespaceFlag := command.FullFlagName(FlagNamespace)
				nsFlagVals, found := ccs.CommandFlags[namespaceFlag]
				if found && len(nsFlagVals) > 0 {
					namespace = nsFlagVals[0]
				}

				names, _ := listActivePodsWithConfig(kubeconfig, namespace)
				for _, name := range names {
					value := prompt.Suggest{Text: name}
					values = append(values, value)
				}
			}
		}
	}

	return prompt.FilterHasPrefix(values, token, true)
}

func completeTarget(ia *command.InteractiveApp, token string, params prompt.Document) []prompt.Suggest {
	var values []prompt.Suggest
	ccs := command.GetCurrentCommandState()
	if ccs != nil && ccs.Command == Name {
		runtimeFlag := command.FullFlagName(FlagRuntime)
		rtFlagVals, found := ccs.CommandFlags[runtimeFlag]
		runtime := AutoRuntime
		if found && len(rtFlagVals) > 0 {
			runtime = rtFlagVals[0]
		}

		runtime = resolveAutoRuntime(runtime)
		switch runtime {
		case KubernetesRuntime:
			kubeconfig := KubeconfigDefault
			kubeconfigFlag := command.FullFlagName(FlagKubeconfig)
			kcFlagVals, found := ccs.CommandFlags[kubeconfigFlag]
			if found && len(kcFlagVals) > 0 {
				kubeconfig = kcFlagVals[0]
			}

			namespace := NamespaceDefault
			namespaceFlag := command.FullFlagName(FlagNamespace)
			nsFlagVals, found := ccs.CommandFlags[namespaceFlag]
			if found && len(nsFlagVals) > 0 {
				namespace = nsFlagVals[0]
			}

			var pod string
			podFlag := command.FullFlagName(FlagPod)
			podFlagVals, found := ccs.CommandFlags[podFlag]
			if found && len(podFlagVals) > 0 {
				pod = podFlagVals[0]
			}

			result, err := listDebuggableK8sContainersWithConfig(kubeconfig, namespace, pod)
			if err == nil {
				for cname, iname := range result {
					value := prompt.Suggest{
						Text:        cname,
						Description: fmt.Sprintf("image: %s", iname),
					}
					values = append(values, value)
				}
			}
		case ContainerdRuntime:
			conts, _ := cdListDebuggableContainers(context.Background(), nil)

			for _, c := range conts {
				value := prompt.Suggest{
					Text:        c.Name,
					Description: fmt.Sprintf("image: %s", c.Image),
				}
				values = append(values, value)
			}
		case PodmanRuntime:
			var connCtx context.Context
			if ccs.CRTConnection != "" {
				connCtx = getPodmanConnContextWithConn(ccs.CRTConnection)
			} else {
				connCtx = getPodmanConnContext()
			}

			if connCtx != nil {
				conts, _ := listDebuggablePodmanContainersWithConfig(connCtx)
				for cname, iname := range conts {
					value := prompt.Suggest{
						Text:        cname,
						Description: fmt.Sprintf("image: %s", iname),
					}
					values = append(values, value)
				}
			}
		default:
			//either no explicit 'runtime' param or other/docker runtime
			//todo: need a way to access/pass the docker client struct (or just pass the connect params)
			result, err := listDebuggableDockerContainersWithConfig(ccs.Dclient)
			if err == nil {
				for cname, iname := range result {
					value := prompt.Suggest{
						Text:        cname,
						Description: fmt.Sprintf("image: %s", iname),
					}
					values = append(values, value)
				}
			}
		}
	}

	return prompt.FilterHasPrefix(values, token, true)
}

func completeSession(ia *command.InteractiveApp, token string, params prompt.Document) []prompt.Suggest {
	var values []prompt.Suggest
	ccs := command.GetCurrentCommandState()
	if ccs != nil && ccs.Command == Name {
		csessValStr := ccs.GetCFValue(FlagConnectSession)

		runtimeFlag := command.FullFlagName(FlagRuntime)
		rtFlagVals, found := ccs.CommandFlags[runtimeFlag]
		handleDockerRuntime := true
		if found && len(rtFlagVals) > 0 {
			handleDockerRuntime = false
			switch resolveAutoRuntime(rtFlagVals[0]) {
			case KubernetesRuntime:
				kubeconfig := KubeconfigDefault
				kubeconfigFlag := command.FullFlagName(FlagKubeconfig)
				kcFlagVals, found := ccs.CommandFlags[kubeconfigFlag]
				if found && len(kcFlagVals) > 0 {
					kubeconfig = kcFlagVals[0]
				}

				namespace := ccs.GetCFValueWithDefault(FlagNamespace, NamespaceDefault)

				var pod string
				podFlag := command.FullFlagName(FlagPod)
				podFlagVals, found := ccs.CommandFlags[podFlag]
				if found && len(podFlagVals) > 0 {
					pod = podFlagVals[0]
				}

				target := ccs.GetCFValue(FlagTarget)

				result, err := listK8sDebugContainersWithConfig(
					kubeconfig,
					namespace,
					pod,
					target,
					command.IsTrueStr(csessValStr))

				if err == nil {
					for _, info := range result {
						desc := fmt.Sprintf("state: %s / start_time: %s / target: %s / image: %s",
							info.State,
							info.StartTime,
							info.TargetContainerName,
							info.SpecImage)
						value := prompt.Suggest{
							Text:        info.Name,
							Description: desc,
						}
						values = append(values, value)
					}
				}
			case ContainerdRuntime:
				var target string
				targetFlag := command.FullFlagName(FlagTarget)
				targetFlagVals, found := ccs.CommandFlags[targetFlag]
				if found && len(targetFlagVals) > 0 {
					target = targetFlagVals[0]
				}

				result, err := cdListDebugContainersWithConfig(
					target,
					command.IsTrueStr(csessValStr))
				if err == nil {
					for _, info := range result {
						desc := fmt.Sprintf("image: %s",
							info.Image)
						value := prompt.Suggest{
							Text:        info.Name,
							Description: desc,
						}
						values = append(values, value)
					}
				}
			case PodmanRuntime:
				var target string
				targetFlag := command.FullFlagName(FlagTarget)
				targetFlagVals, found := ccs.CommandFlags[targetFlag]
				if found && len(targetFlagVals) > 0 {
					target = targetFlagVals[0]
				}

				var connCtx context.Context
				if ccs.CRTConnection != "" {
					connCtx = getPodmanConnContextWithConn(ccs.CRTConnection)
				} else {
					connCtx = getPodmanConnContext()
				}

				if connCtx != nil {
					result, err := listPodmanDebugContainersWithConfig(
						connCtx,
						target,
						command.IsTrueStr(csessValStr))
					if err == nil {
						for _, info := range result {
							desc := fmt.Sprintf("state: %s / start_time: %s / target: %s / image: %s",
								info.State,
								info.StartTime,
								info.TargetContainerName,
								info.SpecImage)
							value := prompt.Suggest{
								Text:        info.Name,
								Description: desc,
							}
							values = append(values, value)
						}
					}
				}
			default:
				handleDockerRuntime = true
			}
		}

		if handleDockerRuntime {
			//either no explicit 'runtime' param or other/docker runtime
			//todo: need a way to access/pass the docker client struct (or just pass the connect params)
			var target string
			targetFlag := command.FullFlagName(FlagTarget)
			targetFlagVals, found := ccs.CommandFlags[targetFlag]
			if found && len(targetFlagVals) > 0 {
				target = targetFlagVals[0]
			}

			result, err := listDockerDebugContainersWithConfig(ccs.Dclient,
				target,
				command.IsTrueStr(csessValStr))
			if err == nil {
				for _, info := range result {
					desc := fmt.Sprintf("state: %s / start_time: %s / target: %s / image: %s",
						info.State,
						info.StartTime,
						info.TargetContainerName,
						info.SpecImage)
					value := prompt.Suggest{
						Text:        info.Name,
						Description: desc,
					}
					values = append(values, value)
				}
			}
		}
	}

	return prompt.FilterHasPrefix(values, token, true)
}
