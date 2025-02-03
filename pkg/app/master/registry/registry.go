package registry

import (
	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/daemon"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/google/go-containerregistry/pkg/v1/tarball"
	log "github.com/sirupsen/logrus"
)

func ConfigureAuth(useDockerCreds bool, account string, secret string, remoteOpts []remote.Option) ([]remote.Option, error) {
	if useDockerCreds {
		remoteOpts = append(remoteOpts, remote.WithAuthFromKeychain(authn.DefaultKeychain))
		return remoteOpts, nil
	}

	if account != "" && secret != "" {
		remoteOpts = append(remoteOpts, remote.WithAuth(&authn.Basic{
			Username: account,
			Password: secret,
		}))

		return remoteOpts, nil
	}

	//it's authn.Anonymous by default, but good to be explicit
	return append(remoteOpts, remote.WithAuth(authn.Anonymous)), nil
}

func PushImageFromTar(
	logger *log.Entry,
	tarPath string,
	remoteImageName string,
	nameOpts []name.Option,
	remoteOpts []remote.Option) error {
	logger = logger.WithField("op", "registry.PushImageFromTar")
	logger.Trace("call")
	defer logger.Trace("exit")

	ref, err := name.ParseReference(remoteImageName, nameOpts...)
	if err != nil {
		logger.WithError(err).Errorf("name.ParseReference(%s)", remoteImageName)
		return err
	}

	img, err := tarball.ImageFromPath(tarPath, nil)
	if err != nil {
		logger.WithError(err).Errorf("tarball.ImageFromPath(%s)", tarPath)
		return err
	}

	err = remote.Write(ref, img, remoteOpts...)
	if err != nil {
		logger.WithError(err).Errorf("tarball.ImageFromPath(%s, %s)", tarPath, remoteImageName)
		return err
	}

	return nil
}

func SaveDockerImage(
	logger *log.Entry,
	localImageName string,
	tarPath string,
	nameOpts []name.Option) error {
	//not really a registry operation, but related to the flow...
	logger = logger.WithField("op", "registry.SaveDockerImage")
	logger.Trace("call")
	defer logger.Trace("exit")

	ref, err := name.ParseReference(localImageName, nameOpts...)
	if err != nil {
		logger.WithError(err).Errorf("name.ParseReference(%s)", localImageName)
		return err
	}

	img, err := daemon.Image(ref)
	if err != nil {
		logger.WithError(err).Errorf("daemon.Image(%s)", localImageName)
		return err
	}

	if err := tarball.WriteToFile(tarPath, ref, img); err != nil {
		logger.WithError(err).Errorf("tarball.WriteToFile(%s, %s)", tarPath, localImageName)
		return err
	}

	return nil
}
