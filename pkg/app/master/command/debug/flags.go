package debug

import (
	log "github.com/sirupsen/logrus"
	"github.com/urfave/cli/v2"
)

// Debug command flag names and usage descriptions
const (
	FlagRuntime      = "runtime"
	FlagRuntimeUsage = "Runtime environment type"

	FlagTarget      = "target"
	FlagTargetUsage = "Target container (name or ID)"

	FlagNamespace      = "namespace"
	FlagNamespaceUsage = "Namespace to target (k8s or containerd runtime)"

	FlagPod      = "pod"
	FlagPodUsage = "Pod to target (k8s runtime)"

	FlagDebugImage      = "debug-image"
	FlagDebugImageUsage = "Debug image to use for the debug sidecar container"

	FlagEntrypoint      = "entrypoint"
	FlagEntrypointUsage = "Custom ENTRYPOINT to use for the debug sidecar container."

	FlagCmd      = "cmd"
	FlagCmdUsage = "Custom CMD to use for the debug sidecar container (alternatively pass custom CMD params after '--')."

	FlagWorkdir      = "workdir"
	FlagWorkdirUsage = "Custom WORKDIR to use for the debug sidecar container."

	//value expected to be "name=value"
	FlagEnv      = "env"
	FlagEnvUsage = "Environment variable to add to the debug sidecar container (format: name=value)."

	//value expected to be "name:path" or "name:path:ro"
	FlagMount      = "mount"
	FlagMountUsage = "Volume to mount in the debug sidecar container (format: name:path or name:path:ro)."

	FlagMountTargetVolumes      = "mount-target-volumes"
	FlagMountTargetVolumesUsage = "Mount all volumes mounted in the target container"

	FlagLoadTargetEnvVars      = "load-target-env-vars"
	FlagLoadTargetEnvVarsUsage = "Load all (container spec) environment variables from the target container"

	FlagTerminal      = "terminal"
	FlagTerminalUsage = "Attach interactive terminal to the debug container"

	FlagRunAsTargetShell      = "run-as-target-shell"
	FlagRunAsTargetShellUsage = "Attach interactive terminal to the debug container and run shell as if it's running in the target container environment."

	FlagListSessions      = "list-sessions"
	FlagListSessionsUsage = "List all debug sessions for the selected target (pod and optionally selected container for k8s or container for other runtimes)."

	FlagShowSessionLogs      = "show-session-logs"
	FlagShowSessionLogsUsage = "Show logs for the selected debug session (using namespace, pod, target container or debug session container name for k8s or debug session container name for other runtimes)."

	FlagSession      = "session"
	FlagSessionUsage = "Debug session container name (used for debug sessoin actions)."

	FlagConnectSession      = "connect-session"
	FlagConnectSessionUsage = "Connect to existing debug session."

	//TBD
	FlagConnectLastSession      = "connect-last-session"
	FlagConnectLastSessionUsage = "Connect to last debug session"

	FlagListNamespaces      = "list-namespaces"
	FlagListNamespacesUsage = "List names for available namespaces (use this flag by itself)."

	FlagListPods      = "list-pods"
	FlagListPodsUsage = "List names for running pods in the selected namespace (use this flag by itself)."

	FlagListDebuggableContainers      = "list-debuggable-containers"
	FlagListDebuggableContainersUsage = "List container names for active containers that can be debugged (use this flag by itself)."

	FlagListDebugImage      = "list-debug-images"
	FlagListDebugImageUsage = "List possible debug images to use for the debug sidecar container (use this flag by itself)."

	FlagKubeconfig      = "kubeconfig"
	FlagKubeconfigUsage = "Kubeconfig file location (k8s runtime)"

	FlagUID      = "uid"
	FlagUIDUsage = "UID to use for the debugging sidecar container"

	FlagGID      = "gid"
	FlagGIDUsage = "GID to use for the debugging sidecar container"

	FlagRunPrivileged      = "run-privileged"
	FlagRunPrivilegedUsage = "Run the debug sidecar as a privileged container (true by default)"

	FlagSecurityContextFromTarget      = "security-context-from-target"
	FlagSecurityContextFromTargetUsage = "Use the security context params from the target container with the debug sidecar container"

	FlagAutoRunAsNonRoot      = "auto-run-as-non-root"
	FlagAutoRunAsNonRootUsage = "Auto-adjust the config to run as non-root"
)

var Flags = map[string]cli.Flag{
	FlagRuntime: &cli.StringFlag{
		Name:    FlagRuntime,
		Value:   AutoRuntime,
		Usage:   FlagRuntimeUsage,
		EnvVars: []string{"DSLIM_DBG_RT"},
	},
	FlagTarget: &cli.StringFlag{
		Name:    FlagTarget,
		Value:   "",
		Usage:   FlagTargetUsage,
		EnvVars: []string{"DSLIM_DBG_TARGET"},
	},
	FlagNamespace: &cli.StringFlag{
		Name:    FlagNamespace,
		Value:   NamespaceDefault,
		Usage:   FlagNamespaceUsage,
		EnvVars: []string{"DSLIM_DBG_TARGET_NS"},
	},
	FlagPod: &cli.StringFlag{
		Name:    FlagPod,
		Value:   "",
		Usage:   FlagPodUsage,
		EnvVars: []string{"DSLIM_DBG_TARGET_POD"},
	},
	FlagDebugImage: &cli.StringFlag{
		Name:    FlagDebugImage,
		Value:   BusyboxImage,
		Usage:   FlagDebugImageUsage,
		EnvVars: []string{"DSLIM_DBG_IMAGE"},
	},
	FlagEntrypoint: &cli.StringFlag{
		Name:    FlagEntrypoint,
		Value:   "",
		Usage:   FlagEntrypointUsage,
		EnvVars: []string{"DSLIM_DBG_ENTRYPOINT"},
	},
	FlagCmd: &cli.StringFlag{
		Name:    FlagCmd,
		Value:   "",
		Usage:   FlagCmdUsage,
		EnvVars: []string{"DSLIM_DBG_CMD"},
	},
	FlagWorkdir: &cli.StringFlag{
		Name:    FlagWorkdir,
		Value:   "",
		Usage:   FlagWorkdirUsage,
		EnvVars: []string{"DSLIM_DBG_WDIR"},
	},
	FlagEnv: &cli.StringSliceFlag{
		Name:    FlagEnv,
		Value:   cli.NewStringSlice(),
		Usage:   FlagEnvUsage,
		EnvVars: []string{"DSLIM_DBG_ENV"},
	},
	FlagLoadTargetEnvVars: &cli.BoolFlag{
		Name:    FlagLoadTargetEnvVars,
		Value:   true, //true by default
		Usage:   FlagLoadTargetEnvVarsUsage,
		EnvVars: []string{"DSLIM_DBG_LOAD_TARGET_ENVS"},
	},
	FlagMount: &cli.StringSliceFlag{
		Name:    FlagMount,
		Value:   cli.NewStringSlice(),
		Usage:   FlagMountUsage,
		EnvVars: []string{"DSLIM_DBG_MOUNT"},
	},
	FlagMountTargetVolumes: &cli.BoolFlag{
		Name:    FlagMountTargetVolumes,
		Value:   false, //false by default (because by default we run privileged)
		Usage:   FlagMountTargetVolumesUsage,
		EnvVars: []string{"DSLIM_DBG_MOUNT_TARGET_VOLUMES"},
	},
	FlagTerminal: &cli.BoolFlag{
		Name:    FlagTerminal,
		Value:   true, //attach interactive terminal by default
		Usage:   FlagTerminalUsage,
		EnvVars: []string{"DSLIM_DBG_TERMINAL"},
	},
	FlagRunAsTargetShell: &cli.BoolFlag{
		Name:    FlagRunAsTargetShell,
		Value:   true, //do it by default (FlagTerminal will be ignored, assumed to be true)
		Usage:   FlagRunAsTargetShellUsage,
		EnvVars: []string{"DSLIM_DBG_RATS"},
	},
	FlagListSessions: &cli.BoolFlag{
		Name:    FlagListSessions,
		Value:   false,
		Usage:   FlagListSessionsUsage,
		EnvVars: []string{"DSLIM_DBG_LIST_SESSIONS"},
	},
	FlagShowSessionLogs: &cli.BoolFlag{
		Name:    FlagShowSessionLogs,
		Value:   false,
		Usage:   FlagShowSessionLogsUsage,
		EnvVars: []string{"DSLIM_DBG_SHOW_SESSION_LOGS"},
	},
	FlagConnectSession: &cli.BoolFlag{
		Name:    FlagConnectSession,
		Value:   false,
		Usage:   FlagConnectSessionUsage,
		EnvVars: []string{"DSLIM_DBG_CONNECT_SESSION"},
	},
	FlagSession: &cli.StringFlag{
		Name:    FlagSession,
		Value:   "",
		Usage:   FlagSessionUsage,
		EnvVars: []string{"DSLIM_DBG_SESSION"},
	},
	FlagListNamespaces: &cli.BoolFlag{
		Name:    FlagListNamespaces,
		Value:   false,
		Usage:   FlagListNamespacesUsage,
		EnvVars: []string{"DSLIM_DBG_LIST_NAMESPACES"},
	},
	FlagListPods: &cli.BoolFlag{
		Name:    FlagListPods,
		Value:   false,
		Usage:   FlagListPodsUsage,
		EnvVars: []string{"DSLIM_DBG_LIST_PODS"},
	},
	FlagListDebuggableContainers: &cli.BoolFlag{
		Name:    FlagListDebuggableContainers,
		Value:   false,
		Usage:   FlagListDebuggableContainersUsage,
		EnvVars: []string{"DSLIM_DBG_LIST_CONTAINERS"},
	},
	FlagListDebugImage: &cli.BoolFlag{
		Name:    FlagListDebugImage,
		Value:   false,
		Usage:   FlagListDebugImageUsage,
		EnvVars: []string{"DSLIM_DBG_LIST_IMAGES"},
	},
	FlagKubeconfig: &cli.StringFlag{
		Name:    FlagKubeconfig,
		Value:   KubeconfigDefault,
		Usage:   FlagKubeconfigUsage,
		EnvVars: []string{"DSLIM_DBG_KUBECONFIG"},
	},
	FlagUID: &cli.Int64Flag{
		Name:    FlagUID,
		Value:   -1,
		Usage:   FlagUIDUsage,
		EnvVars: []string{"DSLIM_DBG_UID"},
	},
	FlagGID: &cli.Int64Flag{
		Name:    FlagGID,
		Value:   -1,
		Usage:   FlagGIDUsage,
		EnvVars: []string{"DSLIM_DBG_GID"},
	},
	FlagRunPrivileged: &cli.BoolFlag{
		Name:    FlagRunPrivileged,
		Value:   true, //true by default
		Usage:   FlagRunPrivilegedUsage,
		EnvVars: []string{"DSLIM_DBG_RUN_PRIV"},
	},
	FlagSecurityContextFromTarget: &cli.BoolFlag{
		Name:    FlagSecurityContextFromTarget,
		Value:   false,
		Usage:   FlagSecurityContextFromTargetUsage,
		EnvVars: []string{"DSLIM_DBG_USE_TARGET_SEC_CTX"},
	},
	FlagAutoRunAsNonRoot: &cli.BoolFlag{
		Name:    FlagAutoRunAsNonRoot,
		Value:   true, //true by default
		Usage:   FlagAutoRunAsNonRootUsage,
		EnvVars: []string{"DSLIM_DBG_AUTO_RUN_AS_NONROOT"},
	},
}

func cflag(name string) cli.Flag {
	cf, ok := Flags[name]
	if !ok {
		log.Fatalf("unknown flag='%s'", name)
	}

	return cf
}
