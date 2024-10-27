package debug

import (
	"fmt"
	"strings"

	"github.com/urfave/cli/v2"

	"github.com/mintoolkit/mint/pkg/app"
	"github.com/mintoolkit/mint/pkg/app/master/command"
	"github.com/mintoolkit/mint/pkg/crt"
)

//Debug container

const (
	Name  = "debug"
	Usage = "Debug the target container from a debug (side-car) container"
	Alias = "dbg"
)

type NVPair struct {
	Name  string
	Value string
}

type Volume struct {
	Name     string
	Path     string
	ReadOnly bool
}

type CommandParams struct {
	/// the runtime environment type
	Runtime string
	/// the running container which we want to attach to
	TargetRef string
	/// the target namespace (k8s runtime)
	TargetNamespace string
	/// the target pod (k8s runtime)
	TargetPod string
	/// the name/id of the container image used for debugging
	DebugContainerImage string
	/// ENTRYPOINT used launching the debugging container
	Entrypoint []string
	CmdIsShell bool
	/// CMD used launching the debugging container
	Cmd []string
	/// WORKDIR used launching the debugging container
	Workdir string
	/// Environment variables used launching the debugging container
	EnvVars []NVPair
	/// load the environment variables from the target container's container spec into the debug container
	DoLoadTargetEnvVars bool
	/// volumes to mount in the debug side-car container
	Volumes []Volume
	/// mount all volumes mounted in the target container
	DoMountTargetVolumes bool
	/// launch the debug container with an interactive terminal attached (like '--it' in docker)
	DoTerminal bool
	/// make it look like shell is running in the target container
	DoRunAsTargetShell bool
	/// Kubeconfig file path (k8s runtime)
	Kubeconfig string
	/// Debug session container name
	Session string
	/// Simple (non-debug) action - list namespaces
	ActionListNamespaces bool
	/// Simple (non-debug) action - list pods
	ActionListPods bool
	/// Simple (non-debug) action - list debuggable container
	ActionListDebuggableContainers bool
	/// Simple (non-debug) action - list debug sessions
	ActionListSessions bool
	/// Simple (non-debug) action - show debug sessions logs
	ActionShowSessionLogs bool
	/// Simple (non-debug) action - connect to an existing debug session
	ActionConnectSession bool
	/// UID to use for the debugging sidecar container
	UID int64
	/// GID to use for the debugging sidecar container
	GID int64
	/// run the debug sidecar as a privileged container
	DoRunPrivileged bool
	/// use the security context params from the target container with the debug sidecar container
	UseSecurityContextFromTarget bool
	/// fallback to using target container user if it's non-root (mostly for kubernetes)
	DoFallbackToTargetUser bool
	// `debug --tui` use mode`
	TUI bool
	// tui -> debug use mode
	GlobalTUI bool
}

func ParseNameValueList(list []string) []NVPair {
	var pairs []NVPair
	for _, val := range list {
		val = strings.TrimSpace(val)
		if val == "" {
			continue
		}

		parts := strings.SplitN(val, "=", 2)
		if len(parts) != 2 {
			continue
		}

		nv := NVPair{Name: parts[0], Value: parts[1]}
		pairs = append(pairs, nv)
	}

	return pairs
}

func ParseMountList(list []string) []Volume {
	var records []Volume
	for _, val := range list {
		val = strings.TrimSpace(val)
		if val == "" {
			continue
		}

		parts := strings.Split(val, ":")
		if len(parts) < 2 || len(parts) > 3 {
			continue
		}

		record := Volume{Name: parts[0], Path: parts[1]}
		if len(parts) > 2 && (parts[2] == "ro" || parts[2] == "readonly") {
			record.ReadOnly = true
		}

		records = append(records, record)
	}

	return records
}

var CLI = &cli.Command{
	Name:    Name,
	Aliases: []string{Alias},
	Usage:   Usage,
	Flags: []cli.Flag{
		command.Cflag(command.FlagRuntime),
		cflag(FlagTarget),
		cflag(FlagNamespace),
		cflag(FlagPod),
		cflag(FlagDebugImage),
		cflag(FlagEntrypoint),
		cflag(FlagCmd),
		cflag(FlagShellCmd),
		cflag(FlagWorkdir),
		cflag(FlagEnv),
		cflag(FlagLoadTargetEnvVars),
		cflag(FlagMount),
		cflag(FlagMountTargetVolumes),
		cflag(FlagTerminal),
		cflag(FlagRunAsTargetShell),
		cflag(FlagListSessions),
		cflag(FlagShowSessionLogs),
		cflag(FlagConnectSession),
		cflag(FlagSession),
		cflag(FlagListNamespaces),
		cflag(FlagListPods),
		cflag(FlagListDebuggableContainers),
		cflag(FlagListDebugImage),
		cflag(FlagKubeconfig),
		cflag(FlagUID),
		cflag(FlagGID),
		cflag(FlagRunPrivileged),
		cflag(FlagSecurityContextFromTarget),
		cflag(FlagFallbackToTargetUser),
		command.Cflag(command.FlagTUI),
	},
	Action: func(ctx *cli.Context) error {
		gcvalues := command.GlobalFlagValues(ctx)
		xc := app.NewExecutionContext(
			Name,
			gcvalues.QuietCLIMode,
			gcvalues.OutputFormat)

		if ctx.Bool(FlagListDebugImage) {
			xc.Out.State("action.list_debug_images")
			for k, v := range debugImages {
				xc.Out.Info("debug.image", ovars{"name": k, "description": v})
			}

			return nil
		}

		commandParams := &CommandParams{
			Runtime:                        ctx.String(command.FlagRuntime),
			TargetRef:                      ctx.String(FlagTarget),
			TargetNamespace:                ctx.String(FlagNamespace),
			TargetPod:                      ctx.String(FlagPod),
			DebugContainerImage:            ctx.String(FlagDebugImage),
			DoTerminal:                     ctx.Bool(FlagTerminal),
			DoRunAsTargetShell:             ctx.Bool(FlagRunAsTargetShell),
			Kubeconfig:                     ctx.String(FlagKubeconfig),
			Workdir:                        ctx.String(FlagWorkdir),
			EnvVars:                        ParseNameValueList(ctx.StringSlice(FlagEnv)),
			DoLoadTargetEnvVars:            ctx.Bool(FlagLoadTargetEnvVars),
			Volumes:                        ParseMountList(ctx.StringSlice(FlagMount)),
			DoMountTargetVolumes:           ctx.Bool(FlagMountTargetVolumes),
			Session:                        ctx.String(FlagSession),
			ActionListNamespaces:           ctx.Bool(FlagListNamespaces),
			ActionListPods:                 ctx.Bool(FlagListPods),
			ActionListDebuggableContainers: ctx.Bool(FlagListDebuggableContainers),
			ActionListSessions:             ctx.Bool(FlagListSessions),
			ActionShowSessionLogs:          ctx.Bool(FlagShowSessionLogs),
			ActionConnectSession:           ctx.Bool(FlagConnectSession),
			UID:                            ctx.Int64(FlagUID),
			GID:                            ctx.Int64(FlagGID),
			DoRunPrivileged:                ctx.Bool(FlagRunPrivileged),
			UseSecurityContextFromTarget:   ctx.Bool(FlagSecurityContextFromTarget),
			DoFallbackToTargetUser:         ctx.Bool(FlagFallbackToTargetUser),
			TUI:                            ctx.Bool(command.FlagTUI),
		}

		if commandParams.ActionListNamespaces &&
			commandParams.Runtime == crt.DockerRuntime {
			xc.Out.Error("param", "unsupported runtime flag")
			xc.Out.State("exited",
				ovars{
					"runtime.provided": commandParams.Runtime,
					"runtime.required": fmt.Sprintf("%s|%s", crt.KubernetesRuntime, crt.ContainerdRuntime),
					"action":           commandParams.ActionListNamespaces,
					"exit.code":        -1,
				})

			xc.Exit(-1)
		}

		if commandParams.ActionListPods &&
			commandParams.Runtime != crt.KubernetesRuntime {
			xc.Out.Error("param", "unsupported runtime flag")
			xc.Out.State("exited",
				ovars{
					"runtime.provided": commandParams.Runtime,
					"runtime.required": crt.KubernetesRuntime,
					"action":           commandParams.ActionListPods,
					"exit.code":        -1,
				})

			xc.Exit(-1)
		}

		var err error
		if rawEntrypoint := ctx.String(FlagEntrypoint); rawEntrypoint != "" {
			commandParams.Entrypoint, err = command.ParseExec(rawEntrypoint)
			if err != nil {
				return err
			}
		}

		if rawCmd := ctx.String(FlagCmd); rawCmd != "" {
			commandParams.Cmd, err = command.ParseExec(rawCmd)
			if err != nil {
				return err
			}
		}

		if rawCmd := ctx.String(FlagShellCmd); rawCmd != "" {
			commandParams.CmdIsShell = true
			commandParams.Cmd, err = command.ParseExec(rawCmd)
			if err != nil {
				return err
			}
		}

		//explicitly setting the entrypoint and/or cmd clears
		//implies a custom debug session where the 'RATS' setting should be ignored
		if len(commandParams.Entrypoint) > 0 || len(commandParams.Cmd) > 0 {
			commandParams.DoRunAsTargetShell = false
		}

		if commandParams.DoRunAsTargetShell {
			commandParams.DoTerminal = true
		}

		if !commandParams.ActionListNamespaces &&
			!commandParams.ActionListPods &&
			!commandParams.ActionListDebuggableContainers &&
			!commandParams.ActionListSessions &&
			!commandParams.ActionShowSessionLogs &&
			!commandParams.ActionConnectSession &&
			commandParams.TargetRef == "" {
			if ctx.Args().Len() < 1 {
				if commandParams.Runtime != crt.KubernetesRuntime {
					xc.Out.Error("param.target", "missing target")
					cli.ShowCommandHelp(ctx, Name)
					return nil
				}
				//NOTE:
				//It's ok to not specify the target container for k8s
				//We'll pick the default or first container in the target pod
			} else {
				commandParams.TargetRef = ctx.Args().First()
				if ctx.Args().Len() > 1 && ctx.Args().Slice()[1] == "--" {
					//NOTE:
					//Keep the original 'no terminal' behavior
					//use this shortcut mode as a way to quickly
					//run one off commands in the debugged container
					//When there's 'no terminal' we show
					//the debugger container log at the end.
					//TODO: revisit the behavior later...
					cmdSlice := ctx.Args().Slice()[2:]
					var cmdClean []string
					for _, v := range cmdSlice {
						v = strings.TrimSpace(v)
						if v != "" {
							cmdClean = append(cmdClean, v)
						}
					}
					if len(cmdClean) > 0 {
						commandParams.CmdIsShell = true
						commandParams.Cmd = cmdClean
						commandParams.DoTerminal = false
						commandParams.DoRunAsTargetShell = false
					}
				}
			}
		}

		if commandParams.DebugContainerImage == "" {
			commandParams.DebugContainerImage = BusyboxImage
		}

		OnCommand(
			xc,
			gcvalues,
			commandParams)

		return nil
	},
}
