package crt

import (
	"errors"
	"io"
	"regexp"
	"strings"
	"time"
	//log "github.com/sirupsen/logrus"
)

var (
	ErrBadParam = errors.New("bad parameter")
	ErrNotFound = errors.New("not found")
)

type ImageIdentity struct {
	ID           string
	ShortTags    []string
	RepoTags     []string
	ShortDigests []string
	RepoDigests  []string
}

type BasicImageInfo struct {
	ID          string
	Size        int64
	Created     int64
	VirtualSize int64
	//empty for filtered calls
	ParentID    string
	RepoTags    []string
	RepoDigests []string
	Labels      map[string]string
}

//note: separate BasicImageInfo for each native.RepoTag

type ImageInfo struct {
	Created      time.Time
	ID           string
	RepoTags     []string
	RepoDigests  []string
	Size         int64
	VirtualSize  int64
	OS           string
	Architecture string
	Author       string
	Config       *RunConfig

	RuntimeVersion string //instead of DockerVersion
	RuntimeName    string //instead of DockerVersion
}

// RunConfig describes the runtime config parameters for container instances (aka Config in other libraries)
// Fields (ordered according to spec): Memory, MemorySwap, CpuShares aren't necessary
// * https://github.com/opencontainers/image-spec/blob/main/config.md#properties (Config field)
// * https://github.com/moby/moby/blob/master/api/types/container/config.go#L70
// Note: related to pkg/docker/dockerimage/ContainerConfig
// TODO: refactor into one set of common structs later
type RunConfig struct {
	User         string              `json:"User,omitempty"`
	ExposedPorts map[string]struct{} `json:"ExposedPorts,omitempty"`
	Env          []string            `json:"Env,omitempty"`
	Entrypoint   []string            `json:"Entrypoint,omitempty"`
	Cmd          []string            `json:"Cmd,omitempty"`
	Volumes      map[string]struct{} `json:"Volumes,omitempty"`
	WorkingDir   string              `json:"WorkingDir,omitempty"`
	Labels       map[string]string   `json:"Labels,omitempty"`
	StopSignal   string              `json:"StopSignal,omitempty"`
	ArgsEscaped  bool                `json:"ArgsEscaped,omitempty"`
	Healthcheck  *HealthConfig       `json:"Healthcheck,omitempty"`
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

type ImageHistory struct {
	ID        string `json:"Id"`
	Created   int64
	CreatedBy string
	Tags      []string
	Size      int64
	Comment   string
}

type PullImageOptions struct {
	Repository   string
	Tag          string
	OutputStream io.Writer
}

type AuthConfig interface{}

type InspectorAPIClient interface {
	HasImage(imageRef string) (*ImageIdentity, error)
	ListImages(imageNameFilter string) (map[string]BasicImageInfo, error)
	ListImagesAll() ([]BasicImageInfo, error)
	InspectImage(imageRef string) (*ImageInfo, error)
	GetImagesHistory(imageRef string) ([]ImageHistory, error)

	PullImage(opts PullImageOptions, authConfig AuthConfig) error
	GetRegistryAuthConfig(account, secret, configPath, registry string) (AuthConfig, error)
}

type ImageSaverAPIClient interface {
	SaveImage(imageRef, localPath string, extract, removeOrig bool) error
}

type APIClient interface {
	InspectorAPIClient
	ImageSaverAPIClient
}

func ImageToIdentity(info *ImageInfo) *ImageIdentity {
	result := &ImageIdentity{
		ID:          info.ID,
		RepoTags:    info.RepoTags,
		RepoDigests: info.RepoDigests,
	}

	uniqueTags := map[string]struct{}{}
	for _, tag := range result.RepoTags {
		parts := strings.Split(tag, ":")
		if len(parts) == 2 {
			uniqueTags[parts[1]] = struct{}{}
		}
	}

	for k := range uniqueTags {
		result.ShortTags = append(result.ShortTags, k)
	}

	uniqueDigests := map[string]struct{}{}
	for _, digest := range result.RepoDigests {
		parts := strings.Split(digest, "@")
		if len(parts) == 2 {
			uniqueDigests[parts[1]] = struct{}{}
		}
	}

	for k := range uniqueDigests {
		result.ShortDigests = append(result.ShortDigests, k)
	}

	return result
}

const (
	https = "https://"
	http  = "http://"
)

func ExtractRegistry(repo string) string {
	var scheme string
	if strings.Contains(repo, https) {
		scheme = https
		repo = strings.TrimPrefix(repo, https)
	}
	if strings.Contains(repo, http) {
		scheme = http
		repo = strings.TrimPrefix(repo, http)
	}
	registry := strings.Split(repo, "/")[0]

	domain := `((?:[a-z\d](?:[a-z\d-]{0,63}[a-z\d])?|\*)\.)+[a-z\d][a-z\d-]{0,63}[a-z\d]`
	ipv6 := `^(([0-9a-fA-F]{1,4}:){7,7}[0-9a-fA-F]{1,4}|([0-9a-fA-F]{1,4}:){1,7}:|([0-9a-fA-F]{1,4}:){1,6}:[0-9a-fA-F]{1,4}|([0-9a-fA-F]{1,4}:){1,5}(:[0-9a-fA-F]{1,4}){1,2}|([0-9a-fA-F]{1,4}:){1,4}(:[0-9a-fA-F]{1,4}){1,3}|([0-9a-fA-F]{1,4}:){1,3}(:[0-9a-fA-F]{1,4}){1,4}|([0-9a-fA-F]{1,4}:){1,2}(:[0-9a-fA-F]{1,4}){1,5}|[0-9a-fA-F]{1,4}:((:[0-9a-fA-F]{1,4}){1,6})|:((:[0-9a-fA-F]{1,4}){1,7}|:)|fe80:(:[0-9a-fA-F]{0,4}){0,4}%[0-9a-zA-Z]{1,}|::(ffff(:0{1,4}){0,1}:){0,1}((25[0-5]|(2[0-4]|1{0,1}[0-9]){0,1}[0-9])\.){3,3}(25[0-5]|(2[0-4]|1{0,1}[0-9]){0,1}[0-9])|([0-9a-fA-F]{1,4}:){1,4}:((25[0-5]|(2[0-4]|1{0,1}[0-9]){0,1}[0-9])\.){3,3}(25[0-5]|(2[0-4]|1{0,1}[0-9]){0,1}[0-9]))$`
	ipv4 := `^(((25[0-5]|2[0-4][0-9]|[01]?[0-9][0-9]?)(\.|$)){4})`
	ipv4Port := `([0-9]{1,3}\.[0-9]{1,3}\.[0-9]{1,3}\.[0-9]{1,3})\:?([0-9]{1,5})?`
	ipv6Port := `(([0-9a-fA-F]{1,4}:){7,7}[0-9a-fA-F]{1,4}|([0-9a-fA-F]{1,4}:){1,7}:|([0-9a-fA-F]{1,4}:){1,6}:[0-9a-fA-F]{1,4}|([0-9a-fA-F]{1,4}:){1,5}(:[0-9a-fA-F]{1,4}){1,2}|([0-9a-fA-F]{1,4}:){1,4}(:[0-9a-fA-F]{1,4}){1,3}|([0-9a-fA-F]{1,4}:){1,3}(:[0-9a-fA-F]{1,4}){1,4}|([0-9a-fA-F]{1,4}:){1,2}(:[0-9a-fA-F]{1,4}){1,5}|[0-9a-fA-F]{1,4}:((:[0-9a-fA-F]{1,4}){1,6})|:((:[0-9a-fA-F]{1,4}){1,7}|:)|fe80:(:[0-9a-fA-F]{0,4}){0,4}%[0-9a-zA-Z]{1,}|::(ffff(:0{1,4}){0,1}:){0,1}((25[0-5]|(2[0-4]|1{0,1}[0-9]){0,1}[0-9])\.){3,3}(25[0-5]|(2[0-4]|1{0,1}[0-9]){0,1}[0-9])|([0-9a-fA-F]{1,4}:){1,4}:((25[0-5]|(2[0-4]|1{0,1}[0-9]){0,1}[0-9])\.){3,3}(25[0-5]|(2[0-4]|1{0,1}[0-9]){0,1}[0-9]))`

	if registry == "localhost" || strings.Contains(registry, "localhost:") {
		return scheme + registry
	}

	validDomain := regexp.MustCompile(domain)
	validIpv4 := regexp.MustCompile(ipv4)
	validIpv6 := regexp.MustCompile(ipv6)
	validIpv4WithPort := regexp.MustCompile(ipv4Port)
	validIpv6WithPort := regexp.MustCompile(ipv6Port)

	if validIpv6WithPort.MatchString(registry) {
		return scheme + registry
	}
	if validIpv4WithPort.MatchString(registry) {
		return scheme + registry
	}
	if validIpv6.MatchString(registry) {
		return scheme + registry
	}
	if validIpv4.MatchString(registry) {
		return scheme + registry
	}

	if !validDomain.MatchString(registry) {
		return https + "index.docker.io"
	}
	return scheme + registry
}
