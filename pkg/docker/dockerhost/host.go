package dockerhost

import (
	"net"
	"net/url"
	"os"

	dockerapi "github.com/fsouza/go-dockerclient"
	log "github.com/sirupsen/logrus"

	"github.com/mintoolkit/mint/pkg/util/jsonutil"
)

const (
	localHostIP = "127.0.0.1"
)

// GetIP returns the Docker host IP address
func GetIP(apiClient *dockerapi.Client) string {
	logger := log.WithField("op", "dockerhost.GetIP")
	dockerHost := os.Getenv("DOCKER_HOST")
	logger.WithField("DOCKER_HOST", dockerHost).Trace("os.Getenv")
	if dockerHost == "" {
		if apiClient != nil {
			netInfo, err := apiClient.NetworkInfo("bridge")
			if err != nil {
				logger.WithError(err).Debug("apiClient.NetworkInfo")
			} else {
				logger.WithField("data", jsonutil.ToString(netInfo)).Trace("netInfo")

				if netInfo != nil && netInfo.Name == "bridge" {
					if len(netInfo.IPAM.Config) > 0 {
						return netInfo.IPAM.Config[0].Gateway
					}
				}
			}
		}

		return localHostIP
	}

	u, err := url.Parse(dockerHost)
	if err != nil {
		return localHostIP
	}

	switch u.Scheme {
	case "unix":
		return localHostIP
	default:
		host, _, err := net.SplitHostPort(u.Host)
		if err != nil {
			return localHostIP
		}

		return host
	}
}
