package config

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"time"

	docker "github.com/fsouza/go-dockerclient"

	"github.com/mintoolkit/mint/pkg/util/fsutil"
)

// AppOptionsFilename is the default name for the app configs
const AppOptionsFilename = "slim.config.json"

// AppOptions provides a set of global application parameters and command-specific defaults
// AppOptions values override the default flag values if they are set
// AppOptions is loaded from the "slim.config.json" file stored in the state path directory
type AppOptions struct {
	Global *GlobalAppOptions `json:"global,omitempty"`
}

// GlobalAppOptions provides a set of global application parameters
type GlobalAppOptions struct {
	NoColor        *bool   `json:"no_color,omitempty"`
	CheckVersion   *bool   `json:"check_version,omitempty"`
	Debug          *bool   `json:"debug,omitempty"`
	Verbose        *bool   `json:"verbose,omitempty"`
	Quiet          *bool   `json:"quiet,omitempty"`
	OutputFormat   *string `json:"output_format,omitempty"`
	LogLevel       *string `json:"log_level,omitempty"`
	Log            *string `json:"log,omitempty"`
	LogFormat      *string `json:"log_format,omitempty"`
	UseTLS         *bool   `json:"tls,omitempty"`
	VerifyTLS      *bool   `json:"tls_verify,omitempty"`
	TLSCertPath    *string `json:"tls_cert_path,omitempty"`
	APIVersion     *string `json:"api_version,omitempty"`
	Host           *string `json:"host,omitempty"`
	StatePath      *string `json:"state_path,omitempty"`
	ReportLocation *string `json:"report_location,omitempty"`
	CRTConnection  *string `json:"crt_connection,omitempty"`
	CRTContext     *string `json:"crt_context,omitempty"`
	ArchiveState   *string `json:"archive_state,omitempty"`
}

func NewAppOptionsFromFile(dir string) (*AppOptions, error) {
	filePath := filepath.Join(dir, AppOptionsFilename)
	var result AppOptions
	err := fsutil.LoadStructFromFile(filePath, &result)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}

		if err == fsutil.ErrNoFileData {
			return nil, nil
		}

		return nil, err
	}

	return &result, nil
}

// TODO: robustly parse `--network`/Network at the CLI level to avoid ambiguity.
// https://github.com/docker/cli/blob/cf8c4bab6477ef62122bda875f80d8472005010d/opts/network.go#L35

// ContainerOverrides provides a set of container field overrides
// It can also be used to update the image instructions when
// the "image-overrides" flag is provided
type ContainerOverrides struct {
	User            string
	Entrypoint      []string
	ClearEntrypoint bool
	Cmd             []string
	ClearCmd        bool
	Workdir         string
	Env             []string
	Hostname        string
	Network         string
	ExposedPorts    map[docker.Port]struct{}
	Volumes         map[string]struct{}
	Labels          map[string]string
}

// ImageNewInstructions provides a set new image instructions
type ImageNewInstructions struct {
	Entrypoint         []string
	ClearEntrypoint    bool
	Cmd                []string
	ClearCmd           bool
	Workdir            string
	Env                []string
	Volumes            map[string]struct{}
	ExposedPorts       map[docker.Port]struct{}
	Labels             map[string]string
	RemoveEnvs         map[string]struct{}
	RemoveVolumes      map[string]struct{}
	RemoveExposedPorts map[docker.Port]struct{}
	RemoveLabels       map[string]struct{}
}

// ContainerBuildOptions provides the options to use when
// building container images from Dockerfiles
type ContainerBuildOptions struct {
	Dockerfile        string
	DockerfileContext string
	Tag               string
	ExtraHosts        string
	BuildArgs         []CBOBuildArg
	Labels            map[string]string
	CacheFrom         []string
	Target            string
	NetworkMode       string
}

type CBOBuildArg struct {
	Name  string
	Value string
}

// ContainerRunOptions provides the options to use running a container
type ContainerRunOptions struct {
	HostConfig *docker.HostConfig
	//Explicit overrides for the base and host config fields
	//Host config field override are applied
	//on top of the fields in the HostConfig struct if it's provided (volume mounts are merged though)
	Runtime      string
	SysctlParams map[string]string
	ShmSize      int64
}

// VolumeMount provides the volume mount configuration information
type VolumeMount struct {
	Source      string
	Destination string
	Options     string
}

// DockerClient provides Docker client parameters
type DockerClient struct {
	UseTLS      bool
	VerifyTLS   bool
	TLSCertPath string
	Host        string
	APIVersion  string
	Context     string
	Env         map[string]string
}

// ContinueAfter mode enums
const (
	CAMContainerProbe = "container-probe"
	CAMProbe          = "probe"
	CAMEnter          = "enter"
	CAMTimeout        = "timeout"
	CAMSignal         = "signal"
	CAMExec           = "exec"
	CAMHostExec       = "host-exec"
	CAMAppExit        = "app-exit"
)

// ContinueAfter provides the command execution mode parameters
type ContinueAfter struct {
	Mode         string
	Timeout      time.Duration
	ContinueChan <-chan struct{}
}

type AppNodejsInspectOptions struct {
	IncludePackages []string
	NextOpts        NodejsWebFrameworkInspectOptions
	NuxtOpts        NodejsWebFrameworkInspectOptions
}

type NodejsWebFrameworkInspectOptions struct {
	IncludeAppDir         bool
	IncludeBuildDir       bool
	IncludeDistDir        bool
	IncludeStaticDir      bool
	IncludeNodeModulesDir bool
}

type KubernetesOptions struct {
	Target         KubernetesTarget
	TargetOverride KubernetesTargetOverride

	Manifests  []string
	Kubeconfig string
}

type KubernetesTarget struct {
	Workload  string
	Namespace string
	Container string
}

func (t *KubernetesTarget) WorkloadName() (string, error) {
	parts := strings.Split(t.Workload, "/")
	if len(parts) != 2 || len(parts[1]) == 0 {
		return "", errors.New("malformed Kubernetes workload name")
	}

	return parts[1], nil
}

type KubernetesTargetOverride struct {
	Image string
}

func (ko KubernetesOptions) HasTargetSet() bool {
	return ko.Target.Workload != ""
}

// ObfuscateAppPackageNames mode enums
const (
	OAPNNone   = "none"
	OAPNEmpty  = "empty"
	OAPNPrefix = "prefix"
	OAPNRandom = "random"
)
