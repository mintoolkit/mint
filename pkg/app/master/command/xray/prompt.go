package xray

import (
	"context"
	"fmt"
	"time"

	"github.com/c-bata/go-prompt"
	"github.com/dustin/go-humanize"
	log "github.com/sirupsen/logrus"

	"github.com/mintoolkit/mint/pkg/app/master/command"
	"github.com/mintoolkit/mint/pkg/crt"
	"github.com/mintoolkit/mint/pkg/crt/docker/dockerutil"
	"github.com/mintoolkit/mint/pkg/crt/podman/podmanutil"
)

var CommandSuggestion = prompt.Suggest{
	Text:        Name,
	Description: Usage,
}

var CommandFlagSuggestions = &command.FlagSuggestions{
	Names: []prompt.Suggest{
		{Text: command.FullFlagName(command.FlagRuntime), Description: command.FlagRuntimeUsage},
		{Text: command.FullFlagName(command.FlagCommandParamsFile), Description: command.FlagCommandParamsFileUsage},
		{Text: command.FullFlagName(command.FlagTarget), Description: command.FlagTargetUsage},
		{Text: command.FullFlagName(command.FlagPull), Description: command.FlagPullUsage},
		{Text: command.FullFlagName(command.FlagShowPullLogs), Description: command.FlagShowPullLogsUsage},
		{Text: command.FullFlagName(command.FlagRegistryAccount), Description: command.FlagRegistryAccountUsage},
		{Text: command.FullFlagName(command.FlagRegistrySecret), Description: command.FlagRegistrySecretUsage},
		{Text: command.FullFlagName(command.FlagDockerConfigPath), Description: command.FlagDockerConfigPathUsage},
		{Text: command.FullFlagName(FlagChanges), Description: FlagChangesUsage},
		{Text: command.FullFlagName(FlagChangesOutput), Description: FlagChangesOutputUsage},
		{Text: command.FullFlagName(FlagLayer), Description: FlagLayerUsage},
		{Text: command.FullFlagName(FlagAddImageManifest), Description: FlagAddImageManifestUsage},
		{Text: command.FullFlagName(FlagAddImageConfig), Description: FlagAddImageConfigUsage},
		{Text: command.FullFlagName(FlagLayerChangesMax), Description: FlagLayerChangesMaxUsage},
		{Text: command.FullFlagName(FlagAllChangesMax), Description: FlagAllChangesMaxUsage},
		{Text: command.FullFlagName(FlagAddChangesMax), Description: FlagAddChangesMaxUsage},
		{Text: command.FullFlagName(FlagModifyChangesMax), Description: FlagModifyChangesMaxUsage},
		{Text: command.FullFlagName(FlagDeleteChangesMax), Description: FlagDeleteChangesMaxUsage},
		{Text: command.FullFlagName(FlagChangePath), Description: FlagChangePathUsage},
		{Text: command.FullFlagName(FlagChangeData), Description: FlagChangeDataUsage},
		{Text: command.FullFlagName(FlagReuseSavedImage), Description: FlagReuseSavedImageUsage},
		{Text: command.FullFlagName(FlagHashData), Description: FlagHashDataUsage},
		{Text: command.FullFlagName(FlagDetectUTF8), Description: FlagDetectUTF8Usage},
		{Text: command.FullFlagName(FlagDetectDuplicates), Description: FlagDetectDuplicatesUsage},
		{Text: command.FullFlagName(FlagShowDuplicates), Description: FlagShowDuplicatesUsage},
		{Text: command.FullFlagName(FlagShowSpecialPerms), Description: FlagShowSpecialPermsUsage},
		{Text: command.FullFlagName(FlagChangeDataHash), Description: FlagChangeDataHashUsage},
		{Text: command.FullFlagName(FlagTopChangesMax), Description: FlagTopChangesMaxUsage},
		{Text: command.FullFlagName(FlagDetectAllCertFiles), Description: FlagDetectAllCertFilesUsage},
		{Text: command.FullFlagName(FlagDetectAllCertPKFiles), Description: FlagDetectAllCertPKFilesUsage},
		{Text: command.FullFlagName(FlagDetectIdentities), Description: FlagDetectIdentitiesUsage},
		{Text: command.FullFlagName(FlagDetectIdentitiesParam), Description: FlagDetectIdentitiesParamUsage},
		{Text: command.FullFlagName(FlagDetectIdentitiesDumpRaw), Description: FlagDetectIdentitiesDumpRawUsage},
		{Text: command.FullFlagName(FlagExportAllDataArtifacts), Description: FlagExportAllDataArtifactsUsage},
		{Text: command.FullFlagName(command.FlagRemoveFileArtifacts), Description: command.FlagRemoveFileArtifactsUsage},
	},
	Values: map[string]command.CompleteValue{
		command.FullFlagName(command.FlagRuntime):             command.CompleteRuntime,
		command.FullFlagName(command.FlagCommandParamsFile):   command.CompleteFile,
		command.FullFlagName(command.FlagPull):                command.CompleteTBool,
		command.FullFlagName(command.FlagShowPullLogs):        command.CompleteBool,
		command.FullFlagName(command.FlagDockerConfigPath):    command.CompleteFile,
		command.FullFlagName(command.FlagTarget):              completeTarget,
		command.FullFlagName(FlagChanges):                     completeLayerChanges,
		command.FullFlagName(FlagChangesOutput):               completeOutputs,
		command.FullFlagName(FlagAddImageManifest):            command.CompleteBool,
		command.FullFlagName(FlagAddImageConfig):              command.CompleteBool,
		command.FullFlagName(FlagHashData):                    command.CompleteBool,
		command.FullFlagName(FlagDetectDuplicates):            command.CompleteBool,
		command.FullFlagName(FlagShowDuplicates):              command.CompleteTBool,
		command.FullFlagName(FlagShowSpecialPerms):            command.CompleteTBool,
		command.FullFlagName(FlagReuseSavedImage):             command.CompleteTBool,
		command.FullFlagName(FlagDetectAllCertFiles):          command.CompleteBool,
		command.FullFlagName(FlagDetectAllCertPKFiles):        command.CompleteBool,
		command.FullFlagName(FlagDetectIdentities):            command.CompleteTBool,
		command.FullFlagName(command.FlagRemoveFileArtifacts): command.CompleteBool,
	},
}

var layerChangeValues = []prompt.Suggest{
	{Text: "none", Description: "Don't show any file system change details in image layers"},
	{Text: "all", Description: "Show all file system change details in image layers"},
	{Text: "delete", Description: "Show only 'delete' file system change details in image layers"},
	{Text: "modify", Description: "Show only 'modify' file system change details in image layers"},
	{Text: "add", Description: "Show only 'add' file system change details in image layers"},
}

func completeLayerChanges(ia *command.InteractiveApp, token string, params prompt.Document) []prompt.Suggest {
	return prompt.FilterHasPrefix(layerChangeValues, token, true)
}

var outputsValues = []prompt.Suggest{
	{Text: "all", Description: "Show changes in all outputs"},
	{Text: "report", Description: "Show changes in command report"},
	{Text: "console", Description: "Show changes in console"},
}

func completeOutputs(ia *command.InteractiveApp, token string, params prompt.Document) []prompt.Suggest {
	return prompt.FilterHasPrefix(outputsValues, token, true)
}

func completeTarget(ia *command.InteractiveApp, token string, params prompt.Document) []prompt.Suggest {
	var values []prompt.Suggest
	ccs := command.GetCurrentCommandState()
	if ccs != nil && ccs.Command == Name {
		runtimeFlag := command.FullFlagName(command.FlagRuntime)
		rtFlagVals, found := ccs.CommandFlags[runtimeFlag]
		runtime := crt.AutoRuntime
		if found && len(rtFlagVals) > 0 {
			runtime = rtFlagVals[0]
		}

		runtime = command.ResolveAutoRuntime(runtime)
		switch runtime {
		case crt.PodmanRuntime:
			var connCtx context.Context
			if ccs.CRTConnection != "" {
				connCtx = crt.GetPodmanConnContextWithConn(ccs.CRTConnection)
			} else {
				connCtx = crt.GetPodmanConnContext()
			}

			if connCtx != nil {
				images, err := podmanutil.ListImages(connCtx, "")
				if err != nil {
					log.Errorf("images.podman.completeTarget(%q): error - %v", token, err)
					return []prompt.Suggest{}
				}

				for name, info := range images {
					description := fmt.Sprintf("size=%v created=%v id=%v",
						humanize.Bytes(uint64(info.Size)),
						time.Unix(info.Created, 0).Format(time.RFC3339),
						info.ID)

					entry := prompt.Suggest{
						Text:        name,
						Description: description,
					}

					values = append(values, entry)
				}
			}
		default:
			//either no explicit 'runtime' param or other/docker runtime
			//todo: need a way to access/pass the docker client struct (or just pass the connect params)
			images, err := dockerutil.ListImages(ccs.Dclient, "")
			if err != nil {
				log.Errorf("images.docker.completeTarget(%q): error - %v", token, err)
				return []prompt.Suggest{}
			}

			for name, info := range images {
				description := fmt.Sprintf("size=%v created=%v id=%v",
					humanize.Bytes(uint64(info.Size)),
					time.Unix(info.Created, 0).Format(time.RFC3339),
					info.ID)

				entry := prompt.Suggest{
					Text:        name,
					Description: description,
				}

				values = append(values, entry)
			}
		}
	}

	return prompt.FilterHasPrefix(values, token, true)
}
