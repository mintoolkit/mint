package debug

import (
	"os"
	"strings"

	"github.com/mintoolkit/mint/pkg/docker/dockerclient"
)

const (
	AutoRuntime     = "auto" //auto-select by detecting runtimes
	AutoRuntimeDesc = "Auto-select based on detected runtimes"

	DockerRuntime     = "docker"
	DockerRuntimeDesc = "Docker runtime - debug a container running in Docker"

	ContainerdRuntime     = "containerd"
	ContainerdRuntimeDesc = "ContainerD runtime"

	KubernetesRuntime     = "k8s"
	KubernetesRuntimeDesc = "Kubernetes runtime - debug a container running in Kubernetes"

	KubeconfigDefault = "${HOME}/.kube/config"
	NamespaceDefault  = "default"

	PodmanRuntime     = "podman"
	PodmanRuntimeDesc = "Podman runtime"
)

//todo: refactor when the "debuggers" are refactored in their own packages

const (
	ContainerdRuntimeSocket = "/var/run/containerd/containerd.sock"
	DockerRuntimeSocket     = "/var/run/docker.sock"
	PodmanRuntimeSocket     = "/var/run/podman/podman.sock"
)

type RuntimeInfo struct {
	Name        string
	Description string
	Socket      string
}

var runtimes = []RuntimeInfo{
	{
		Socket:      ContainerdRuntimeSocket,
		Name:        ContainerdRuntime,
		Description: ContainerdRuntimeDesc,
	},
	{
		Socket:      dockerclient.UserDockerSocket(),
		Name:        DockerRuntime,
		Description: DockerRuntimeDesc,
	},
	{
		Socket:      dockerclient.UnixSocketPath,
		Name:        DockerRuntime,
		Description: DockerRuntimeDesc,
	},
	{
		Socket:      PodmanRuntimeSocket,
		Name:        PodmanRuntime,
		Description: PodmanRuntimeDesc,
	},
	{
		Socket:      getPodmanSocketPath(),
		Name:        PodmanRuntime,
		Description: PodmanRuntimeDesc,
	},
	{
		Socket:      getPodmanRemotePath(),
		Name:        PodmanRuntime,
		Description: PodmanRuntimeDesc,
	},
}

func AvailableRuntimes() []string {
	runtimeSet := map[string]struct{}{}
	for _, info := range runtimes {
		if info.Socket == "" {
			continue
		}

		if strings.HasPrefix(info.Socket, "/") {
			if hasSocket(info.Socket) {
				runtimeSet[info.Name] = struct{}{}
			}
		} else {
			//adding remote paths (for podman and others; without checking, for now)
			runtimeSet[info.Name] = struct{}{}
		}
	}

	var available []string
	for k := range runtimeSet {
		available = append(available, k)
	}

	return available
}

func AutoSelectRuntime() string {
	available := AvailableRuntimes()
	if len(available) > 0 {
		return available[0]
	}

	return DockerRuntime
}

func hasSocket(name string) bool {
	_, err := os.Stat(name)
	if err == nil || !os.IsNotExist(err) {
		return true
	}

	return false
}
