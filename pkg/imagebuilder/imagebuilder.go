package imagebuilder

import (
	"errors"
	"fmt"
	df "github.com/mintoolkit/mint/pkg/docker/dockerfile"
	"io"
	"os"
	"strings"
	"time"
)

var (
	ErrInvalidContextDir = errors.New("invalid context directory")
)

// ImageConfig describes the container image configurations (aka ConfigFile or V1Image/Image in other libraries)
// Fields (ordered according to spec):
// * https://github.com/opencontainers/image-spec/blob/main/config.md#properties
// * https://github.com/moby/moby/blob/e1c92184f08153456ecbf5e302a851afd6f28e1c/image/image.go#LL40C6-L40C13
// Note: related to pkg/docker/dockerimage/V1ConfigObject|ConfigObject
// TODO: refactor into one set of common structs later
type ImageConfig struct {
	Created      time.Time `json:"created,omitempty"`
	Author       string    `json:"author,omitempty"`
	Architecture string    `json:"architecture"`
	OS           string    `json:"os"`
	OSVersion    string    `json:"os.version,omitempty"`
	OSFeatures   []string  `json:"os.features,omitempty"`
	Variant      string    `json:"variant,omitempty"`
	Config       RunConfig `json:"config"`
	RootFS       *RootFS   `json:"rootfs"` //not used building images
	History      []History `json:"history,omitempty"`
	AddHistory   []History `json:"add_history,omitempty"` //extra field
	//Extra fields
	Container     string `json:"container,omitempty"`
	DockerVersion string `json:"docker_version,omitempty"`
	//More extra fields
	ID      string `json:"id,omitempty"`
	Comment string `json:"comment,omitempty"`
}

type RootFS struct {
	Type    string   `json:"type"`
	DiffIDs []string `json:"diff_ids,omitempty"`
}

type History struct {
	Created    string `json:"created,omitempty"`
	Author     string `json:"author,omitempty"`
	CreatedBy  string `json:"created_by,omitempty"`
	Comment    string `json:"comment,omitempty"`
	EmptyLayer bool   `json:"empty_layer,omitempty"`
}

// RunConfig describes the runtime config parameters for container instances (aka Config in other libraries)
// Fields (ordered according to spec): Memory, MemorySwap, CpuShares aren't necessary
// * https://github.com/opencontainers/image-spec/blob/main/config.md#properties (Config field)
// * https://github.com/moby/moby/blob/master/api/types/container/config.go#L70
// Note: related to pkg/docker/dockerimage/ContainerConfig
// TODO: refactor into one set of common structs later
type RunConfig struct {
	User               string              `json:"User,omitempty"`
	ExposedPorts       map[string]struct{} `json:"ExposedPorts,omitempty"`
	AddExposedPorts    map[string]struct{} `json:"AddExposedPorts,omitempty"`    //extra field
	RemoveExposedPorts []string            `json:"RemoveExposedPorts,omitempty"` //extra field
	Env                []string            `json:"Env,omitempty"`
	AddEnv             []string            `json:"AddEnv,omitempty"` //extra field
	Entrypoint         []string            `json:"Entrypoint,omitempty"`
	IsShellEntrypoint  bool                `json:"IsShellEntrypoint,omitempty"` //extra field
	Cmd                []string            `json:"Cmd,omitempty"`
	IsShellCmd         bool                `json:"IsShellCmd,omitempty"` //extra field
	Volumes            map[string]struct{} `json:"Volumes,omitempty"`
	AddVolumes         map[string]struct{} `json:"AddVolumes,omitempty"` //extra field
	WorkingDir         string              `json:"WorkingDir,omitempty"`
	Labels             map[string]string   `json:"Labels,omitempty"`
	AddLabels          map[string]string   `json:"AddLabels,omitempty"` //extra field
	StopSignal         string              `json:"StopSignal,omitempty"`
	ArgsEscaped        bool                `json:"ArgsEscaped,omitempty"`
	Healthcheck        *HealthConfig       `json:"Healthcheck,omitempty"`
	//Extra fields
	AttachStderr    bool     `json:"AttachStderr,omitempty"`
	AttachStdin     bool     `json:"AttachStdin,omitempty"`
	AttachStdout    bool     `json:"AttachStdout,omitempty"`
	Domainname      string   `json:"Domainname,omitempty"`
	Hostname        string   `json:"Hostname,omitempty"`
	Image           string   `json:"Image,omitempty"`
	OnBuild         []string `json:"OnBuild,omitempty"`
	OpenStdin       bool     `json:"OpenStdin,omitempty"`
	StdinOnce       bool     `json:"StdinOnce,omitempty"`
	Tty             bool     `json:"Tty,omitempty"`
	NetworkDisabled bool     `json:"NetworkDisabled,omitempty"`
	MacAddress      string   `json:"MacAddress,omitempty"`
	StopTimeout     *int     `json:"StopTimeout,omitempty"`
	Shell           []string `json:"Shell,omitempty"`
}

type HealthConfig struct {
	Test          []string      `json:",omitempty"`
	Interval      time.Duration `json:",omitempty"`
	Timeout       time.Duration `json:",omitempty"`
	StartPeriod   time.Duration `json:",omitempty"`
	StartInterval time.Duration `json:",omitempty"`
	Retries       int           `json:",omitempty"`
}

type SimpleBuildOptions struct {
	From             string
	FromTar          string
	Tags             []string
	Layers           []LayerDataInfo
	ImageConfig      *ImageConfig
	HideBuildHistory bool
	OutputImageTar   string

	/*
	   //todo:  add 'Healthcheck'
	   Entrypoint   []string
	   Cmd          []string
	   WorkDir      string
	   User         string
	   StopSignal   string
	   OnBuild      []string
	   Volumes      map[string]struct{}
	   EnvVars      []string
	   ExposedPorts map[string]struct{}
	   Labels       map[string]string
	   Architecture string
	*/
}

type LayerSourceType string

const (
	TarSource      LayerSourceType = "lst.tar"
	DirSource      LayerSourceType = "lst.dir"
	FileSource     LayerSourceType = "lst.file"
	FileListSource LayerSourceType = "lst.file_list"
)

type LayerDataInfo struct {
	Type            LayerSourceType
	Source          string
	Params          *DataParams
	EntrypointLayer bool
	ResetCmd        bool
	//TODO: add other common layer metadata...
}

type DataParams struct {
	TargetPath string
	//TODO: add useful fields (e.g., to filter directory files or to use specific file perms, etc)
}

type ImageResult struct {
	ID        string   `json:"id,omitempty"`
	Digest    string   `json:"digest,omitempty"`
	Name      string   `json:"name,omitempty"`
	OtherTags []string `json:"other_tags,omitempty"`
}

type SimpleBuildEngine interface {
	Name() string
	Build(options SimpleBuildOptions) (*ImageResult, error)
}

type NVParam struct {
	Name  string
	Value string
}

type DockerfileBuildOptions struct {
	Dockerfile   string
	BuildContext string
	IgnoreFile   string
	ImagePath    string
	BuildArgs    []NVParam
	Labels       map[string]string
	Target       string
	NetworkMode  string
	ExtraHosts   string //already a comma separated list
	CacheFrom    []string
	CacheTo      []string
	Platforms    []string
	OutputStream io.Writer
}

type DockerfileBuildEngine interface {
	Name() string
	Build(options DockerfileBuildOptions) error
}

func SimpleBuildOptionsFromDockerfileData(data string, ignoreExeInstructions bool) (*SimpleBuildOptions, error) {
	var options SimpleBuildOptions
	lines := strings.Split(data, "\n")

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "#") {
			continue
		}

		parts := strings.SplitN(line, " ", 2)
		if len(parts) != 2 {
			continue
		}

		instName := strings.ToLower(parts[0])
		switch instName {
		case df.InstTypeEntrypoint:
			//options.Entrypoint []string
		case df.InstTypeCmd:
			//options.Cmd []string
		case df.InstTypeEnv:
			//options.EnvVars []string
		case df.InstTypeExpose:
			//options.ExposedPorts map[string]struct{}
		case df.InstTypeLabel:
			//options.Labels map[string]string
		case df.InstTypeUser:
			//options.User = parts[1]
			options.ImageConfig.Config.User = parts[1]
		case df.InstTypeVolume:
			//options.Volumes map[string]struct{}
		case df.InstTypeWorkdir:
			//options.WorkDir = parts[1]
			options.ImageConfig.Config.WorkingDir = parts[1]
		case df.InstTypeAdd:
			//support tar files (ignore other things, at leas, for now)
			//options.Layers []LayerDataInfo
		case df.InstTypeCopy:
			//options.Layers []LayerDataInfo
		case df.InstTypeMaintainer:
			//TBD
		case df.InstTypeHealthcheck:
			//TBD
		case df.InstTypeFrom:
			//options.From string
		case df.InstTypeArg:
			//TODO
		case df.InstTypeRun:
			if !ignoreExeInstructions {
				return nil, fmt.Errorf("RUN instructions are not supported")
			}
		case df.InstTypeOnbuild:
			//IGNORE
		case df.InstTypeShell:
			//IGNORE
		case df.InstTypeStopSignal:
			//IGNORE
		}
	}
	return &options, nil
}

func SimpleBuildOptionsFromDockerfile(path string, ignoreExeInstructions bool) (*SimpleBuildOptions, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	return SimpleBuildOptionsFromDockerfileData(string(data), ignoreExeInstructions)
}

func SimpleBuildOptionsFromImageConfig(data *ImageConfig) (*SimpleBuildOptions, error) {
	return &SimpleBuildOptions{ImageConfig: data}, nil
}
