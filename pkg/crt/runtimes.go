package crt

import (
	"net"
	"time"

	log "github.com/sirupsen/logrus"

	"os"
	"strings"

	"github.com/mintoolkit/mint/pkg/crt/docker/dockerclient"
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

var runtimeDefaultConnections = []RuntimeInfo{
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
		Socket:      GetPodmanSocketPath(),
		Name:        PodmanRuntime,
		Description: PodmanRuntimeDesc,
	},
	{
		Socket:      GetPodmanRemotePath(), //only reads configs (no REST calls)
		Name:        PodmanRuntime,
		Description: PodmanRuntimeDesc,
	},
}

func AvailableRuntimes() []string {
	usable := map[string]struct{}{}
	for _, info := range runtimeDefaultConnections {
		if info.Socket == "" {
			continue
		}

		if strings.HasPrefix(info.Socket, "/") {
			if HasSocket(info.Socket) {
				if CanConnect(info.Socket) {
					usable[info.Name] = struct{}{}
				}
			}
		} else {
			//adding remote paths (for podman and others; without checking, for now)
			usable[info.Name] = struct{}{}
		}
	}

	var available []string
	//need to preserve the order from 'runtimes'
	saved := map[string]struct{}{}
	for _, info := range runtimeDefaultConnections {
		_, ufound := usable[info.Name]
		_, sfound := saved[info.Name]
		if ufound && !sfound {
			available = append(available, info.Name)
			saved[info.Name] = struct{}{}
		}
	}

	return available
}

func AutoSelectRuntime() string {
	available := AvailableRuntimes()
	log.Debugf("Available runtimes: %v", available)
	if len(available) > 0 {
		return available[0]
	}

	return DockerRuntime // Question -> This runtime may not necessarily be available?
}

func HasSocket(name string) bool {
	_, err := os.Stat(name)
	if err == nil || !os.IsNotExist(err) {
		return true
	}

	return false
}

func CanConnect(socket string) bool {
	timeout := 5 * time.Second
	conn, err := net.DialTimeout("unix", socket, timeout)

	if err != nil {
		// If there are permission issues, this line will be tripped
		// when trying to connect to the socket.
		log.Debugf("Error connecting to socket: %s: %v", socket, err)
		return false
	}
	defer conn.Close()

	return true
}
