package crt

import (
	"context"
	"fmt"
	"os"
	"strings"
	"sync"

	"github.com/containers/common/pkg/config"
	"github.com/containers/podman/v5/pkg/bindings"
	log "github.com/sirupsen/logrus"

	"github.com/mintoolkit/mint/pkg/util/jsonutil"
)

var trueVal = true

var PodmanConnErr error

var podmanSocketPathInit sync.Once
var podmanSocketPath string

var podmanRemotePathInit sync.Once
var podmanRemotePath string

var podmanConnCtxInit sync.Once
var podmanConnCtx context.Context

func PodmanIsRemote() bool {
	return false
}

func GetPodmanSocketPath() string {
	podmanSocketPathInit.Do(func() {
		dcURI, dcIdentity := PodmanGetDefaultConnection()
		log.Tracef("HandlePodmanRuntime: URI=%s Identity=%s", dcURI, dcIdentity)
		podmanRemotePath = dcURI
		if dcURI == "" {
			sockDir := os.Getenv("XDG_RUNTIME_DIR")
			if sockDir != "" {
				podmanSocketPath = fmt.Sprintf("%s/podman/podman.sock", sockDir)
				if !HasSocket(podmanSocketPath) {
					podmanSocketPath = ""
				} else {
					return
				}
			}

			//we might have $XDG_RUNTIME_DIR,
			//but the actual Podman socket might still be /var/run/podman/podman.sock
			sockDir = "/var/run"
			podmanSocketPath = fmt.Sprintf("%s/podman/podman.sock", sockDir)
			if !HasSocket(podmanSocketPath) {
				podmanSocketPath = ""
			}
		}
	})
	return podmanSocketPath
}

func GetPodmanRemotePath() string {
	podmanRemotePathInit.Do(func() {
		dcURI, dcIdentity := PodmanGetDefaultConnection()
		log.Tracef("HandlePodmanRuntime: URI=%s Identity=%s", dcURI, dcIdentity)
		podmanRemotePath = dcURI

	})
	return podmanRemotePath
}

func GetPodmanConnContext() context.Context {
	podmanConnCtxInit.Do(func() {
		dcURI, dcIdentity := PodmanGetDefaultConnection()
		log.Tracef("HandlePodmanRuntime: URI=%s Identity=%s", dcURI, dcIdentity)
		podmanRemotePath = dcURI
		if dcURI == "" {
			var foundSock bool
			sockDir := os.Getenv("XDG_RUNTIME_DIR")
			if sockDir != "" {
				podmanSocketPath = fmt.Sprintf("%s/podman/podman.sock", sockDir)
				if !HasSocket(podmanSocketPath) {
					podmanSocketPath = ""
				} else {
					dcURI = fmt.Sprintf("unix://%s/podman/podman.sock", sockDir)
					foundSock = true
				}
			}

			if !foundSock {
				//we might have $XDG_RUNTIME_DIR,
				//but the actual Podman socket might still be /var/run/podman/podman.sock
				sockDir = "/var/run"
				podmanSocketPath = fmt.Sprintf("%s/podman/podman.sock", sockDir)
				if !HasSocket(podmanSocketPath) {
					podmanSocketPath = ""
					return
				}

				dcURI = fmt.Sprintf("unix://%s/podman/podman.sock", sockDir)
			}
		}

		if dcURI == "" {
			podmanConnCtx = nil
			return
		}

		ctx := context.Background()
		connCtx, err := bindings.NewConnectionWithIdentity(ctx, dcURI, dcIdentity, false)
		if err != nil {
			PodmanConnErr = err
			log.WithError(err).Trace("getPodmanConnContext:bindings.NewConnectionWithIdentity")
		}

		podmanConnCtx = connCtx
	})
	return podmanConnCtx
}

func GetPodmanConnContextWithConn(connURI string) context.Context {
	podmanConnCtxInit.Do(func() {
		if strings.HasPrefix(connURI, "/") {
			connURI = fmt.Sprintf("unix://%s", connURI)
		}

		log.Tracef("GetPodmanConnContextWithConn: URI=%s", connURI)
		podmanRemotePath = connURI
		if connURI == "" {
			GetPodmanConnContext()
			return
		}

		ctx := context.Background()
		connCtx, err := bindings.NewConnectionWithIdentity(ctx, connURI, "", false)
		if err != nil {
			PodmanConnErr = err
			log.WithError(err).Tracef("getPodmanConnContextWithConn(%s):bindings.NewConnectionWithIdentity", connURI)
		}

		podmanConnCtx = connCtx
	})
	return podmanConnCtx
}

func PodmanGetDefaultConnection() (string, string) {
	podmanConfig, err := config.Default()
	if err != nil {
		log.WithError(err).Error("config.Default")
		return "", ""
	}

	connections, err := podmanConfig.GetAllConnections()
	if err != nil {
		log.WithError(err).Error("podmanConfig.GetAllConnections")
		return "", ""
	}

	log.Tracef("Connections: %s", jsonutil.ToString(connections))

	for _, c := range connections {
		if c.Default {
			return c.URI, c.Identity
		}
	}

	log.Tracef("Config.Engine: ActiveService=%s ServiceDestinations=%s",
		podmanConfig.Engine.ActiveService,
		jsonutil.ToString(podmanConfig.Engine.ServiceDestinations))

	if podmanConfig.Engine.ActiveService != "" {
		sd, found := podmanConfig.Engine.ServiceDestinations[podmanConfig.Engine.ActiveService]
		if !found {
			return "", ""
		}

		return sd.URI, sd.Identity
	}

	return "", ""
}
