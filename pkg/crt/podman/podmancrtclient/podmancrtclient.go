package podmancrtclient

import (
	//"fmt"
	"context"
	"errors"
	"fmt"
	"strings"

	//log "github.com/sirupsen/logrus"
	"github.com/containers/podman/v5/pkg/bindings/images"
	"github.com/containers/storage"

	"github.com/mintoolkit/mint/pkg/crt"
	"github.com/mintoolkit/mint/pkg/crt/podman/podmanutil"
)

type Instance struct {
	pclient context.Context
}

func New(providerClient context.Context) *Instance {
	return &Instance{
		pclient: providerClient,
	}
}

func (ref *Instance) HasImage(imageRef string) (*crt.ImageIdentity, error) {
	pii, err := podmanutil.HasImage(ref.pclient, imageRef)
	if err != nil {
		if err == podmanutil.ErrNotFound {
			err = crt.ErrNotFound
		}

		return nil, err
	}
	ii := &crt.ImageIdentity{
		ID:           pii.ID,
		ShortTags:    pii.ShortTags,
		RepoTags:     pii.RepoTags,
		ShortDigests: pii.ShortDigests,
		RepoDigests:  pii.RepoDigests,
	}

	return ii, nil
}

func (ref *Instance) ListImagesAll() ([]crt.BasicImageInfo, error) {
	options := &images.ListOptions{}
	pimages, err := images.List(ref.pclient, options.WithAll(true))
	if err != nil {
		return nil, err
	}

	var imageList []crt.BasicImageInfo
	for _, r := range pimages {
		imageList = append(imageList, crt.BasicImageInfo{
			ID:          strings.TrimPrefix(r.ID, "sha256:"),
			Size:        r.Size,
			Created:     r.Created,
			VirtualSize: r.VirtualSize,
			ParentID:    r.ParentId,
			RepoTags:    r.RepoTags,
			RepoDigests: r.RepoDigests,
			Labels:      r.Labels,
		})
	}

	return imageList, nil
}

func (ref *Instance) ListImages(imageNameFilter string) (map[string]crt.BasicImageInfo, error) {
	//needs extra testing...
	pimages, err := podmanutil.ListImages(ref.pclient, imageNameFilter)
	if err != nil {
		return nil, err
	}

	images := map[string]crt.BasicImageInfo{}
	for k, v := range pimages {
		images[k] = crt.BasicImageInfo{
			ID:      v.ID,
			Size:    v.Size,
			Created: v.Created,
		}
	}

	return images, nil
}

func (ref *Instance) InspectImage(imageRef string) (*crt.ImageInfo, error) {
	pimage, err := images.GetImage(ref.pclient, imageRef, nil)
	if err != nil {
		if errors.Is(err, storage.ErrImageUnknown) {
			return nil, crt.ErrNotFound
		}

		return nil, err
	}

	result := &crt.ImageInfo{
		RuntimeName:    "podman",
		RuntimeVersion: pimage.Version,
		ID:             pimage.ID,
		RepoTags:       pimage.RepoTags,
		RepoDigests:    pimage.RepoDigests,
		Size:           pimage.Size,
		VirtualSize:    pimage.VirtualSize,
		OS:             pimage.Os,
		Architecture:   pimage.Architecture,
		Author:         pimage.Author,
	}

	if pimage.Created != nil {
		result.Created = *pimage.Created
	}

	if pimage.Config != nil {
		//https://github.com/opencontainers/image-spec/blob/main/specs-go/v1/config.go
		result.Config = &crt.RunConfig{
			User:         pimage.Config.User,
			ExposedPorts: pimage.Config.ExposedPorts,
			Env:          pimage.Config.Env,
			Entrypoint:   pimage.Config.Entrypoint,
			Cmd:          pimage.Config.Cmd,
			Volumes:      pimage.Config.Volumes,
			WorkingDir:   pimage.Config.WorkingDir,
			Labels:       pimage.Config.Labels,
			StopSignal:   pimage.Config.StopSignal,
			ArgsEscaped:  pimage.Config.ArgsEscaped,
			//not defined:
			//AttachStderr: ,
			//AttachStdin: ,
			//AttachStdout: ,
			//Domainname: ,
			//Hostname: ,
			//Image: ,
			//OnBuild: ,
			//OpenStdin: ,
			//StdinOnce: ,
			//Tty: ,
			//NetworkDisabled: ,
			//MacAddress: ,
			//StopTimeout: ,
			//Shell: ,
		}

		if pimage.HealthCheck != nil {
			result.Config.Healthcheck = &crt.HealthConfig{
				Test:          pimage.HealthCheck.Test,
				Interval:      pimage.HealthCheck.Interval,
				Timeout:       pimage.HealthCheck.Timeout,
				StartPeriod:   pimage.HealthCheck.StartPeriod,
				StartInterval: pimage.HealthCheck.StartInterval,
				Retries:       pimage.HealthCheck.Retries,
			}
		}
	}

	return result, nil
}

func (ref *Instance) PullImage(opts crt.PullImageOptions, authConfig crt.AuthConfig) error {
	//todo: add support to pull specific architecture later
	imageName := opts.Repository
	if opts.Tag != "" {
		imageName = strings.Join([]string{opts.Repository, opts.Tag}, ":")
	}

	options := &images.PullOptions{}
	if opts.OutputStream != nil {
		options.WithProgressWriter(opts.OutputStream)
	}

	if authConfig != nil {
		ac, ok := authConfig.(*authConfigData)
		if !ok {
			return fmt.Errorf("invalid authConfigData")
		}

		if ac.account != "" {
			options.WithUsername(ac.account)
		}

		if ac.secret != "" {
			options.WithPassword(ac.secret)
		}

		if ac.configPath != "" {
			options.WithAuthfile(ac.configPath)
		}
	}

	_, err := images.Pull(ref.pclient, imageName, options)
	if err != nil {
		return err
	}

	return nil
}

type authConfigData struct {
	account    string
	secret     string
	configPath string
	registry   string
}

func (ref *Instance) GetRegistryAuthConfig(account, secret, configPath, registry string) (crt.AuthConfig, error) {
	output := &authConfigData{
		account:    account,
		secret:     secret,
		configPath: configPath,
		registry:   registry,
	}

	return output, nil
}

func (ref *Instance) SaveImage(imageRef, localPath string, extract, removeOrig bool) error {
	err := podmanutil.SaveImage(ref.pclient, imageRef, localPath, extract, removeOrig, true)
	if err != nil {
		if err == podmanutil.ErrBadParam {
			err = crt.ErrBadParam
		}
		return err
	}

	return nil
}

func (ref *Instance) GetImagesHistory(imageRef string) ([]crt.ImageHistory, error) {
	phistory, err := images.History(ref.pclient, imageRef, nil)
	if err != nil {
		return nil, err
	}

	var result []crt.ImageHistory
	for _, r := range phistory {
		result = append(result, crt.ImageHistory{
			ID:        r.ID,
			Created:   r.Created,
			CreatedBy: r.CreatedBy,
			Tags:      r.Tags,
			Size:      r.Size,
			Comment:   r.Comment,
		})
	}

	return result, nil
}
