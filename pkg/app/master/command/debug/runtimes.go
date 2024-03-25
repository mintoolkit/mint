package debug

import (
	"os"
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
		Socket:      DockerRuntimeSocket,
		Name:        DockerRuntime,
		Description: DockerRuntimeDesc,
	},
	{
		Socket:      PodmanRuntimeSocket,
		Name:        PodmanRuntime,
		Description: PodmanRuntimeDesc,
	},
}

func AvailableRuntimes() []string {
	var available []string
	for _, info := range runtimes {
		if info.Socket == "" {
			continue
		}

		if hasSocket(info.Socket) {
			available = append(available, info.Name)
		}
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
