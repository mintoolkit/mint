package debug

import (
// "strings"
)

const (
	CgrCustomDebugImage   = "cgr.dev/chainguard/min-toolkit-debug:latest"
	WolfiBaseImage        = "cgr.dev/chainguard/wolfi-base:latest"
	BusyboxImage          = "busybox@sha256:05a79c7279f71f86a2a0d05eb72fcb56ea36139150f0a75cd87e80a4272e4e39"
	NicolakaNetshootImage = "nicolaka/netshoot"
	KoolkitsNodeImage     = "lightruncom/koolkits:node"
	KoolkitsPythonImage   = "lightruncom/koolkits:python"
	KoolkitsGolangImage   = "mintoolkit/koolkits-golang:latest"
	KoolkitsJVMImage      = "lightruncom/koolkits:jvm"
	DigitaloceanDoksImage = "digitalocean/doks-debug:latest"
	ZinclabsUbuntuImage   = "public.ecr.aws/zinclabs/debug-ubuntu-base:latest"
	InfuserImage          = "ghcr.io/teaxyz/infuser:latest"
)

var debugImages = map[string]string{
	CgrCustomDebugImage:   "Chainguard minToolkit debug image - https://images.chainguard.dev/directory/image/min-toolkit-debug , https://edu.chainguard.dev/chainguard/chainguard-images/reference/min-toolkit-debug",
	WolfiBaseImage:        "A lightweight Wolfi base image - https://github.com/chainguard-images/images/tree/main/images/wolfi-base",
	BusyboxImage:          "A lightweight image with common unix utilities - https://busybox.net/about.html",
	NicolakaNetshootImage: "Network trouble-shooting swiss-army container - https://github.com/nicolaka/netshoot",
	KoolkitsNodeImage:     "Node.js KoolKit - https://github.com/lightrun-platform/koolkits/tree/main/nodejs",
	KoolkitsPythonImage:   "Python KoolKit - https://github.com/lightrun-platform/koolkits/tree/main/python",
	KoolkitsGolangImage:   "Go KoolKit - https://github.com/lightrun-platform/koolkits/tree/main/golang",
	KoolkitsJVMImage:      "JVM KoolKit - https://github.com/lightrun-platform/koolkits/blob/main/jvm/README.md",
	DigitaloceanDoksImage: "Kubernetes manifests for investigation and troubleshooting - https://github.com/digitalocean/doks-debug",
	ZinclabsUbuntuImage:   "Common utilities for debugging your cluster - https://github.com/openobserve/debug-container",
	InfuserImage:          "Tea package manager image - https://github.com/teaxyz/infuser",
}

func ShellCommandPrefix(imageName string) []string {
	shellName := defaultShellName
	//TODO:
	//Need to have a reliable solution to deal with
	//the dynamic library dependencies for bash
	//before we default to it in interactive debug shells
	//Need to work out the compat issues linking the shared
	//object dir(s) from the debugging container
	//if strings.Contains(imageName, "lightruncom/koolkits") ||
	//   strings.Contains(imageName, "ubuntu") ||
	//   strings.Contains(imageName, "debian") {
	//   	shellName = bashShellName
	//   	//debian/ubuntu-based images link 'sh' to 'dash', which doesn't support 'set -o pipefail'
	//}

	return []string{shellName, "-c"}
}
